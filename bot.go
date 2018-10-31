package main

import (
	"crypto/rand"
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"io"
	"log"
	"math/big"
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
	statsWriter io.WriteCloser
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
			// Update stats if the message comes from @osprogramadores.
			updateMessageStats(x.modules.statsWriter, update, osProgramadoresGroup)

			// Notifications.
			manageNotifications(x, update)

			switch {
			// Forward message handling.
			case update.Message.ForwardFrom != nil && x.config.DeleteFwd:
				// Remove forwarded message and log.
				x.bot.DeleteMessage(tgbotapi.DeleteMessageConfig{
					ChatID:    update.Message.Chat.ID,
					MessageID: update.Message.MessageID,
				})
				log.Printf("Removed forwarded message. ChatID: %v, MessageID: %v", update.Message.Chat.ID, update.Message.MessageID)

			// Location.
			case update.Message.Location != nil:
				x.processLocationRequest(update)

			// Join event.
			case update.Message.NewChatMembers != nil:
				x.processJoinEvents(update)

			// User commands.
			case update.Message.IsCommand():
				x.processUserCommands(update)
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
	x.bot.DeleteMessage(tgbotapi.DeleteMessageConfig{
		ChatID:    update.Message.Chat.ID,
		MessageID: update.Message.MessageID,
	})

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

// updateMessageStats updates the message statistics for all messages from a
// specific username.  Emits an error message to output in case of errors.
func updateMessageStats(w io.Writer, update tgbotapi.Update, username string) {
	if update.Message.From != nil && update.Message.Chat.UserName == username {
		if saved, err := saveStats(w, &update); err != nil {
			log.Println(T("stats_error_saving"), err.Error(), saved)
		}
	}
}

// processLocationRequest fetches user geo-location information from the
// request and adds the approximate location of the user to a point in the map
// using handleLocation.  Returns a visible message to the user in case of
// problems.
func (x *opBot) processLocationRequest(update tgbotapi.Update) {
	userid := update.Message.From.ID
	location := update.Message.Location

	err := handleLocation(&x.modules.locations,
		x.config.LocationKey,
		fmt.Sprintf("%d", userid),
		location.Latitude,
		location.Longitude)

	// Give feedback to user, if message was sent privately.
	if isPrivateChat(update.Message.Chat) {
		message := T("location_success")
		if err != nil {
			message = T("location_fail")
		}
		x.sendReply(update, message)
	}
}

// processJoinEvent sends a new message to newly joined users.
func (x *opBot) processJoinEvents(update tgbotapi.Update) {
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
}

// processUserCommands processes all user to bot commands (usually starting with a slash) by
// parsing the input and calling the appropriate command handler.
func (x *opBot) processUserCommands(update tgbotapi.Update) {
	cmd := strings.ToLower(update.Message.Command())

	bcmd, ok := x.commands[cmd]
	if !ok {
		log.Printf("Ignoring invalid command: %q", cmd)
		return
	}
	// Fail silently if non-private request on private only command.
	if bcmd.pvtOnly && !isPrivateChat(update.Message.Chat) {
		log.Printf("Ignoring non-private request on private only command %q", cmd)
		return
	}
	// Handle command. Emit (and log) error.
	err := bcmd.handler(update)
	if err != nil {
		e := fmt.Sprintf(T("handler_error"), err.Error())
		x.sendReply(update, e)
		log.Println(e)
	}
}
