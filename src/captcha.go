package main

import (
	"bytes"
	"fmt"
	"github.com/afocus/captcha"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"image/color"
	"image/png"
	"log"
	"math/rand"
	"regexp"
	"strconv"
	"sync"
	"time"
)

const (
	// TODO: remove this hardcoding. For this, this requires the
	// fonts-dejavu-core package in Debian & derived distros.
	captchaFont = "/usr/share/fonts/truetype/dejavu/DejaVuSerif-Bold.ttf"
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

// Add adds a userID and expiration to the list of users for which we're still
// waiting for a captcha response.
func (x *pendingCaptchaType) add(userID int, captcha botCaptcha) {
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
// the captcha and emits a random captcha.
func (x *opBot) sendCaptcha(bot tgbotInterface, update tgbotapi.Update, user tgbotapi.User) {
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
	msg, err := sendPhoto(bot, update.Message.Chat.ID, fb, fmt.Sprintf(T("enter_captcha"), name))
	if err != nil {
		log.Printf("Warning: Unable to send captcha message: %v", err)
	} else {
		selfDestructMessage(bot, msg.Chat.ID, msg.MessageID, x.captchaTime+time.Duration(10*time.Second))
	}
}

// genCaptchaImage generates a captcha image based on the captcha code. It
// assumes the code to be between 0 and 9999.
func genCaptchaImage(code int) (tgbotapi.FileBytes, error) {
	var ret tgbotapi.FileBytes

	if code < 0 || code > 9999 {
		return ret, fmt.Errorf("captcha code must be between 0 and 9999, got %d", code)
	}

	cap := captcha.New()
	cap.SetFont(captchaFont)
	cap.SetSize(240, 150)
	cap.SetFrontColor(color.RGBA{255, 255, 255, 255})
	cap.SetBkgColor(color.RGBA{255, 0, 0, 255}, color.RGBA{0, 0, 255, 255}, color.RGBA{0, 153, 0, 255})

	img := cap.CreateCustom(fmt.Sprintf("%04.4d", code))

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

		name := nameRef(user)

		// At this point, we reached our captcha timeout and the user is still
		// in the pending captcha list, meaning they didn't confirm the
		// captcha.
		_, err := sendMessage(bot, update.Message.Chat.ID, fmt.Sprintf(T("no_captcha_received"), name))
		if err != nil {
			log.Printf("Warning: Unable to send 'invalid captcha' message.")
		}

		// Kick user and remove from list.
		if err = kickUser(bot, update.Message.Chat.ID, user.ID); err != nil {
			log.Printf("Warning: Unable to kick user %s (uid=%d) out of the channel.", name, user.ID)
		}
		x.pendingCaptcha.del(user.ID)
	})
}

// markAsPendingCaptcha marks the user status as pending Captcha response.
func (x *opBot) markAsPendingCaptcha(bot tgbotInterface, user tgbotapi.User, code int) {
	log.Printf("Adding user to the pending-captcha list: %q, uid=%d\n", formatName(user), user.ID)
	x.pendingCaptcha.add(user.ID, botCaptcha{
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
