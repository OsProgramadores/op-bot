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

	// TODO: Create "Set" functions inside opbot so we can set botCommands
	// externally without mucking with the internals of the struct.
	b.commands = []botCommand{
		botCommand{"indent", "Indenta um programa no repl.it (/indent url)", false, b.indentHandler},
		botCommand{"hackerdetected", "Dispara o alarme anti-hacker. :)", false, b.hackerHandler},
		botCommand{"setlocation", "Atualiza posição geográfica usando código postal (/setlocation <pais> <código postal>)", true, b.locationHandler},
		botCommand{"cep", "Atualiza posição geográfica usando CEP", true, b.locationHandler},
		botCommand{"help", "Mensagem de help", true, b.helpHandler},
	}

	// Make it so!
	b.Run()
}
