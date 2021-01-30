package main

import (
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/patrickmn/go-cache"
	"io"
	"log"
	"math/rand"
	"strings"
	"time"
)

const (
	// osProgramadoresURL contains the main group URL.
	osProgramadoresURL = "https://osprogramadores.com"

	// osProgramadoresURL contains the main group URL.
	osProgramadoresRulesURL = "https://osprogramadores.com/regras/"

	// osProgramadoresGroup is the group username.
	osProgramadoresGroup = "osprogramadores"
)

// opBot defines an instance of op-bot.
type opBot struct {
	config   botConfig
	commands map[string]botCommand

	// New users must follow certain restrictions.
	newUserCache *cache.Cache

	// List of users not yet validated by captcha.
	pendingCaptcha *pendingCaptchaType

	// Don't send warning messages to new users on every infraction.
	newUserWarningCache *cache.Cache

	// Time to live for welcome messages
	welcomeMessageTTL time.Duration

	// How long to wait for the correct captcha (0 = disable feature).
	captchaTime time.Duration

	notifications notificationsInterface
	media         mediaInterface
	bans          bansInterface
	geolocations  geoLocationsInterface

	// statsWriter holds handler to write stats to disk.
	statsWriter io.WriteCloser
}

// botCommands holds the commands accepted by the bot, their description and a handler function.
type botCommand struct {
	desc      string
	adminOnly bool
	pvtOnly   bool
	enabled   bool
	handler   func(tgbotInterface, tgbotapi.Update) error
}

// newOpBot returns a new OpBot.
func newOpBot(config botConfig) (opBot, error) {
	sw, err := initStats()
	if err != nil {
		return opBot{}, fmt.Errorf("error initializing stats: %v", err)
	}

	// Initialize RNG.
	rand.Seed(time.Now().UnixNano())

	// Convert from parsed duration to time.Duration.
	duration := config.NewUserProbationTime.Duration

	return opBot{
		config:        config,
		notifications: newNotifications(),
		media:         newBotMedia(),
		bans:          newBans(),
		geolocations:  newGeolocations(config.LocationKey),
		statsWriter:   sw,

		newUserCache:   cache.New(duration, duration),
		pendingCaptcha: newPendingCaptchaType(),

		// How often will re-send warning messages to offending new users.
		newUserWarningCache: cache.New(30*time.Minute, time.Hour),

		// By default welcome messages will last for 30 minutes.
		welcomeMessageTTL: 30 * time.Minute,

		// Enable captcha by default.
		captchaTime: 1 * time.Minute,
	}, nil
}

// Close performs cleanup functions on the bot.
func (x *opBot) Close() {
	x.statsWriter.Close()
}

// Run is the main message dispatcher for the bot.
func (x *opBot) Run(bot *tgbotapi.BotAPI) {
	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, _ := bot.GetUpdatesChan(u)

	for update := range updates {
		switch {
		case update.CallbackQuery != nil:
			x.handleCallbackQuery(bot, update)

		case update.Message != nil:
			promMessageCount.Inc()

			// Update stats if the message comes from @osprogramadores.
			updateMessageStats(x.statsWriter, update, osProgramadoresGroup)

			// Notifications.
			x.notifications.manageNotifications(bot, update)

			// Handle messages from users who are yet to validate the captcha.
			// Explicitly ignore NewChatMember requests since users who leave
			// the group and re-join will create one of such messages.

			captcha := userCaptcha(x, bot, update.Message.Chat.ID, update.Message.From.ID)
			if captcha != nil && update.Message.NewChatMembers == nil {
				text := update.Message.Text
				name := formatName(*update.Message.From)
				userid := update.Message.From.ID
				msgid := update.Message.MessageID

				// Remove all messages, validate text later (see below).
				log.Printf("Removing message %d from non-captcha validated user %s (id=%d), want captcha=%04.4d: %q", msgid, name, userid, captcha.code, text)
				deleteMessage(bot, update.Message.Chat.ID, msgid)

				// If the user requested another captcha, reset the code and
				// send another captcha.
				if captchaResendRequest(text) {
					x.sendCaptcha(bot, update, *update.Message.From)
					continue
				}

				// If the text of this message matches the captcha, remove user
				// from pendingCaptcha list and send the welcome message.
				// Matching or not, continue to the next message right after,
				// since the captcha message purpose has already been
				// fulfilled.
				if matchCaptcha(*captcha, text) {
					// Remove from the pendingCaptcha list. The goroutine
					// started to kick this user at join time will find nothing
					// and exit normally.
					promCaptchaValidatedCount.Inc()
					x.pendingCaptcha.del(userid)
					x.sendWelcome(bot, update, *update.Message.From)
				}
				continue
			}

			// Block many types of rich media from new users (but always allows admins).
			admin, err := isAdmin(bot, update.Message.Chat.ID, update.Message.From.ID)
			if err != nil {
				log.Printf("Unable to determine if user is an admin. Assuming not.")
			}
			if x.config.NewUserProbationTime.Hours() > 0 && !admin {
				x.processNewUsers(bot, update)
			}

			switch {
			// Forward message handling.
			case x.config.DeleteFwd && isForwarded(update.Message):
				// Remove forwarded message and log.
				bot.DeleteMessage(tgbotapi.DeleteMessageConfig{
					ChatID:    update.Message.Chat.ID,
					MessageID: update.Message.MessageID,
				})
				log.Printf("Removed forwarded message. ChatID: %v, MessageID: %v", update.Message.Chat.ID, update.Message.MessageID)

			// Location.
			case update.Message.Location != nil:
				x.processLocationRequest(bot, update)

			// New User Join event.
			//
			// Telegram generates these events anytime a new user joins the group.
			// The list of New Members is present on update.Message.NewChatMembers.
			case update.Message.NewChatMembers != nil:
				for _, newUser := range *update.Message.NewChatMembers {
					promJoinCount.Inc()

					log.Printf("Processing new user request for user %q, uid=%d\n", formatName(newUser), newUser.ID)

					// Ban bots. Move on to next user.
					if newUser.IsBot {
						x.banNewBots(bot, update, newUser)
					}

					// At this point we probably have a real user. Send the captcha
					// and add user to the new users list. Welcome message is sent
					// after the user validates.
					log.Printf("Captcha time is %d, captcha enabled = %v", x.captchaTime, captchaEnabled(x))
					if captchaEnabled(x) {
						x.sendCaptcha(bot, update, newUser)
						x.captchaReaper(bot, update, newUser)
					} else {
						x.sendWelcome(bot, update, newUser)
					}
				}

			// User commands.
			case update.Message.IsCommand():
				x.processUserCommands(bot, update)
			}
		}
	}
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

// banNewBots bans the user if it is a bot and not in our bot whitelist.
// Returns true if a bot was banned, false otherwise. Due to the way telegram
// works, this only works for supergroups.
func (x *opBot) banNewBots(bot kickChatMemberer, update tgbotapi.Update, user tgbotapi.User) {
	// Only if configured.
	if !x.config.KickBots {
		return
	}
	// Bots only.
	if !user.IsBot {
		return
	}

	// Note: It's safe to use user.UsernName here as bots should always have a name.

	// Skip whitelisted bots.
	if stringInSlice(user.UserName, x.config.BotWhitelist) {
		log.Printf("Whitelisted bot %q has joined. Doing nothing.", user.UserName)
		return
	}
	// Ban!
	if err := banUser(bot, update.Message.Chat.ID, user.ID); err != nil {
		log.Printf("Error attempting to ban bot named %q: %v", user.UserName, err)
	}
	log.Printf("Banned bot %q. Hasta la vista, baby...", user.UserName)
}

// sendWelcome sends a new message to newly joined users.
func (x *opBot) sendWelcome(bot sendDeleteMessager, update tgbotapi.Update, user tgbotapi.User) {
	// No welcome to bots.
	if user.IsBot {
		return
	}
	// New users get flagged as such. If new user restrictions are
	// enabled, only text messages will be allowed.
	strID := fmt.Sprintf("%d", user.ID)
	if _, found := x.newUserCache.Get(strID); !found {
		log.Printf("User %s marked as a new user.", formatName(user))
		x.newUserCache.Set(strID, time.Now(), cache.DefaultExpiration)
	}

	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(buttonURL(T("visit_our_group_website"), osProgramadoresURL)),
		tgbotapi.NewInlineKeyboardRow(buttonURL(T("read_the_rules"), osProgramadoresRulesURL)),
	)
	welcome, err := sendMessageWithMarkup(bot, update.Message.Chat.ID, update.Message.MessageID, fmt.Sprintf(T("welcome"), nameRef(user)), markup)
	if err != nil {
		log.Printf("Error sending welcome message to user %s", formatName(user))
		return
	}

	// Delete welcome message after the configured timeout.
	selfDestructMessage(bot, welcome.Chat.ID, welcome.MessageID, x.welcomeMessageTTL)
}

// processNewUsers verifies if the user has been on the list for less than a
// pre-determined amount of time. If so, delete any non-text messages from the
// user and send a self-destructing warning message.
func (x *opBot) processNewUsers(bot sendDeleteMessager, update tgbotapi.Update) {
	strID := fmt.Sprintf("%d", update.Message.From.ID)

	// Return immediately if user not in probation list.
	if _, found := x.newUserCache.Get(strID); !found {
		return
	}

	// Blocks non-text messages. Checks messages and edited messages.
	for _, msg := range []*tgbotapi.Message{update.Message, update.EditedMessage} {
		if richMessage(msg) {
			promRichMessageDeletedCount.Inc()

			// Log and delete message.
			log.Printf("New user (%s) attempted to send non-text message. Deleting and notifying.", formatName(*msg.From))

			// We only send a reply message if the user does not appear in the
			// newUserWarningCache (which has a expiration of minutes). The
			// idea is to prevent a repeat offender from causing the bot to
			// flood the group.
			if _, found := x.newUserWarningCache.Get(strID); !found {
				markup := tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(buttonURL(T("read_the_rules"), osProgramadoresRulesURL)),
				)
				reply, err := sendReplyWithMarkup(bot, msg.Chat.ID, msg.MessageID, T("only_text_messages"), markup)
				// We log errors but try to move ahead and still block the offending message.
				if err != nil {
					log.Printf("Error sending rules message: %v", err)
				}
				// Delete warning message.
				selfDestructMessage(bot, reply.Chat.ID, reply.MessageID, 0)
			}

			// Delete original message.
			bot.DeleteMessage(tgbotapi.DeleteMessageConfig{
				ChatID:    msg.Chat.ID,
				MessageID: msg.MessageID,
			})
			x.newUserWarningCache.Set(strID, time.Now(), cache.DefaultExpiration)
		}
	}
}

// processUserCommands processes all user to bot commands (usually starting with a slash) by
// parsing the input and calling the appropriate command handler.
func (x *opBot) processUserCommands(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
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
	// Fail silently if a regular user makes an admin-only request.
	if bcmd.adminOnly {
		admin, err := isAdmin(bot, update.Message.Chat.ID, update.Message.From.ID)
		if err != nil {
			log.Printf("Error retrieving user info for %v: %v", update, err)
			return
		}
		if !admin {
			log.Printf("Regular user %s attempted to use admin-only command: %s (ignored)", formatName(*update.Message.From), cmd)
			return
		}
	}

	// Handle command. Emit (and log) error.
	err := bcmd.handler(bot, update)
	if err != nil {
		e := fmt.Sprintf(T("handler_error"), err.Error())
		sendReply(bot, update.Message.Chat.ID, update.Message.MessageID, e)
		log.Println(e)
	}
}

// selfDestructMessage deletes a message in a chat after the specified amount of time.
// If the ttl is set to zero, assume a default of 30m.
func selfDestructMessage(bot deleteMessager, chatID int64, messageID int, ttl time.Duration) {
	if ttl < 0 {
		return
	}
	if ttl == 0 {
		ttl = time.Duration(30 * time.Minute)
	}

	time.AfterFunc(ttl, func() {
		bot.DeleteMessage(tgbotapi.DeleteMessageConfig{
			ChatID:    chatID,
			MessageID: messageID,
		})
	})
}

// richMessage returns true if the message is a rich message containing video, photos,
// audio, etc. False otherwise.
func richMessage(m *tgbotapi.Message) bool {
	return m != nil && (m.Animation != nil || m.Audio != nil || m.Document != nil || m.Game != nil ||
		m.Photo != nil || m.Video != nil || m.VideoNote != nil || m.Voice != nil)
}

// isAdmin returns true if the user is a member of a given chat ID and has
// administrator privileges.
func isAdmin(bot getChatMemberer, chatID int64, userID int) (bool, error) {
	q := tgbotapi.ChatConfigWithUser{
		ChatID: chatID,
		UserID: userID,
	}

	chatmember, err := bot.GetChatMember(q)
	if err != nil {
		return false, err
	}
	return (chatmember.IsAdministrator() || chatmember.IsCreator()), nil
}

// isBanned returns true if the user was previously banned (kick/banned).
func isBanned(bot getChatMemberer, chatID int64, userID int) (bool, error) {
	q := tgbotapi.ChatConfigWithUser{
		ChatID: chatID,
		UserID: userID,
	}

	chatmember, err := bot.GetChatMember(q)
	if err != nil {
		return false, err
	}
	return chatmember.WasKicked(), nil
}

// stringInSlice returns true if a given string is in a string slice, false otherwise.
func stringInSlice(str string, list []string) bool {
	for _, s := range list {
		if str == s {
			return true
		}
	}
	return false
}
