package main

import (
	"fmt"
	"log"
	"net/http"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

var (
	// BuildVersion holds the git HEAD commit # at build time
	// (or nil if the binary was not built using make).
	BuildVersion string

	// T holds our global translation function. We return blank
	// by default to make test initialization simpler.
	T = func(string) string {
		return ""
	}
)

func main() {
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Unable to load config: %s", err)
	}
	log.Printf("Loaded config: %+v", config)

	T, err = loadTranslation(translationFile(config.Language))
	if err != nil {
		log.Fatalf("Unable to load translations: %s", err)
	}

	bot, err := tgbotapi.NewBotAPI(config.BotToken)
	if err != nil {
		log.Fatalf("%s: %s", T("error_starting_bot"), err)
	}

	// Create new bot.
	opbot, err := newOpBot(config)
	if err != nil {
		log.Fatalf("%s: %s", T("error_starting_bot"), err)
	}
	defer opbot.Close()

	if err = opbot.notifications.loadNotificationSettings(); err != nil {
		log.Printf("Error loading notifications: %v (assuming no notifications)", err)
	}

	if err = opbot.media.loadMedia(); err != nil {
		log.Printf("Error loading media list: %v (assuming empty media cache)", err)
	}

	if err = opbot.bans.loadBanRequestsInfo(); err != nil {
		log.Printf("Error loading info on the requested bans: %v (assuming no bans)", err)
	}

	if err := opbot.geolocations.readLocations(); err != nil {
		log.Printf("Error reading locations: %v (assuming no locations recorded)", err)
	}

	// Print version
	log.Printf("Starting op-bo, Git Build: %s\n", BuildVersion)

	// Start the HTTP server listing the location info.
	opbot.geolocations.serveLocations()

	// Register commands.
	// Parameters: command, description, admin only, private only, enabled, handler.
	opbot.Register("hackerdetected", T("register_hackerdetected"), false, false, true, opbot.hackerHandler)
	opbot.Register("help", T("register_help"), false, true, true, opbot.helpHandler)
	opbot.Register("notifications", T("notifications_help"), false, true, true, opbot.notifications.notificationHandler)
	opbot.Register("ban", T("ban_help"), false, false, true, opbot.bans.banRequestHandler)
	opbot.Register("new_user_probation_time", T("new_user_probation_time_help"), true, false, true, opbot.setNewUserProbationTimeHandler)
	opbot.Register("welcome_message_ttl", T("welcome_message_ttl_help"), true, false, true, opbot.setWelcomeMessageTTLHandler)
	opbot.Register("captcha_time", T("captcha_time_help"), true, false, true, opbot.setCaptchaTimeHandler)

	// Start listener
	go http.ListenAndServe(fmt.Sprintf(":%d", opbot.config.ServerPort), nil)

	// Make it so!
	opbot.Run(bot)
}
