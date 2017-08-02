package main

import (
	"encoding/json"
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
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

	if err := toggleNotifications(&x.userNotifications, uidstr, update.Message.From.UserName); err != nil {
		return fmt.Errorf(T("notification_fail"), notificationStatus(&x.userNotifications, uidstr))
	}

	text := fmt.Sprintf(T("notification_success"), notificationStatus(&x.userNotifications, uidstr))
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

	err := saveNotifications(notificationSettings)

	if err == nil {
		// Save succeded, let's update our map.
		n.Users = notificationSettings
	}

	return err
}

// saveNotification saves notifications settings to notificationsDB file.
func saveNotifications(settings map[string]string) error {
	buf, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	datadir, err := dataDir()
	if err != nil {
		return err
	}

	err = os.MkdirAll(datadir, 0755)
	if err != nil {
		return err
	}

	tmpfile, err := ioutil.TempFile(datadir, "temp-notification")
	if err != nil {
		log.Printf("Error creating temp file to save notification settings: %v", err)
		return err
	}
	defer os.Remove(tmpfile.Name())

	if _, err = tmpfile.Write(buf); err != nil {
		log.Printf("Error writing notification settings to temp file: %v", err)
		return err
	}

	if err = tmpfile.Close(); err != nil {
		log.Printf("Error closing temp file with notification settings: %v", err)
		return err
	}

	f := filepath.Join(datadir, notificationsDB)
	return os.Rename(tmpfile.Name(), f)
}

// loadNotificationSettings loads notifications database from notificationsDB
// file.
func loadNotificationSettings(n *notifications) error {
	n.Lock()
	defer n.Unlock()

	datadir, err := dataDir()
	if err != nil {
		return err
	}
	f := filepath.Join(datadir, notificationsDB)

	buf, err := ioutil.ReadFile(f)
	if err != nil {
		return err
	}

	return json.Unmarshal(buf, &n.Users)
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

// manageNotifications is going to notify users based on the message being
// replied to and user mentions, based on whether such users have enabled
// notifications.
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
		if !ok && notificationsEnabled(&x.userNotifications, uidstr) {
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
					username := trDelete(update.Message.Text[entity.Offset:entity.Offset+entity.Length], "@")

					if notificationsEnabled(&x.userNotifications, username) {
						if _, ok := notified[username]; ok {
							// User was notified already.
							continue
						}

						uidstr, ok := idByNotificationUserName(&x.userNotifications, username)
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
					if notificationsEnabled(&x.userNotifications, uidstr) {
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

	notificationText := fmt.Sprintf(notificationMsg, update.Message.Chat.Title, formatName(*update.Message.From), update.Message.Text)

	msg := tgbotapi.NewMessage(int64(recipient), notificationText)
	msg.ParseMode = "Markdown"

	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(buttonURL(T("go_to_notification"), fmt.Sprintf("https://t.me/%s/%d", update.Message.Chat.UserName, update.Message.MessageID))),
	)
	msg.ReplyMarkup = &markup

	return x.bot.Send(msg)
}
