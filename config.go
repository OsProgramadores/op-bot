package main

import (
	"errors"
	"fmt"
	"github.com/BurntSushi/toml"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
)

const (
	defaultServerPort = 3000
	configFile        = "config.toml"
	msgsFile          = "messages.toml"

	// Directory usually under $HOME/.config that holds all configurations.
	opBotConfigDir = "op-bot"
)

type botConfig struct {
	// BotToken contains the Telegram token for this bot.
	BotToken string `toml:"token"`

	// LocationKey contains an alphanum key used to scramble
	// the user IDs when storing the location.
	LocationKey string `toml:"location_key"`

	// ServerPort contains the TCP server port.
	ServerPort int `toml:"server_port"`

	// API Key from www.cepaberto.com (brazilian postal code to geo location service.)
	CepAbertoKey string `toml:"cep_aberto_key"`
}

type botMessages struct {
	Welcome              string `toml:"welcome"`
	Rules                string `toml:"rules"`
	ReadTheRules         string `toml:"read_the_rules"`
	VisitOurGroupWebsite string `toml:"visit_our_group_website"`
	URL                  string `toml:"url"`
	LocationSuccess      string `toml:"location_success"`
	LocationFail         string `toml:"location_fail"`
}

// loadConfig loads the configuration items for the bot from 'configFile' under
// the home directory, and assigns sane defaults to certain configuration items.
func loadConfig() (botConfig, error) {
	config := botConfig{}

	cfgdir, err := configDir()
	if err != nil {
		return botConfig{}, err
	}
	f := filepath.Join(cfgdir, configFile)

	buf, err := ioutil.ReadFile(f)
	if err != nil {
		return botConfig{}, err
	}
	if _, err := toml.Decode(string(buf), &config); err != nil {
		return botConfig{}, err
	}

	// Check mandatory fields
	if config.BotToken == "" {
		return botConfig{}, errors.New("token não pode estar em branco")
	}

	// Defaults
	if config.ServerPort == 0 {
		config.ServerPort = defaultServerPort
	}

	return config, nil
}

func loadMessages() (botMessages, error) {
	messages := botMessages{}

	cfgdir, err := configDir()
	if err != nil {
		return botMessages{}, err
	}
	f := filepath.Join(cfgdir, msgsFile)

	buf, err := ioutil.ReadFile(f)
	if err != nil {
		return botMessages{}, err
	}
	if _, err := toml.Decode(string(buf), &messages); err != nil {
		return botMessages{}, err
	}

	return messages, nil
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

// configDir returns the location for config files. Use the XDG_CONFIG_HOME
// environment variable, or the fallback value of $HOME/.config if the variable
// is not set.
func configDir() (string, error) {
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg != "" {
		return filepath.Join(xdg, opBotConfigDir), nil
	}
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", opBotConfigDir), nil
}

// dataDir returns the location for data files. Use the XDG_DATA_HOME
// environment variable, or the fallback value of $HOME/.local/share if the variable
// is not set.
func dataDir() (string, error) {
	xdg := os.Getenv("XDG_DATA_HOME")
	if xdg != "" {
		return filepath.Join(xdg, opBotConfigDir), nil
	}
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", opBotConfigDir), nil
}
