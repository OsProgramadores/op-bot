package main

import (
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
	"regexp"
	"strings"
)

func answerCallbackWithNotification(x *opBot, callbackID, text string) error {
	_, err := x.bot.AnswerCallbackQuery(
		tgbotapi.CallbackConfig{
			CallbackQueryID: callbackID,
			Text:            text,
			ShowAlert:       true,
		},
	)
	return err
}

func extractRequestID(data, prefix, errorMsg string) (string, error) {
	regex := regexp.MustCompile(fmt.Sprintf(`%s-(\S+)`, prefix))
	matches := regex.FindStringSubmatch(data)
	if matches == nil {
		err := fmt.Errorf("%s: %q", errorMsg, data)
		log.Printf("extractRequestID: %v", err)
		return "", err
	}
	return matches[1], nil
}

func handleCallbackQuery(x *opBot, update tgbotapi.Update) error {
	data := update.CallbackQuery.Data

	switch {
	case data == "rules":
		answerCallbackWithNotification(x, update.CallbackQuery.ID, T("rules"))
	case strings.HasPrefix(data, "ban-user-"):
		requestID, err := extractRequestID(data, "ban-user", "malformed ban request callback query")
		if err != nil {
			answerCallbackWithNotification(x, update.CallbackQuery.ID, T("callback_invalid_request"))
			break
		}
		responseMessage := T("delete_and_ban_success")
		// We pass `true' as parameter to indicate we want to ban the user as well.
		if deleteMessageFromBanRequest(x, update.CallbackQuery.From, requestID, true) != nil {
			responseMessage = T("delete_and_ban_fail")
		}
		answerCallbackWithNotification(x, update.CallbackQuery.ID, responseMessage)
	case strings.HasPrefix(data, "delete-message-"):
		requestID, err := extractRequestID(data, "delete-message", "malformed delete message request callback query")
		if err != nil {
			answerCallbackWithNotification(x, update.CallbackQuery.ID, T("callback_invalid_request"))
			break
		}
		responseMessage := T("delete_message_success")
		// We pass `false' here to indicate we don't want to also ban the user.
		if deleteMessageFromBanRequest(x, update.CallbackQuery.From, requestID, false) != nil {
			responseMessage = T("delete_message_fail")
		}
		answerCallbackWithNotification(x, update.CallbackQuery.ID, responseMessage)
	}
	return nil
}
