package main

import (
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
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
			//Location
			case update.Message.Location != nil:
				user := update.Message.From
				location := update.Message.Location
				handleLocation(config.LocationKey, fmt.Sprintf("%d", user.ID), location.Latitude, location.Longitude)

			// join event.
			case update.Message.NewChatMembers != nil:
				names := make([]string, len(*update.Message.NewChatMembers))
				for index, user := range *update.Message.NewChatMembers {
					names[index] = formatName(user)
				}
				name := strings.Join(names, ", ")

				markup := tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(websiteButton(msgs)),
					tgbotapi.NewInlineKeyboardRow(rulesButton(msgs)),
				)
				sendReplyWithMarkup(update.Message.Chat.ID, update.Message.MessageID, updateMsg(msgs.Welcome, name), markup, bot)
			case update.Message.IsCommand():
				handleCommands(update, bot)
			}
		}
	}
}

func handleCommands(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	cmd := update.Message.Command()
	args := strings.Trim(update.Message.CommandArguments(), " ")

	switch strings.ToLower(cmd) {
	case "indent":
		// Command: /indent replit_url
		repl, err := handleReplItURL(&runner{}, args)

		if err != nil {
			sendReply(update.Message.Chat.ID, update.Message.MessageID, err.Error(), bot)
			return err
		}

		msg := fmt.Sprintf("Acesse a versão indentada em %s. Lembre que a última revisão sempre está disponível em https://repl.it/%s/latest.", repl.newURL, repl.SessionID)
		sendReply(update.Message.Chat.ID, update.Message.MessageID, msg, bot)

	case "hackerdetected":
		// This gif is available at http://i.imgur.com/LPn1Ya9.gif.
		// Below we have a (bot-specific) Telegram document ID for it.
		// It works for @osprogramadores_bot.
		hackerGif := "CgADAQADFAADczjpRo3QR3X-LC5EAg"

		gif := tgbotapi.NewDocumentShare(update.Message.Chat.ID, hackerGif)

		replyTo := update.Message.MessageID
		if update.Message.ReplyToMessage != nil {
			replyTo = update.Message.ReplyToMessage.MessageID
		}
		gif.ReplyToMessageID = replyTo
		bot.Send(gif)
	}
	return nil
}

func formatName(user tgbotapi.User) string {
	firstName := user.FirstName
	lastName := user.LastName

	return strings.Trim(fmt.Sprintf("%s %s", firstName, lastName), " ")
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
