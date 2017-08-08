package main

import (
	"errors"
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"os"
	"path/filepath"
)

const (
	statsDB = "stats.csv"
)

// saveStats saves information on the received message to statsDB file as CSV.
func saveStats(u *tgbotapi.Update) (string, error) {
	if u.Message == nil {
		return "", errors.New(T("stats_error_empty_message"))
	}

	if u.Message.From == nil {
		return "", errors.New(T("stats_error_unknown_user"))
	}

	// Chat id, UNIX timestamp, user id, msg len.
	line := fmt.Sprintf("%d,%d,%d,%d\n", u.Message.MessageID, u.Message.Date, u.Message.From.ID, len(u.Message.Text)+len(u.Message.Caption))

	datadir, err := dataDir()
	if err != nil {
		return "", err
	}

	err = os.MkdirAll(datadir, 0755)
	if err != nil {
		return "", err
	}

	filename := filepath.Join(datadir, statsDB)
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err = f.WriteString(line); err != nil {
		line = ""
	}

	return line, err
}
