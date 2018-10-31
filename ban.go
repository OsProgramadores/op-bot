package main

import (
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
	"strings"
	"sync"
)

const (
	// File to store info on the requested bans.
	requestedBansDB = "bans.json"
	// Threshold to notify admins. If the offending message reaches this number
	// of reports, the admins are notified.
	adminNotificationDefaultThreshold = 1
	// Parsing mode: Markdown.
	parseModeMarkdown = "Markdown"
)

// banRequest stores info on each ban requested via the /ban command.
type banRequest struct {
	// The ID of the offending message.
	MessageID int64 `json:"message"`
	// The ID of the group/channel in which the message has been reported.
	ChatID int64 `json:"chat"`
	// ID of the author of the message.
	Author int64 `json:"author"`
	// Text of the offending message, without considering any captions.
	Text string `json:"text"`
	// Map of users who reported this message, along with the ID of the report
	// message.
	Reporters map[int64]int64 `json:"reporters"`
	// Map of notifications sent about this message. Here we have the ID of the
	// admin notified and the ID of the notification (message) sent.
	// The additional media message -- in case there was media in the message
	// being reported -- will not be changed, and stays so it is easy to see
	// what the report was about once it has been deleted.
	Notifications map[int64]int64 `json:"notifications"`
	// Indicates whether this message has been removed by an admin.
	MessageRemoved bool `json:"removed"`
	// Admin who removed the message.
	RemovedBy int64 `json:"handler"`
}

// banRequestList stores the list of bans requested alongside the threshold for
// notifying the admins.
type banRequestList struct {
	// NotificationThreshold indicates the number of reports we need in order to
	// notify the admins about a reported message.
	NotificationThreshold int `json:"threshold"`
	// Bans has as a key a combination of the ID of the reporting message with
	// the ID of the group/channel this message happened. This is mapped to a
	// banRequest struct, which contains additional info on the report itself.
	Bans map[string]banRequest `json:"bans"`
}

type requestedBans struct {
	sync.RWMutex
	// List of requested bans alongside the threshold for notifying the admins.
	Requests banRequestList
}

// nolint: gocyclo
// banRequestHandler does it all.
func (x *opBot) banRequestHandler(update tgbotapi.Update) error {
	// This command is not supposed to be issued in private.
	if update.Message == nil || update.Message.Chat == nil || isPrivateChat(update.Message.Chat) {
		log.Printf("banRequestHandler: either i) the message or chat was nil or ii) this command was issued in a private message")
		return nil
	}

	// If there is no message being replied to, we have no idea what exactly
	// the user is reporting, so just ignore.
	if update.Message.ReplyToMessage == nil {
		log.Printf("banRequestHandler: /ban called by %q without being in response to a message; ignoring.", update.Message.From)
		return nil
	}

	offendingMessageID := update.Message.ReplyToMessage.MessageID

	x.modules.reportedBans.Lock()
	defer x.modules.reportedBans.Unlock()

	key := fmt.Sprintf("%d:%d", offendingMessageID, update.Message.Chat.ID)
	if _, ok := x.modules.reportedBans.Requests.Bans[key]; ok {
		if x.modules.reportedBans.Requests.Bans[key].MessageRemoved {
			// Message was already removed. Not sure this should happen, but
			// let's just return here anyway.
			log.Printf("banRequestHandler: /ban called by %q on message %q, but it was already removed; ignoring.", update.Message.From, offendingMessageID)
			return nil
		}
	} else {
		report := banRequest{
			Author:         int64(update.Message.ReplyToMessage.From.ID),
			ChatID:         update.Message.Chat.ID,
			MessageID:      int64(offendingMessageID),
			MessageRemoved: false,
			Notifications:  map[int64]int64{},
			RemovedBy:      0,
			Reporters:      map[int64]int64{},
		}
		x.modules.reportedBans.Requests.Bans[key] = report
		if err := safeWriteJSON(x.modules.reportedBans.Requests, requestedBansDB); err != nil {
			log.Printf("banRequestHandler: problem updating file %q after adding new request %v: %v", requestedBansDB, report, err)
			return nil
		}
	}

	// Message is still around, so let's decide what to do.
	// Let's start by adding (or updating) the Reporters list, to add info
	// on the person reporting it this time.
	x.modules.reportedBans.Requests.Bans[key].Reporters[int64(update.Message.From.ID)] = int64(update.Message.MessageID)
	if err := safeWriteJSON(x.modules.reportedBans.Requests, requestedBansDB); err != nil {
		log.Printf("banRequestHandler: problem updating the file %q after updating list of ban reporters for message %q in %q: %v", requestedBansDB, x.modules.reportedBans.Requests.Bans[key].MessageID, x.modules.reportedBans.Requests.Bans[key].ChatID, err)
		return nil
	}

	// If we haven't notified the admins yet *and* the threshold has been
	// met, notify them now!
	if len(x.modules.reportedBans.Requests.Bans[key].Notifications) == 0 && len(x.modules.reportedBans.Requests.Bans[key].Reporters) >= x.modules.reportedBans.Requests.NotificationThreshold {
		chatConfig := tgbotapi.ChatConfig{ChatID: update.Message.Chat.ID}
		admins, err := x.bot.GetChatAdministrators(chatConfig)
		if err != nil {
			log.Printf("banRequestHandler: problem trying to get chat administrators: %v", err)
			return nil
		}

		for _, admin := range admins {
			msgid, err := notifyAdmin(x, admin.User, update)
			if err != nil {
				log.Printf("banRequestHandler: problem notifying admin %q (uid: %q): %v", formatName(*admin.User), admin.User.ID, err)
				continue
			}
			// Store the admin who was notified alongside with the ID of the
			// notification.
			x.modules.reportedBans.Requests.Bans[key].Notifications[int64(admin.User.ID)] = msgid
		}

		// Update file if we have sent any notifcations.
		if len(x.modules.reportedBans.Requests.Bans[key].Notifications) > 0 {
			report, _ := x.modules.reportedBans.Requests.Bans[key]
			// Since we are going to update the info on disk, we take the
			// opportunity to also save the content of the offending message.
			// We use this information when updating the notifications, once an
			// admin has made a decision.
			report.Text = update.Message.ReplyToMessage.Text
			x.modules.reportedBans.Requests.Bans[key] = report
			if err := safeWriteJSON(x.modules.reportedBans.Requests, requestedBansDB); err != nil {
				log.Printf("banRequestHandler: problem updating file %q after sending notifications to admins regarding offending message %q in %q: %v", requestedBansDB, x.modules.reportedBans.Requests.Bans[key].MessageID, x.modules.reportedBans.Requests.Bans[key].ChatID, err)
				return nil
			}
		}
	}

	return nil
}

// loadBanRequestsInfo loads the requested bans from the disk.
func loadBanRequestsInfo(b *requestedBans) error {
	b.Lock()
	defer b.Unlock()

	return readJSONFromDataDir(&b.Requests, requestedBansDB)
}

// notifyAdmin notifies `admin' on the reported message, giving the following
// options:
// - go to message (if in a public group/channel);
// - remove the offending message;
// - remove the offending message and ban its author.
// It returns the id of the notification message sent.
func notifyAdmin(x *opBot, admin *tgbotapi.User, update tgbotapi.Update) (int64, error) {
	offendingMessageID := update.Message.ReplyToMessage.MessageID
	chatID := update.Message.Chat.ID

	requestID := fmt.Sprintf("%d:%d", offendingMessageID, chatID)

	removeMessageButton := button(T("remove_message"), fmt.Sprintf("delete-message-%s", requestID))
	removeMessageAndBanUserButton := button(T("remove_message_and_ban"), fmt.Sprintf("ban-user-%s", requestID))

	var markup tgbotapi.InlineKeyboardMarkup

	// Links won't work if there is no username.
	if len(update.Message.Chat.UserName) > 0 {
		goToMessageButton := buttonURL(T("go_to_notification"), fmt.Sprintf("https://t.me/%s/%d", update.Message.Chat.UserName, update.Message.ReplyToMessage.MessageID))
		markup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(goToMessageButton),
			tgbotapi.NewInlineKeyboardRow(removeMessageButton, removeMessageAndBanUserButton),
		)
	} else {
		markup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(removeMessageButton, removeMessageAndBanUserButton),
		)
	}

	notificationText := fmt.Sprintf(T("notify_admin"), formatName(*update.Message.From), update.Message.From.ID, update.Message.Chat.Title, update.Message.ReplyToMessage.Text)
	// We also replace literal newline `\n` with "\n", so that the lines will
	// actually break, instead of displaying \n's.
	notificationText = strings.Replace(notificationText, `\n`, "\n", -1)

	msg := tgbotapi.NewMessage(int64(admin.ID), notificationText)
	msg.ParseMode = parseModeMarkdown
	msg.ReplyMarkup = &markup

	// Let's create a media message, if there's any media in the reported message.
	mediaMsg, ok, err := createMediaMessage(update.Message.ReplyToMessage, int64(admin.ID), nil)
	if err != nil {
		return 0, err
	}

	// Send the notification message first.
	sentMessage, err := x.bot.Send(msg)
	if err != nil {
		return 0, err
	}

	// Now send the media message, if there is one.
	if ok {
		x.bot.Send(mediaMsg)
	}

	// We return the ID of the notification message, as we may want to update
	// it eventually, such as when it has been dealt with.
	return int64(sentMessage.MessageID), nil
}

// deleteMessageFromBanRequest delestes the offending message, and optionally
// also bans the user who sent it.
func deleteMessageFromBanRequest(x *opBot, admin *tgbotapi.User, requestID string, shouldBanAsWell bool) error {
	err := deleteMessage(x, admin, requestID)
	if err != nil {
		return err
	}

	x.modules.reportedBans.RLock()
	defer x.modules.reportedBans.RUnlock()

	if !shouldBanAsWell {
		// We already did what we were intended to do, so just update the
		// notifications sent and return.
		return updateBanRequestNotification(x, requestID, admin, T("notification_update_delete"))
	}

	memberConfig := tgbotapi.ChatMemberConfig{ChatID: x.modules.reportedBans.Requests.Bans[requestID].ChatID, UserID: int(x.modules.reportedBans.Requests.Bans[requestID].Author)}
	_, err = x.bot.KickChatMember(tgbotapi.KickChatMemberConfig{ChatMemberConfig: memberConfig})
	if err != nil {
		return err
	}
	return updateBanRequestNotification(x, requestID, admin, T("notification_update_delete_and_ban"))
}

// deleteMessage deletes the message indicated by `requestID' and updates the
// information on disk relative to it.
func deleteMessage(x *opBot, admin *tgbotapi.User, requestID string) error {
	x.modules.reportedBans.Lock()
	defer x.modules.reportedBans.Unlock()

	report, ok := x.modules.reportedBans.Requests.Bans[requestID]
	if !ok {
		return fmt.Errorf("request id %q not found", requestID)
	}

	if report.MessageRemoved {
		return fmt.Errorf("message indicated by request %q already handled", requestID)
	}

	_, err := x.bot.DeleteMessage(tgbotapi.DeleteMessageConfig{ChatID: report.ChatID, MessageID: int(report.MessageID)})
	if err != nil {
		return err
	}

	// Now let's update the data in disk.
	report.MessageRemoved = true
	report.RemovedBy = int64(admin.ID)
	x.modules.reportedBans.Requests.Bans[requestID] = report
	return safeWriteJSON(x.modules.reportedBans.Requests, requestedBansDB)
}

// updateBanRequestNotification updates the notifications sent informing the
// decision made and the admin who made it. Locks, if needed, should be taken
// care of outside this function.
func updateBanRequestNotification(x *opBot, requestID string, admin *tgbotapi.User, message string) error {
	report, _ := x.modules.reportedBans.Requests.Bans[requestID]

	notificationMessage := fmt.Sprintf(T("notification_handled"), formatName(*admin), admin.ID, message, report.Text)
	for adminID, notificationID := range report.Notifications {
		editmsg := tgbotapi.NewEditMessageText(adminID, int(notificationID), notificationMessage)
		editmsg.ParseMode = parseModeMarkdown
		x.bot.Send(editmsg)
	}
	return nil
}
