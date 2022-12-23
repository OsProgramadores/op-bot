package main

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

const (
	defaultServerPort = 3000
	configFile        = "config.toml"

	// Directory usually under $HOME/.config that holds all configurations.
	opBotConfigDir = "op-bot"

	// Default directory for the translations.
	opBotTranslationDir = "./translations"
)

// duration satisfies the encoding.TextUnmarshaler interface to
// provide direct decoding of durations in the config file.
type duration struct {
	time.Duration
}

// UnmarshalText decodes the text representation of duration in the
// configuration into a time.Duration object.
func (d *duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

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

	// Restriction time for new users (can't post pictures, audio, etc)
	// Set to 0 to disable this feature.
	NewUserProbationTime duration `toml:"new_user_probation_time"`
}

// loadConfig loads the configuration items for the bot from 'configFile' under
// the home directory, and assigns sane defaults to certain configuration items.
func loadConfig() (botConfig, error) {
	// Hardwire some defaults and let the config override them.
	config := botConfig{
		NewUserProbationTime: duration{time.Duration(24 * time.Hour)},
		KickBots:             true,
		DeleteFwd:            true,
	}

	cfgdir, err := configDir()
	if err != nil {
		return botConfig{}, err
	}
	f := filepath.Join(cfgdir, configFile)

	buf, err := os.ReadFile(f)
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

// translationFile returns the name of the translation file based on the
// desired language. The default directory is "./translations" under the
// current directory, but can be overriden by the environment variable
// "TRANSLATIONS_DIR".
func translationFile(lang string) string {
	dir := os.Getenv("TRANSLATIONS_DIR")
	if dir == "" {
		dir = opBotTranslationDir
	}

	return filepath.Join(dir, lang+"-all.toml")
}

// loadTranslation loads the translation file from the specified file and
// returns a function to handle the translation of messages.
func loadTranslation(fname string) (func(string) string, error) {
	var config interface{}
	if _, err := toml.DecodeFile(fname, &config); err != nil {
		return nil, fmt.Errorf("unable to load translation file %s: %v", fname, err)
	}

	// Make sure every element can be asserted into a string.
	// Failure here means a configuration file problem.
	items, ok := config.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid configuration file")
	}
	for _, v := range items {
		if _, ok := v.(string); !ok {
			return nil, fmt.Errorf("invalid configuration file")
		}
	}

	// This anonymous function returns the translation string
	// from a key, or the key itself if it does not exist in
	// the translation file.
	return func(msg string) string {
		keyval := config.(map[string]interface{})
		val, ok := keyval[msg]
		if !ok {
			return msg
		}
		return val.(string)
	}, nil
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
