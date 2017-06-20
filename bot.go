package main

import (
	"fmt"
	"gopkg.in/telegram-bot-api.v4"
	"log"
	"os"
	"strings"
)

func runBot(config botConfig, bot *tgbotapi.BotAPI) {
	msgs, err := loadMessages()
	if err != nil {
		fmt.Println("Houston, we have a problem: ", err)
		fmt.Println("You can see an example of bot messages file at 'config/messages.json.sample'")
		os.Exit(1)
	}

	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, _ := bot.GetUpdatesChan(u)

	for update := range updates {
		switch {
		case update.CallbackQuery != nil:
			switch update.CallbackQuery.Data {
			case "rules":
				bot.AnswerCallbackQuery(
					tgbotapi.CallbackConfig{
						CallbackQueryID: update.CallbackQuery.ID,
						Text:            msgs.Rules,
						ShowAlert:       true,
						CacheTime:       60,
					},
				)
			}

		case update.Message != nil:
			switch {
			// join event.
			case update.Message.NewChatMember != nil:
				name := formatName(update)

				markup := tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(websiteButton(msgs)),
					tgbotapi.NewInlineKeyboardRow(rulesButton(msgs)),
				)
				sendReplyWithMarkup(update.Message.Chat.ID, update.Message.MessageID, updateMsg(msgs.Welcome, name), markup, bot)
			}
		}
	}
}

func formatName(update tgbotapi.Update) string {
	uid := update.Message.NewChatMember.ID
	// The following ones are optional.
	username := update.Message.NewChatMember.UserName
	firstName := update.Message.NewChatMember.FirstName
	lastName := update.Message.NewChatMember.LastName

	mention := fmt.Sprintf("user id %d", uid)
	if len(username) > 0 {
		mention = fmt.Sprintf("@%s", username)
	}

	name := strings.Trim(fmt.Sprintf("%s %s", firstName, lastName), " ")
	if len(name) == 0 {
		return mention
	}

	return fmt.Sprintf("%s (%s)", name, mention)
}

func updateMsg(src, target string) string {
	return strings.Replace(src, "%s", target, -1)
}

func sendMessage(dest int64, text string, bot *tgbotapi.BotAPI) (tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(dest, text)
	return bot.Send(msg)
}

func sendReply(dest int64, replyToID int, text string, bot *tgbotapi.BotAPI) (tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(dest, text)
	msg.ReplyToMessageID = replyToID
	return bot.Send(msg)
}

func sendReplyWithMarkup(dest int64, replyToID int, text string, markup tgbotapi.InlineKeyboardMarkup, bot *tgbotapi.BotAPI) (tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(dest, text)
	msg.ReplyToMessageID = replyToID
	msg.ReplyMarkup = &markup
	return bot.Send(msg)
}

func rulesButton(messages botMessages) tgbotapi.InlineKeyboardButton {
	return tgbotapi.NewInlineKeyboardButtonData(messages.ReadTheRules, "rules")
}

func websiteButton(messages botMessages) tgbotapi.InlineKeyboardButton {
	return tgbotapi.NewInlineKeyboardButtonURL(messages.VisitOurGroupWebsite, messages.URL)
}
