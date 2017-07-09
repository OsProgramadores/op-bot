package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
)

const (
	defaultServerPort = 3000
)

var (
	configFile = ".osprogramadores_bot.json"
	msgsFile   = "config/messages.json"
)

type botConfig struct {
	// BotToken contains the Telegram token for this bot.
	BotToken string

	// LocationKey contains an alphanum key used to scramble
	// the user IDs when storing the location.
	LocationKey string

	// ServerPort contains the TCP server port.
	ServerPort int
}

type botMessages struct {
	Welcome              string
	Rules                string
	ReadTheRules         string
	VisitOurGroupWebsite string
	URL                  string
}

// loadConfig loads the configuration items for the bot from 'configFile' under
// the home directory, and assigns sane defaults to certain configuration items.
func loadConfig() (botConfig, error) {
	config := botConfig{}

	home, err := homeDir()
	if err != nil {
		return config, err
	}
	jsonFile := filepath.Join(home, configFile)

	buf, err := ioutil.ReadFile(jsonFile)
	if err != nil {
		return botConfig{}, err
	}
	if err := json.Unmarshal(buf, &config); err != nil {
		return botConfig{}, err
	}

	// Defaults
	if config.ServerPort == 0 {
		config.ServerPort = defaultServerPort
	}

	return config, nil
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

// homeDir returns the user's home directory or an error if the variable HOME
// is not set, or os.user fails, or the directory cannot be found.
func homeDir() (string, error) {
	// Get home directory from the HOME environment variable first.
	home := os.Getenv("HOME")
	if home == "" {
		usr, err := user.Current()
		if err != nil {
			return "", fmt.Errorf("erro lendo informações sobre o usuário corrente: %q", err)
		}
		home = usr.HomeDir
	}
	_, err := os.Stat(home)
	if os.IsNotExist(err) || !os.ModeType.IsDir() {
		return "", fmt.Errorf("diretório home %q tem que existir e ser um diretório", home)
	}
	// Other errors than file not found.
	if err != nil {
		return "", err
	}
	return home, nil
}
