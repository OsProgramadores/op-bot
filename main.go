package main

import (
	"github.com/go-telegram-bot-api/telegram-bot-api"
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
	log.SetFlags(0)

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

	go serveLocations(config)

	// Create new bot
	b := opBot{
		config: config,
		bot:    bot,
	}
	// Register commands
	b.Register("indent", T("register_indent"), false, true, b.indentHandler)
	b.Register("hackerdetected", T("register_hackerdetected"), false, true, b.hackerHandler)
	b.Register("setlocation", T("register_setlocation"), true, true, b.locationHandler)
	b.Register("cep", T("register_cep"), true, true, b.locationHandler)
	b.Register("help", T("register_help"), true, true, b.helpHandler)

	// Make it so!
	b.Run()
}
