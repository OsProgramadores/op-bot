package main

import (
	"gopkg.in/telegram-bot-api.v4"
	"log"
)

var (
	// T holds our global translation function. We return blank
	// by default to make test initialization simpler.
	// TODO: investigate why we can't use i18n.Translatefunc as the type here.
	T = func(translationID string, args ...interface{}) string {
		return ""
	}
)

func main() {
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Unable to load config: %s", err)
	}

	T, err = loadTranslation(config.Language)
	if err != nil {
		log.Fatalf("Unable to load translations: %s", err)
	}

	bot, err := tgbotapi.NewBotAPI(config.BotToken)
	if err != nil {
		log.Fatalf("%s: %s", T("error_starting_bot"), err)
	}

	// Create new bot
	opbot := opBot{
		config:        config,
		notifications: newNotifications(),
		media:         newBotMedia(),
		bans:          newBans(),
		geolocations:  newGeolocations(config.LocationKey),
	}

	opbot.statsWriter, err = initStats()
	if err != nil {
		log.Printf("Error initializing stats: %v", err)
	} else {
		defer opbot.statsWriter.Close()
	}

	if err = opbot.notifications.loadNotificationSettings(); err != nil {
		log.Printf("Error loading notifications: %v", err)
	}

	if err = opbot.media.loadMedia(); err != nil {
		log.Printf("Error loading media list: %v", err)
	}

	if err = opbot.bans.loadBanRequestsInfo(); err != nil {
		log.Printf("Error loading info on the requested bans: %v", err)
	}

	if err := opbot.geolocations.readLocations(); err != nil {
		log.Printf("Error reading locations: %v", err)
	}

	// Start the HTTP server listing the location info.
	go opbot.geolocations.serveLocations(opbot.config.ServerPort)

	// Register commands.
	// Parameters: command, description, admin only, private only, enabled, handler.
	opbot.Register("hackerdetected", T("register_hackerdetected"), false, false, true, opbot.hackerHandler)
	opbot.Register("help", T("register_help"), false, true, true, opbot.helpHandler)
	opbot.Register("notifications", T("notifications_help"), false, true, true, opbot.notifications.notificationHandler)
	opbot.Register("ban", T("ban_help"), false, false, true, opbot.bans.banRequestHandler)

	// Make it so!
	opbot.Run(bot)
}
