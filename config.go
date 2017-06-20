package main

import (
	"encoding/json"
	"os"
)

var (
	configFile = "config/token.json"
	msgsFile   = "config/messages.json"
)

type botConfig struct {
	BotToken string
}

type botMessages struct {
	Welcome              string
	Rules                string
	ReadTheRules         string
	VisitOurGroupWebsite string
	URL                  string
}

func loadConfig() (config botConfig, err error) {
	var jsonConfig *os.File

	jsonConfig, err = os.Open(configFile)
	if err != nil {
		return
	}
	defer jsonConfig.Close()

	decoder := json.NewDecoder(jsonConfig)
	err = decoder.Decode(&config)
	return
}

func loadMessages() (messages botMessages, err error) {
	var jsonConfig *os.File

	jsonConfig, err = os.Open(msgsFile)
	if err != nil {
		return
	}
	defer jsonConfig.Close()

	decoder := json.NewDecoder(jsonConfig)
	err = decoder.Decode(&messages)
	return
}
