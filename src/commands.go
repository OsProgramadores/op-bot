package main

import (
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
	"math/rand"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Register registers a command a its handler on the bot.
func (x *opBot) Register(cmd string, desc string, adminOnly bool, pvtOnly bool, enabled bool, handler func(tgbotInterface, tgbotapi.Update) error) {
	if x.commands == nil {
		x.commands = map[string]botCommand{}
	}

	x.commands[cmd] = botCommand{
		desc:      desc,
		adminOnly: adminOnly,
		pvtOnly:   pvtOnly,
		enabled:   enabled,
		handler:   handler,
	}
	log.Printf("Registered command %q, %q", cmd, desc)
}

// hackerHandler provides anti-hacker protection to the bot.
func (x *opBot) hackerHandler(bot tgbotInterface, update tgbotapi.Update) error {
	// Gifs for /hackerdetected.
	media := []string{
		// Balaclava guy "hacking".
		"http://i.imgur.com/oubTSqS.gif",
		// "Hacker" with gas mask.
		"http://i.imgur.com/m4rP3jK.gif",
		// "Anonymous hacker" kissed by mom.
		"http://i.imgur.com/LPn1Ya9.gif",
	}

	// Remove message that triggered /hackerdetected command.
	if _, err := bot.DeleteMessage(tgbotapi.DeleteMessageConfig{
		ChatID:    update.Message.Chat.ID,
		MessageID: update.Message.MessageID,
	}); err != nil {
		log.Printf("Error deleting message %v on chat id %v", update.Message.MessageID, update.Message.Chat.ID)
	}

	// Selects randomly one of the available media and send it.
	// Here we are generating an integer in [0, len(media)).
	x.media.sendMedia(bot, update, media[rand.Int()%len(media)])

	// No need to report on errors.
	return nil
}

// helpHandler sends a help message back to the user.
func (x *opBot) helpHandler(bot tgbotInterface, update tgbotapi.Update) error {
	var helpMsg []string
	for c, bcmd := range x.commands {
		admin := ""
		if bcmd.adminOnly {
			admin = " (Admin)"
		}
		if bcmd.enabled {
			helpMsg = append(helpMsg, fmt.Sprintf("/%s: %s%s", c, bcmd.desc, admin))
		}
	}

	// Predictable order.
	sort.Strings(helpMsg)
	sendReply(bot, update.Message.Chat.ID, update.Message.MessageID, strings.Join(helpMsg, "\n"))
	return nil
}

// setWelcomeMessageTTLHandler sets the time (in minutes) welcome messages
// should live before being automatically removed. A value of 0 disables the
// feature.
func (x *opBot) setWelcomeMessageTTLHandler(bot tgbotInterface, update tgbotapi.Update) error {
	d, err := parseCmdDuration(update.Message.Text)
	if err != nil {
		return err
	}

	x.welcomeMessageTTL = d

	var note string
	if d.Seconds() <= 0 {
		note = " (disabled)"
	}
	reply, err := sendReply(bot, update.Message.Chat.ID, update.Message.MessageID, fmt.Sprintf("Welcome message TTL set to: %v%s", d, note))
	if err != nil {
		return err
	}
	selfDestructMessage(bot, reply.Chat.ID, reply.MessageID, 0)
	return nil
}

// setCaptchaTimeHandler sets the time we give new users to respond to the captcha before they
// get kicked from the group.
func (x *opBot) setCaptchaTimeHandler(bot tgbotInterface, update tgbotapi.Update) error {
	d, err := parseCmdDuration(update.Message.Text)
	if err != nil {
		return err
	}

	x.captchaTime = d

	var note string
	if d.Seconds() <= 0 {
		note = " (disabled)"
	}
	reply, err := sendReply(bot, update.Message.Chat.ID, update.Message.MessageID, fmt.Sprintf("Captcha time set to: %v%s", d, note))
	if err != nil {
		return err
	}
	selfDestructMessage(bot, reply.Chat.ID, reply.MessageID, 0)
	return nil
}

// setNewUserProbationTimeHandler sets the time we consider new users as "new".
// This sets a number of posting restrictions. This function uses ParseDuration
// to parse the time. Set a suffix of 'h' to indicate hours (E.g, 8h). A value
// of zero disables the feature. The minimum value is 1h (to prevent mistakes
// such as setting the value too low.)
func (x *opBot) setNewUserProbationTimeHandler(bot tgbotInterface, update tgbotapi.Update) error {
	d, err := parseCmdDuration(update.Message.Text)
	if err != nil {
		return err
	}

	if d.Hours() < 1.0 && d.Hours() != 0 {
		d = time.Duration(1 * time.Hour)
	}
	x.config.NewUserProbationTime = duration{d}

	note := "disabled"
	if d.Seconds() > 0 {
		note = fmt.Sprintf("set to %s", d)
	}
	reply, err := sendReply(bot, update.Message.Chat.ID, update.Message.MessageID, fmt.Sprintf("New User Probation time %s", note))
	if err != nil {
		return err
	}
	selfDestructMessage(bot, reply.Chat.ID, reply.MessageID, 0)
	return nil
}

// parseCmdDuration returns the time passed to a text message command. E.g: the command
// "/whatever 10h", will return a time.Duration of 10h. The function ignores
// the command itself.
func parseCmdDuration(text string) (time.Duration, error) {
	re := regexp.MustCompile(`/[a-zA-Z_-]+\s+(.+)`)
	groups := re.FindStringSubmatch(text)
	if len(groups) < 1 || groups[1] == "" {
		return 0, fmt.Errorf("unable to find the duration in %q", text)
	}
	value := groups[1]

	d, err := time.ParseDuration(value)
	if err != nil || d.Seconds() < 0 {
		return 0, fmt.Errorf("invalid time specification: %s", value)
	}
	return d, nil
}
