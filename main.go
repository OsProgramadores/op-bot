package main

import (
	"fmt"
	"gopkg.in/telegram-bot-api.v4"
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

	runBot(config, bot)
}
