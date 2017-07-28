package main

import (
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
)

const (
	configTipMsg = `You can find a sample config file inside the "examples" directory in the source tree.`
)

func main() {
	log.SetFlags(0)

	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Unable to load config file: %s\n%s", err, configTipMsg)
	}

	bot, err := tgbotapi.NewBotAPI(config.BotToken)
	if err != nil {
		log.Fatalf("Unable to start the bot (missing token in config?): %s", err)
	}

	messages, err := loadMessages()
	if err != nil {
		log.Fatalf("Unable to load messages file: %s\n%s", err, configTipMsg)
	}

	go serveLocations(config)

	// Create new bot
	b := opBot{
		config:   config,
		messages: messages,
		bot:      bot,
	}
	// Register commands
	b.Register("indent", "Indenta um programa no repl.it (/indent url)", false, true, b.indentHandler)
	b.Register("hackerdetected", "Dispara o alarme anti-hacker. :)", false, true, b.hackerHandler)
	b.Register("setlocation", "Atualiza posição geográfica usando código postal (/setlocation <pais> <código postal>)", true, true, b.locationHandler)
	b.Register("cep", "Atualiza posição geográfica usando CEP", true, true, b.locationHandler)
	b.Register("help", "Mensagem de help", true, true, b.helpHandler)

	// Make it so!
	b.Run()
}
