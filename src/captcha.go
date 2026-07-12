package main

import (
	"bytes"
	"fmt"
	"image/png"
	"log"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/dchest/captcha"
	tgbotapi "github.com/osprogramadores/telegram-bot-api"
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
func (x *opBot) sendCaptcha(bot tgbotInterface, chatID int64, messageID int, user tgbotapi.User) {
	promCaptchaCount.Inc()

	// Do not send captcha messages to bots (belt and suspenders...)
	if user.IsBot {
		return
	}
	name := nameRef(user)

	// Random captcha, expires in x.captchaTime duration.
	code := rand.Int() % 10000

	x.markAsPendingCaptcha(user, code)

	// Send the captcha message. Set to autodestruct in captcha_time + 10s.
	log.Printf("Sending captcha %04.4d to user %s (uid=%d)", code, name, user.ID)

	fb, err := genCaptchaImage(code)
	if err != nil {
		log.Printf("Warning: Unable to generate captcha image. Ignoring")
		return
	}
	// Send.
	msg, err := sendPhotoReply(bot, chatID, messageID, fb, fmt.Sprintf(T("enter_captcha"), name))
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
func (x *opBot) captchaReaper(bot tgbotInterface, chatID int64, user tgbotapi.User) {
	time.AfterFunc(x.captchaTime, func() {
		// User not in the list means they already confirmed captcha.
		_, ok := x.pendingCaptcha.get(user.ID)
		if !ok {
			return
		}
		promCaptchaFailedCount.Inc()

		name := nameRef(user)

		// check if this user is already banned and possibly skip some of the following steps.
		_, err := isBanned(bot, chatID, user.ID)
		if err != nil {
			log.Printf("Warning: Unable to get information for user %s (uid=%d): %v", name, user.ID, err)
		}

		// At this point, we reached our captcha timeout and the user is still
		// in the pending captcha list, meaning they didn't confirm the
		// captcha.

		x.handleCaptchaFailure(bot, chatID, 0, user)
	})
}

// markAsPendingCaptcha marks the user status as pending Captcha response.
func (x *opBot) markAsPendingCaptcha(user tgbotapi.User, code int) {
	log.Printf("Adding user to the pending-captcha list: %q, uid=%d\n", formatName(user), user.ID)
	x.pendingCaptcha.set(user.ID, botCaptcha{
		code:       code,
		expiration: time.Now().Add(x.captchaTime),
	})
}

// userCaptcha returns the captcha code for the user iff the captcha feature is
// enabled, and the user has not yet been validated.
func userCaptcha(x *opBot, bot getChatMemberer, chatid int64, userid int) *botCaptcha {
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

// handleCaptchaFailure deals with users who failed the captcha (timeout or wrong answer).
func (x *opBot) handleCaptchaFailure(bot tgbotInterface, chatID int64, messageID int, user tgbotapi.User) {
	name := nameRef(user)
	fails := x.captchaFails.increment(user.ID)

	log.Printf("User %s (uid=%d) failed captcha. Total fails: %d", name, user.ID, fails)

	// Remove from pending
	x.pendingCaptcha.del(user.ID)

	banned, err := isBanned(bot, chatID, user.ID)
	if err != nil {
		log.Printf("Warning: Unable to get information for user %s (uid=%d): %v", name, user.ID, err)
	}

	if banned {
		log.Printf("User %s (uid=%d) has already been banned. Not doing anything.", name, user.ID)
		return
	}

	switch fails {
	case 1:
		sendMessage(bot, chatID, fmt.Sprintf(T("captcha_fail_1"), name))
		kickUser(bot, chatID, user.ID)
		unBanUser(bot, chatID, user.ID)
	case 2:
		sendMessage(bot, chatID, fmt.Sprintf(T("captcha_fail_2"), name))
		kickUser(bot, chatID, user.ID)
		unBanUser(bot, chatID, user.ID)
	case 3:
		sendMessage(bot, chatID, fmt.Sprintf(T("captcha_fail_3"), name))
		kickUserUntil(bot, chatID, user.ID, time.Now().Add(24*time.Hour))
	default:
		sendMessage(bot, chatID, fmt.Sprintf(T("captcha_fail_max"), name))
		banUser(bot, chatID, user.ID)
	}
}

const captchaFailuresDB = "captcha_failures.json"

type captchaFailures struct {
	sync.RWMutex
	Failures map[int]int `json:"failures"`
}

func newCaptchaFailures() *captchaFailures {
	cf := &captchaFailures{
		Failures: map[int]int{},
	}
	cf.load()
	return cf
}

func (c *captchaFailures) load() {
	c.Lock()
	defer c.Unlock()
	readJSONFromDataDir(&c.Failures, captchaFailuresDB)
	if c.Failures == nil {
		c.Failures = map[int]int{}
	}
}

func (c *captchaFailures) save() error {
	return safeWriteJSON(c.Failures, captchaFailuresDB)
}

func (c *captchaFailures) increment(userID int) int {
	c.Lock()
	defer c.Unlock()
	c.Failures[userID]++
	fails := c.Failures[userID]
	c.save()
	return fails
}

func (c *captchaFailures) reset(userID int) {
	c.Lock()
	defer c.Unlock()
	delete(c.Failures, userID)
	c.save()
}
