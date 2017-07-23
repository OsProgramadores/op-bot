package main

import (
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
	"os"
)

func main() {
	config, err := loadConfig()
	if err != nil {
		fmt.Println("Houston, we have a problem: ", err)
		fmt.Println("You can see an example of bot token config file at 'config/token.json.sample'")
		os.Exit(1)
	}

	bot, err := tgbotapi.NewBotAPI(config.BotToken)
	if err != nil {
		log.Panic(err)
	}

	messages, err := loadMessages()
	if err != nil {
		fmt.Println("Unable to load messages file: ", err)
		fmt.Println("You can see an example of bot messages file at 'config/messages.json.sample'")
		os.Exit(1)
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
