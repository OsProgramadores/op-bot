package main

import (
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
	"strconv"
	"strings"
	"sync"
)

const (
	notificationsDB = "notifications.json"
)

// notifications maps user `ids' -> `usernames' and `usernames' -> `user ids'.
type notifications struct {
	sync.RWMutex
	Users map[string]string `json:"users"`
}

// notificationHandler enables/disables user notifications.
func (x *opBot) notificationHandler(update tgbotapi.Update) error {
	uidstr := fmt.Sprintf("%d", update.Message.From.ID)

	if err := toggleNotifications(&x.modules.userNotifications, uidstr, update.Message.From.UserName); err != nil {
		return fmt.Errorf(T("notification_fail"), notificationStatus(&x.modules.userNotifications, uidstr))
	}

	text := fmt.Sprintf(T("notification_success"), notificationStatus(&x.modules.userNotifications, uidstr))
	x.sendReply(update, text)
	return nil
}

// notificationStatus returns a string with the enabled/disabled status of
// notifications for the given user.
func notificationStatus(n *notifications, user string) string {
	if notificationsEnabled(n, user) {
		return T("notifications_enabled")
	}
	return T("notifications_disabled")
}

// notificationsEnabled returns a boolean indicating whether the user passed as
// argument has enabled notifications.
func notificationsEnabled(n *notifications, user string) bool {
	n.RLock()
	defer n.RUnlock()

	_, ok := n.Users[user]
	return ok
}

// toggleNotifications changes the current notification settings for the
// specific user, and save the resulting config to the notificationsDB file.
func toggleNotifications(n *notifications, userid, username string) error {
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

	err := safeWriteJSON(notificationSettings, notificationsDB)

	if err == nil {
		// Save succeeded, let's update our map.
		n.Users = notificationSettings
	}

	return err
}

// loadNotificationSettings loads notifications database from notificationsDB
// file.
func loadNotificationSettings(n *notifications) error {
	n.Lock()
	defer n.Unlock()

	return readJSONFromDataDir(&n.Users, notificationsDB)
}

// idByNotificationUserName queries the notifications settings and returns both
// the user id from the passed username and a boolean indicating whether it
// found the username in question in the database.
func idByNotificationUserName(n *notifications, username string) (string, bool) {
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
func manageNotifications(x *opBot, update tgbotapi.Update) error {
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
		if !ok && notificationsEnabled(&x.modules.userNotifications, uidstr) {
			// Using `true' here because this notification is due to a message
			// being replied to.
			_, err := sendNotification(x, uid, update, true)
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
					username := trDelete(string(runes[entity.Offset:entity.Offset+entity.Length]), "@")

					if notificationsEnabled(&x.modules.userNotifications, username) {
						if _, ok := notified[username]; ok {
							// User was notified already.
							continue
						}

						uidstr, ok := idByNotificationUserName(&x.modules.userNotifications, username)
						if !ok {
							continue
						}
						uid, err := strconv.Atoi(uidstr)
						if err != nil {
							continue
						}
						// Using `false' here because this notification is not
						// due to a  message being replied to.
						_, err = sendNotification(x, uid, update, false)
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
					if notificationsEnabled(&x.modules.userNotifications, uidstr) {
						if _, ok := notified[uidstr]; ok {
							// User was notified already.
							continue
						}

						// Using `false' here because this notification is not
						// due to a message being replied to.
						_, err := sendNotification(x, entity.User.ID, update, false)
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
func sendNotification(x *opBot, recipient int, update tgbotapi.Update, response bool) (tgbotapi.Message, error) {
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
		return x.bot.Send(msg)
	}

	// There's additional media, so send the text message followed by the one
	// containing the media.
	if _, err = x.bot.Send(msg); err != nil {
		return tgbotapi.Message{}, err
	}
	return x.bot.Send(mediaMsg)
}
