package main

import (
	"crypto/rand"
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
	"math/big"
	"os"
	"strings"
)

const (
	// osProgramadoresURL contains the main group URL.
	osProgramadoresURL = "https://osprogramadores.com"

	// osProgramadoresGroup is the group username.
	osProgramadoresGroup = "osprogramadores"
)

// opBotModules defines the data used by the modules the bot implements.
type opBotModules struct {
	// userNotifications stores the notification settings.
	userNotifications notifications
	// statsWriter is responsible for writing the stats info to disk.
	statsWriter *os.File
	// media has a list of media files used by the bot.
	media mediaList
	// reportedBans lists the bans requested via /ban.
	reportedBans requestedBans
	// locations lists the geolocation info from users.
	locations geoLocationList
}

// opBot defines an instance of op-bot.
type opBot struct {
	config   botConfig
	commands map[string]botCommand
	modules  opBotModules
	bot      *tgbotapi.BotAPI
}

// botCommands holds the commands accepted by the bot, their description and a handler function.
type botCommand struct {
	desc      string
	adminOnly bool
	pvtOnly   bool
	enabled   bool
	handler   func(tgbotapi.Update) error
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
			handleCallbackQuery(x, update)

		case update.Message != nil:
			// Log stats if we the message comes from @osprogramadores.
			if update.Message.From != nil && update.Message.Chat.UserName == osProgramadoresGroup {
				if saved, err := saveStats(x.modules.statsWriter, &update); err != nil {
					log.Println(T("stats_error_saving"), err.Error(), saved)
				}
			}

			// Notifications.
			manageNotifications(x, update)

			switch {
			//Location.
			case update.Message.Location != nil:
				user := update.Message.From
				location := update.Message.Location
				err := handleLocation(&x.modules.locations, x.config.LocationKey, fmt.Sprintf("%d", user.ID), location.Latitude, location.Longitude)

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
	toDelete := tgbotapi.DeleteMessageConfig{ChatID: update.Message.Chat.ID, MessageID: update.Message.MessageID}
	x.bot.DeleteMessage(toDelete)

	// Selects randomly one of the available media and send it.
	// Here we are generating an integer in [0, len(media)).
	randomIndex, err := rand.Int(rand.Reader, big.NewInt(int64(len(media))))
	if err != nil {
		log.Printf("Error generating random index for hackerHandler media: %v", err)
		return nil
	}

	sendMedia(x, update, media[randomIndex.Int64()])
	return nil
}

// Register registers a command a its handler on the bot.
func (x *opBot) Register(cmd string, desc string, adminOnly bool, pvtOnly bool, enabled bool, handler func(tgbotapi.Update) error) {
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

// helpHandler sends a help message back to the user.
func (x *opBot) helpHandler(update tgbotapi.Update) error {
	helpMsg := make([]string, len(x.commands))
	ix := 0
	for c, bcmd := range x.commands {
		if !bcmd.adminOnly {
			helpMsg[ix] = fmt.Sprintf("/%s: %s", c, bcmd.desc)
			ix++
		}
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
