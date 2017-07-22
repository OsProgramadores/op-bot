package main

import (
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
	"os"
	"strings"
)

// botCommands holds the commands accepted by the bot, their description and a handler function.
type botCommand struct {
	cmd     string
	desc    string
	pvtOnly bool
	handler func(tgbotapi.Update, *tgbotapi.BotAPI, botConfig, botMessages) error
}

var (
	commands []botCommand
)

func init() {
	commands = []botCommand{
		botCommand{"indent", "Indenta um programa no repl.it (/indent url)", false, indentHandler},
		botCommand{"hackerdetected", "Dispara o alarme anti-hacker. :)", false, hackerHandler},
		botCommand{"setlocation", "Atualiza posição geográfica usando código postal (/setlocation <pais> <código postal>)", true, locationHandler},
		botCommand{"cep", "Atualiza posição geográfica usando CEP", true, locationHandler},
		botCommand{"help", "Mensagem de help", true, helpHandler},
	}
}

// runBot is the main message dispatcher for the bot.
func runBot(config botConfig, bot *tgbotapi.BotAPI) {
	msgs, err := loadMessages()
	if err != nil {
		fmt.Println("Unable to load messages file: ", err)
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

			//Location.
			case update.Message.Location != nil:
				user := update.Message.From
				location := update.Message.Location
				err := handleLocation(config.LocationKey, fmt.Sprintf("%d", user.ID), location.Latitude, location.Longitude)

				// Give feedback to user, if message was sent privately.
				if isPrivateChat(update.Message.Chat) {
					message := msgs.LocationSuccess
					if err != nil {
						message = msgs.LocationFail
					}
					sendReply(update.Message.Chat.ID, update.Message.MessageID, message, bot)
				}

			// Join event.
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
				sendReplyWithMarkup(update.Message.Chat.ID,
					update.Message.MessageID,
					fmt.Sprintf(msgs.Welcome, name),
					markup,
					bot)

			// User commands.
			case update.Message.IsCommand():
				cmd := strings.ToLower(update.Message.Command())
				found := false

				for _, c := range commands {
					if c.cmd == cmd {
						found = true

						// Fail silently if non-private request on private only command.
						if c.pvtOnly && !isPrivateChat(update.Message.Chat) {
							log.Printf("Ignoring non-private request on private only command %q", cmd)
							break
						}
						// Handle command.
						err := c.handler(update, bot, config, msgs)
						if err != nil {
							sendReply(update.Message.Chat.ID, update.Message.MessageID, err.Error(), bot)
						}
						break
					}
				}
				if !found {
					log.Printf("Ignoring invalid command %q", cmd)
				}
			}
		}
	}
}

// hackerHandler provides anti-hacker protection to the bot.
func hackerHandler(update tgbotapi.Update, bot *tgbotapi.BotAPI, config botConfig, msgs botMessages) error {
	// This gif is available at http://i.imgur.com/LPn1Ya9.gif.
	// Below we have a (bot-specific) Telegram document ID for it.
	// It works for @osprogramadores_bot.
	hackerGif := "CgADAQADFAADczjpRo3QR3X-LC5EAg"

	gif := tgbotapi.NewDocumentShare(update.Message.Chat.ID, hackerGif)

	// Reply to quoted message, if any.
	if update.Message.ReplyToMessage != nil {
		gif.ReplyToMessageID = update.Message.ReplyToMessage.MessageID
	}

	// Remove message that triggered /hackerdetected command.
	toDelete := tgbotapi.DeleteMessageConfig{ChatID: update.Message.Chat.ID, MessageID: update.Message.MessageID}
	bot.DeleteMessage(toDelete)
	bot.Send(gif)

	return nil
}

// helpHandler sends a help message back to the user.
func helpHandler(update tgbotapi.Update, bot *tgbotapi.BotAPI, config botConfig, messages botMessages) error {
	msgs := make([]string, len(commands))
	for i, c := range commands {
		msgs[i] = fmt.Sprintf("/%s: %s", c.cmd, c.desc)
	}

	sendReply(update.Message.Chat.ID, update.Message.MessageID, strings.Join(msgs, "\n"), bot)

	return nil
}

// indentHandler indents the code in a repl.it URL.
func indentHandler(update tgbotapi.Update, bot *tgbotapi.BotAPI, config botConfig, msgs botMessages) error {
	args := strings.Trim(update.Message.CommandArguments(), " ")

	repl, err := handleReplItURL(&runner{}, args)

	if err != nil {
		return err
	}

	msg := fmt.Sprintf("Acesse a versão indentada em %s. Lembre que a última revisão sempre está disponível em https://repl.it/%s/latest.", repl.newURL, repl.SessionID)
	sendReply(update.Message.Chat.ID, update.Message.MessageID, msg, bot)
	return nil
}

// isPrivateChat returns true if a chat is private.
func isPrivateChat(chat *tgbotapi.Chat) bool {
	return chat.Type == "private"
}

// formatName returns the user full name in the form "Firstname Lastname".
func formatName(user tgbotapi.User) string {
	firstName := user.FirstName
	lastName := user.LastName

	return strings.Trim(fmt.Sprintf("%s %s", firstName, lastName), " ")
}

// sendReply sends a reply to a specific MessageID.
func sendReply(dest int64, replyToID int, text string, bot *tgbotapi.BotAPI) (tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(dest, text)
	msg.ReplyToMessageID = replyToID
	return bot.Send(msg)
}

// sendReplyWithMarkup sends a reply to a specific MessageID with markup.
func sendReplyWithMarkup(dest int64, replyToID int, text string, markup tgbotapi.InlineKeyboardMarkup, bot *tgbotapi.BotAPI) (tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(dest, text)
	msg.ReplyToMessageID = replyToID
	msg.ReplyMarkup = &markup
	return bot.Send(msg)
}

// rulesButton creates a button with the "rules" label.
func rulesButton(messages botMessages) tgbotapi.InlineKeyboardButton {
	return tgbotapi.NewInlineKeyboardButtonData(messages.ReadTheRules, "rules")
}

// websiteButton creates a button with a preformatted message.
func websiteButton(messages botMessages) tgbotapi.InlineKeyboardButton {
	return tgbotapi.NewInlineKeyboardButtonURL(messages.VisitOurGroupWebsite, messages.URL)
}
