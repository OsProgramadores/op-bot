package main

import (
	"bytes"
	"fmt"
	"github.com/dchest/captcha"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"image/png"
	"log"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	captchaWidth  = 400
	captchaHeight = 240
)

// Precompile the regular expression used to match captchas.
var captchaRegex *regexp.Regexp

func init() {
	captchaRegex = regexp.MustCompile(`\d{4}`)
}

type botCaptcha struct {
	code       int
	expiration time.Time
}

// pendingCaptchaType holds a list of UserIDs that have yet to be validated by
// captcha or any other means to detect non-humans.
type pendingCaptchaType struct {
	sync.RWMutex
	users map[int]botCaptcha
}

// set sets a userID and expiration to the list of users for which we're still
// waiting for a captcha response.
func (x *pendingCaptchaType) set(userID int, captcha botCaptcha) {
	x.Lock()
	x.users[userID] = captcha
	x.Unlock()
}

// get retrieves a botCaptcha object from the list of users pending captcha
// validation.
func (x *pendingCaptchaType) get(userID int) (botCaptcha, bool) {
	x.Lock()
	exp, ok := x.users[userID]
	x.Unlock()
	return exp, ok
}

// del removes a userID from the list of users pending captcha validation.
func (x *pendingCaptchaType) del(userID int) {
	x.Lock()
	delete(x.users, userID)
	x.Unlock()
}

func newPendingCaptchaType() *pendingCaptchaType {
	return &pendingCaptchaType{
		users: map[int]botCaptcha{},
	}
}

// sendCaptcha adds the user to the map of users that have not yet responded to
// the captcha and sends a random captcha image as a reply to the message.
func (x *opBot) sendCaptcha(bot tgbotInterface, update tgbotapi.Update, user tgbotapi.User) {
	promCaptchaCount.Inc()

	// Do not send captcha messages to bots (belt and suspenders...)
	if user.IsBot {
		return
	}
	name := nameRef(user)

	// Random captcha, expires in x.captchaTime duration.
	code := rand.Int() % 10000

	x.markAsPendingCaptcha(bot, user, code)

	// Send the captcha message. Set to autodestruct in captcha_time + 10s.
	log.Printf("Sending captcha %04.4d to user %s (uid=%d)", code, name, user.ID)

	fb, err := genCaptchaImage(code)
	if err != nil {
		log.Printf("Warning: Unable to generate captcha image. Ignoring")
		return
	}
	// Send.
	msg, err := sendPhotoReply(bot, update.Message.Chat.ID, update.Message.MessageID, fb, fmt.Sprintf(T("enter_captcha"), name))
	if err != nil {
		log.Printf("Warning: Unable to send captcha message: %v", err)
		return
	}
	// Clean message after captcha duration + 10 seconds.
	selfDestructMessage(bot, msg.Chat.ID, msg.MessageID, x.captchaTime+time.Duration(10*time.Second))
}

// genCaptchaImage generates a captcha image based on the captcha code. It
// assumes the code to be between 0 and 9999.
func genCaptchaImage(code int) (tgbotapi.FileBytes, error) {
	var ret tgbotapi.FileBytes

	if code < 0 || code > 9999 {
		return ret, fmt.Errorf("captcha code must be between 0 and 9999, got %d", code)
	}
	codeStr := fmt.Sprintf("%04.4d", code)
	img := captcha.NewImage(codeStr, bincode(codeStr), captchaWidth, captchaHeight)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		log.Printf("Unable to generate image captcha: %v", err)
		return ret, err
	}
	ret = tgbotapi.FileBytes{
		Name:  "captcha",
		Bytes: buf.Bytes(),
	}
	buf.Reset()
	return ret, nil
}

// captchaReaper creates a function to reap this user after the timeout in
// x.captchaTime if the user still has not confirmed the captcha.
func (x *opBot) captchaReaper(bot tgbotInterface, update tgbotapi.Update, user tgbotapi.User) {
	time.AfterFunc(x.captchaTime, func() {
		// User not in the list means they already confirmed captcha.
		_, ok := x.pendingCaptcha.get(user.ID)
		if !ok {
			return
		}
		promCaptchaFailedCount.Inc()

		name := nameRef(user)

		// Let's check if this user is already banned, in which case we can
		// skip some of the following steps.
		banned, err := isBanned(bot, update.Message.Chat.ID, user.ID)
		if err != nil {
			log.Printf("Warning: Unable to get information for user %s (uid=%d): %v", name, user.ID, err)
		}

		// At this point, we reached our captcha timeout and the user is still
		// in the pending captcha list, meaning they didn't confirm the
		// captcha.

		// Let's remove the pending request.
		defer x.pendingCaptcha.del(user.ID)

		// Do nothing if user is already kicked (probably by an admin).
		if banned {
			log.Printf("User %s (uid=%d) has already been banned (by admin?) Not unbanning.", name, user.ID)
			return
		}

		// As the user is not yet banned, proced to kick+unban.
		// Kick user and remove from list.
		_, err = sendMessage(bot, update.Message.Chat.ID, fmt.Sprintf(T("no_captcha_received"), name))
		if err != nil {
			log.Printf("Warning: Unable to send 'invalid captcha' message.")
		}

		if err = kickUser(bot, update.Message.Chat.ID, user.ID); err != nil {
			log.Printf("Warning: Unable to kick user %s (uid=%d) out of the channel.", name, user.ID)
		}

		if err = unBanUser(bot, update.Message.Chat.ID, user.ID); err != nil {
			log.Printf("Warning: Unable to UNBAN user %s (uid=%d) (May be locked out.)", name, user.ID)
		}
	})
}

// markAsPendingCaptcha marks the user status as pending Captcha response.
func (x *opBot) markAsPendingCaptcha(bot tgbotInterface, user tgbotapi.User, code int) {
	log.Printf("Adding user to the pending-captcha list: %q, uid=%d\n", formatName(user), user.ID)
	x.pendingCaptcha.set(user.ID, botCaptcha{
		code:       code,
		expiration: time.Now().Add(x.captchaTime),
	})
}

// userCaptcha returns the captcha code for the user iff the captcha feature is
// enabled, the user is not an admin, and the user has not yet been validated.
func userCaptcha(x *opBot, bot getChatMemberer, chatid int64, userid int) *botCaptcha {
	admin, err := isAdmin(bot, chatid, userid)
	if err != nil {
		log.Printf("Unable to determine if userid %d is an admin. Assuming regular user.", userid)
	}
	if admin {
		return nil
	}
	if !captchaEnabled(x) {
		return nil
	}
	captcha, ok := x.pendingCaptcha.get(userid)
	if !ok {
		return nil
	}
	return &captcha
}

// matchCaptcha retrieves a numeric sequence from the text of the message and
// returns true if it matches captcha.code.
func matchCaptcha(captcha botCaptcha, text string) bool {
	// Ignore anything over 40 characters, just in case someone
	// wants to send a message with all possible combinations. :)
	if len(text) > 40 {
		return false
	}
	match := captchaRegex.FindString(text)
	// No match at all.
	if match == "" {
		return false
	}
	// Found code attempt, but does not match the captcha code.
	code, _ := strconv.Atoi(match)
	if code == captcha.code {
		return true
	}
	log.Printf("Found captcha code %d, wanted %d. Message %q", code, captcha.code, text)
	return false
}

// captchaEnabled returns true if the captcha feature is enabled.
func captchaEnabled(bot *opBot) bool {
	return bot.captchaTime.Seconds() > 0
}

// bincode converts a string of digits into its binary representation.
// Non-digits will be silently ignored.
func bincode(s string) []byte {
	ret := []byte{}
	for _, d := range s {
		if unicode.IsDigit(d) {
			ret = append(ret, byte(d-'0'))
		}
	}
	return ret
}

// captchaResendRequest returns true if the text contains a request for another captcha.
func captchaResendRequest(s string) bool {
	return strings.EqualFold(s, T("another_captcha"))
}
