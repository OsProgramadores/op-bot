package main

import (
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
	"strings"
)

const (
	// osProgramadoresURL contains the main group URL.
	osProgramadoresURL = "https://osprogramadores.com"
)

// opBot defines an instance of op-bot
type opBot struct {
	config   botConfig
	commands map[string]botCommand
	bot      *tgbotapi.BotAPI
}

// botCommands holds the commands accepted by the bot, their description and a handler function.
type botCommand struct {
	desc    string
	pvtOnly bool
	enabled bool
	handler func(tgbotapi.Update) error
}

// Run is the main message dispatcher for the bot.
func (x *opBot) Run() {
	x.bot.Debug = true
	log.Printf("Authorized on account %s", x.bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, _ := x.bot.GetUpdatesChan(u)

	for update := range updates {
		switch {
		case update.CallbackQuery != nil:
			switch update.CallbackQuery.Data {
			case "rules":
				x.bot.AnswerCallbackQuery(
					tgbotapi.CallbackConfig{
						CallbackQueryID: update.CallbackQuery.ID,
						Text:            T("rules"),
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
				err := handleLocation(x.config.LocationKey, fmt.Sprintf("%d", user.ID), location.Latitude, location.Longitude)

				// Give feedback to user, if message was sent privately.
				if isPrivateChat(update.Message.Chat) {
					message := T("location_success")
					if err != nil {
						message = T("location_fail")
					}
					x.sendReply(update, message)
				}

			// Join event.
			case update.Message.NewChatMembers != nil:
				names := make([]string, len(*update.Message.NewChatMembers))
				for index, user := range *update.Message.NewChatMembers {
					names[index] = formatName(user)
				}
				name := strings.Join(names, ", ")

				markup := tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(buttonURL(T("visit_our_group_website"), osProgramadoresURL)),
					tgbotapi.NewInlineKeyboardRow(button(T("read_the_rules"), "rules")),
				)
				x.sendReplyWithMarkup(update, fmt.Sprintf(T("welcome"), name), markup)

			// User commands.
			case update.Message.IsCommand():
				cmd := strings.ToLower(update.Message.Command())

				bcmd, ok := x.commands[cmd]
				if !ok {
					log.Printf("Ignoring invalid command: %q", cmd)
					break
				}
				// Fail silently if non-private request on private only command.
				if bcmd.pvtOnly && !isPrivateChat(update.Message.Chat) {
					log.Printf("Ignoring non-private request on private only command %q", cmd)
					break
				}
				// Handle command. Emit (and log) error.
				err := bcmd.handler(update)
				if err != nil {
					e := fmt.Sprintf(T("handler_error"), err.Error())
					x.sendReply(update, e)
					fmt.Println(e)
				}
			}
		}
	}
}

// hackerHandler provides anti-hacker protection to the bot.
func (x *opBot) hackerHandler(update tgbotapi.Update) error {
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
	x.bot.DeleteMessage(toDelete)
	x.bot.Send(gif)

	return nil
}

// Register registers a command a its handler on the bot.
func (x *opBot) Register(cmd string, desc string, pvtOnly bool, enabled bool, handler func(tgbotapi.Update) error) {
	if x.commands == nil {
		x.commands = map[string]botCommand{}
	}

	x.commands[cmd] = botCommand{
		desc:    desc,
		pvtOnly: pvtOnly,
		enabled: enabled,
		handler: handler,
	}
	log.Printf("Registered command %q, %q", cmd, desc)
}

// helpHandler sends a help message back to the user.
func (x *opBot) helpHandler(update tgbotapi.Update) error {
	helpMsg := make([]string, len(x.commands))
	ix := 0
	for c, bcmd := range x.commands {
		helpMsg[ix] = fmt.Sprintf("/%s: %s", c, bcmd.desc)
		ix++
	}

	x.sendReply(update, strings.Join(helpMsg, "\n"))
	return nil
}

// indentHandler indents the code in a repl.it URL.
func (x *opBot) indentHandler(update tgbotapi.Update) error {
	args := strings.Trim(update.Message.CommandArguments(), " ")

	repl, err := handleReplItURL(&runner{}, args)
	if err != nil {
		return err
	}

	msg := fmt.Sprintf(T("indent_ok"), repl.newURL, repl.SessionID)
	x.sendReply(update, msg)
	return nil
}

// sendReply sends a reply to a specific MessageID.
func (x *opBot) sendReply(update tgbotapi.Update, text string) (tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, text)
	msg.ReplyToMessageID = update.Message.MessageID
	return x.bot.Send(msg)
}

// sendReplyWithMarkup sends a reply to a specific MessageID with markup.
func (x *opBot) sendReplyWithMarkup(update tgbotapi.Update, text string, markup tgbotapi.InlineKeyboardMarkup) (tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, text)
	msg.ReplyToMessageID = update.Message.MessageID
	msg.ReplyMarkup = &markup
	return x.bot.Send(msg)
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

// button returns a button with the specified message and label.
func button(msg, label string) tgbotapi.InlineKeyboardButton {
	return tgbotapi.NewInlineKeyboardButtonData(msg, label)
}

// buttonURL creates a button with an URL.
func buttonURL(msg, url string) tgbotapi.InlineKeyboardButton {
	return tgbotapi.NewInlineKeyboardButtonURL(msg, url)
}
