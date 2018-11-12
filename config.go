package main

import (
	"errors"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/i18n"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
)

const (
	defaultServerPort = 3000
	configFile        = "config.toml"

	// Directory usually under $HOME/.config that holds all configurations.
	opBotConfigDir = "op-bot"

	// Directory under config where translations are stored.
	opBotTranslationDir = "translations"
)

type botConfig struct {
	// BotToken contains the Telegram token for this bot.
	BotToken string `toml:"token"`

	// LocationKey contains an alphanum key used to scramble
	// the user IDs when storing the location.
	LocationKey string `toml:"location_key"`

	// Language defines the language for all bot messages
	Language string `toml:"language"`

	// ServerPort contains the TCP server port.
	ServerPort int `toml:"server_port"`

	// Automatically delete all forwarded messages?
	DeleteFwd bool `toml:"delete_fwd"`

	// Kick other bots from the channel at join time.
	KickBots bool `toml:"kick_bots"`

	// Bots in this whitelist won't be automatically kicked.
	BotWhitelist []string `toml:"bot_whitelist"`
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
		return botConfig{}, errors.New("token cannot be null")
	}

	// Defaults
	if config.ServerPort == 0 {
		config.ServerPort = defaultServerPort
	}
	if config.Language == "" {
		config.Language = "en-us"
	}

	return config, nil
}

// loadTranslation loads the translation file for the specified language
// and returns a Tfunc function to handle the translation.
func loadTranslation(lang string) (i18n.TranslateFunc, error) {
	// Empty translate func
	tfunc := func(translationID string, args ...interface{}) string {
		return ""
	}

	cfgdir, err := configDir()
	if err != nil {
		return tfunc, err
	}

	f := filepath.Join(cfgdir, opBotTranslationDir, lang+"-all.toml")
	if err := i18n.LoadTranslationFile(f); err != nil {
		return tfunc, err
	}

	return i18n.Tfunc(lang)
}

// homeDir returns the user's home directory or an error if the variable HOME
// is not set, or os.user fails, or the directory cannot be found.
func homeDir() (string, error) {
	// Get home directory from the HOME environment variable first.
	home := os.Getenv("HOME")
	if home == "" {
		usr, err := user.Current()
		if err != nil {
			return "", fmt.Errorf(T("error_reading_user_info"), err)
		}
		home = usr.HomeDir
	}
	_, err := os.Stat(home)
	if os.IsNotExist(err) || !os.ModeType.IsDir() {
		return "", fmt.Errorf(T("error_homedir_must_exist"), home)
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
// is not set. It also attempts to create dataDir, in case it does not exist.
func dataDir() (string, error) {
	xdg := os.Getenv("XDG_DATA_HOME")

	if xdg != "" {
		dir := filepath.Join(xdg, opBotConfigDir)
		return dir, os.MkdirAll(dir, 0755)
	}

	home, err := homeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".local", "share", opBotConfigDir)
	return dir, os.MkdirAll(dir, 0755)
}
