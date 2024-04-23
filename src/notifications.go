package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

const (
	notificationsDB = "notifications.json"
)

// notifications maps user `ids' -> `usernames' and `usernames' -> `user ids'.
type notifications struct {
	sync.RWMutex
	Users           map[string]string `json:"users"`
	notificationsDB string
}

// newNotifications creates a new notification type.
func newNotifications() *notifications {
	return &notifications{
		notificationsDB: notificationsDB,
	}
}

// notificationHandler enables/disables user notifications.
func (n *notifications) notificationHandler(bot tgbotInterface, update tgbotapi.Update) error {
	uidstr := fmt.Sprintf("%d", update.Message.From.ID)

	if err := n.toggleNotifications(uidstr, update.Message.From.UserName); err != nil {
		return fmt.Errorf(T("notification_fail"), n.notificationStatus(uidstr))
	}

	text := fmt.Sprintf(T("notification_success"), n.notificationStatus(uidstr))
	sendReply(bot, update.Message.Chat.ID, update.Message.MessageID, text)
	return nil
}

// notificationStatus returns a string with the enabled/disabled status of
// notifications for the given user.
func (n *notifications) notificationStatus(user string) string {
	if n.notificationsEnabled(user) {
		return T("notifications_enabled")
	}
	return T("notifications_disabled")
}

// notificationsEnabled returns a boolean indicating whether the user passed as
// argument has enabled notifications.
func (n *notifications) notificationsEnabled(user string) bool {
	n.RLock()
	defer n.RUnlock()

	_, ok := n.Users[user]
	return ok
}

// toggleNotifications toggles the current notification settings for the
// specific user, and save the resulting config to the notificationsDB file.
func (n *notifications) toggleNotifications(userid, username string) error {
	n.Lock()
	defer n.Unlock()

	// Since the saving to disk may fail, we are not touching the actual map
	// here; instead, we copy the data and use this copy. If all goes well, we
	// update the actual map with this copy, to keep consistency.
	notificationSettings := map[string]string{}
	for k, v := range n.Users {
		notificationSettings[k] = v
	}

	savedUserName, ok := notificationSettings[userid]
	if ok {
		delete(notificationSettings, userid)
		if len(savedUserName) > 0 {
			delete(notificationSettings, savedUserName)
		}
	} else {
		notificationSettings[userid] = username
		if len(username) > 0 {
			notificationSettings[username] = userid
		}
	}

	err := safeWriteJSON(notificationSettings, n.notificationsDB)

	if err == nil {
		// Save succeeded, update map.
		n.Users = notificationSettings
	}

	return err
}

// loadNotificationSettings loads notifications database from the
// notificationsDB file.
func (n *notifications) loadNotificationSettings() error {
	n.Lock()
	defer n.Unlock()
	return readJSONFromDataDir(&n.Users, n.notificationsDB)
}

// idByNotificationUserName queries the notifications settings and returns both
// the user id from the passed username and a boolean indicating whether it
// found the username in question in the database.
func (n *notifications) idByNotificationUserName(username string) (string, bool) {
	n.RLock()
	defer n.RUnlock()

	if len(username) == 0 {
		return "", false
	}

	uidstr, ok := n.Users[username]
	return uidstr, ok
}

// nolint: gocyclo
// manageNotifications notifies users based on the message being replied to and
// user mentions, if the user has notifications enabled.
func (n *notifications) manageNotifications(bot sender, update tgbotapi.Update) error {
	if isPrivateChat(update.Message.Chat) {
		return nil
	}

	if update.Message.ReplyToMessage == nil && update.Message.Entities == nil {
		return nil
	}

	// Since we may have repeated mentions and/or have messages being replied
	// to, let's keep track of who was notified, so that we only notify the
	// same person once.
	notified := map[string]bool{}

	// Let's notify the user from the message being replied to, if notifications
	// are enabled for him/her.
	if update.Message.ReplyToMessage != nil {
		uid := update.Message.ReplyToMessage.From.ID
		uidstr := fmt.Sprintf("%d", uid)

		_, ok := notified[uidstr]
		if !ok && n.notificationsEnabled(uidstr) {
			// Using `true' here because this notification is due to a message
			// being replied to.
			_, err := sendNotification(bot, uid, update, true)
			if err != nil {
				log.Printf("Failed to send notification to %q (%d)", formatName(*update.Message.From), update.Message.From.ID)
			} else {
				// Let's keep track of this notified user.
				notified[uidstr] = true
				if len(update.Message.ReplyToMessage.From.UserName) > 0 {
					notified[update.Message.ReplyToMessage.From.UserName] = true
				}
			}
		}
	}

	// Now let's notify the users being mentioned, if they have enabled
	// notifications.
	if update.Message.Entities != nil {
		for _, entity := range *update.Message.Entities {
			if entity.Type == "mention" || entity.Type == "text_mention" {
				if entity.Type == "mention" {
					// The text may contain UTF-8 literals, so we need to
					// convert it to runes to be able to use the offset and
					// length properly.
					runes := []rune(update.Message.Text)
					// Sanity check.
					if entity.Offset+entity.Length > cap(runes) {
						// This should not happen, so let's log it and leave.
						log.Printf("Failed to parse username from runes: capacity: %d, offset: %d, length: %d; update message: %+v", cap(runes), entity.Offset, entity.Length, update.Message)
						continue
					}
					username := trDelete(string(runes[entity.Offset:entity.Offset+entity.Length]), "@")

					if n.notificationsEnabled(username) {
						if _, ok := notified[username]; ok {
							// User was notified already.
							continue
						}

						uidstr, ok := n.idByNotificationUserName(username)
						if !ok {
							continue
						}
						uid, err := strconv.Atoi(uidstr)
						if err != nil {
							continue
						}
						// Using `false' here because this notification is not
						// due to a  message being replied to.
						_, err = sendNotification(bot, uid, update, false)
						if err != nil {
							log.Printf("Failed to send notification to %q (%d)", formatName(*update.Message.From), update.Message.From.ID)
						} else {
							// Let's keep track of this notified user.
							notified[username] = true
							notified[uidstr] = true
						}
					}
				} else {
					// entity.Type == "text_mention".
					uidstr := fmt.Sprintf("%d", entity.User.ID)
					if n.notificationsEnabled(uidstr) {
						if _, ok := notified[uidstr]; ok {
							// User was notified already.
							continue
						}

						// Using `false' here because this notification is not
						// due to a message being replied to.
						_, err := sendNotification(bot, entity.User.ID, update, false)
						if err != nil {
							log.Printf("Failed to send notification to %q (%d)", formatName(*update.Message.From), update.Message.From.ID)
						} else {
							// Let's keep track of this notified user.
							notified[uidstr] = true
							if len(entity.User.UserName) > 0 {
								notified[entity.User.UserName] = true
							}
						}
					}
				}
			}
		}
	}
	return nil
}

// sendNotification notifies the specified recipient. `response' indicates
// whether this notification is due to a message being replied to, in which case
// it will be `true', or if it is due to a mention, in which case it will be
// `false'.
func sendNotification(bot sender, recipient int, update tgbotapi.Update, response bool) (tgbotapi.Message, error) {
	if recipient == update.Message.From.ID {
		log.Printf("Not sending notification to %d, the same sender of the message", recipient)
		return tgbotapi.Message{}, nil
	}

	notificationMsg := T("notification_mentioned")
	if response {
		notificationMsg = T("notification_replied")
	}

	notificationText := fmt.Sprintf(notificationMsg, formatName(*update.Message.From), update.Message.From.ID, update.Message.Chat.Title, update.Message.Text)

	// We also replace literal newline `\n` with "\n", so that the lines will
	// actually break, instead of displaying \n's.
	notificationText = strings.Replace(notificationText, `\n`, "\n", -1)

	// Now the markup, which will contain a link to the message, if applicable.
	var markup *tgbotapi.InlineKeyboardMarkup
	// Links won't work if there is no username.
	if len(update.Message.Chat.UserName) > 0 {
		inlineKeyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(buttonURL(T("go_to_notification"), fmt.Sprintf("https://t.me/%s/%d", update.Message.Chat.UserName, update.Message.MessageID))),
		)
		markup = &inlineKeyboard
	}

	destination := int64(recipient)
	msg := tgbotapi.NewMessage(destination, notificationText)
	msg.ParseMode = parseModeMarkdown

	// Now let's check if there is additional media to send as well.
	mediaMsg, ok, err := createMediaMessage(update.Message, destination, markup)
	if err != nil || !ok {
		if err != nil {
			log.Printf("error creating media message: %v\n", err)
		}
		if markup != nil {
			msg.ReplyMarkup = markup
		}
		return bot.Send(msg)
	}

	// There's additional media, so send the text message followed by the one
	// containing the media.
	if _, err = bot.Send(msg); err != nil {
		return tgbotapi.Message{}, err
	}
	return bot.Send(mediaMsg)
}
