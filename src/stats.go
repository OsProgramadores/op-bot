package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	tgbotapi "github.com/osprogramadores/telegram-bot-api"
)

const (
	statsDB = "stats.csv"
)

// initStats opens the stats file for logging the information.
func initStats() (io.WriteCloser, error) {
	datadir, err := dataDir()
	if err != nil {
		return nil, err
	}

	f, err := os.OpenFile(filepath.Join(datadir, statsDB), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		return nil, err
	}

	return f, nil
}

// saveStats saves information on the received message to statsDB file as CSV.
func saveStats(w io.Writer, u *tgbotapi.Update) (string, error) {
	if w == nil {
		return "", errors.New(T("stats_error_nil_writer"))
	}

	if u.Message == nil {
		return "", errors.New(T("stats_error_empty_message"))
	}

	if u.Message.From == nil {
		return "", errors.New(T("stats_error_unknown_user"))
	}

	// Chat id, UNIX timestamp, user id, msg len.
	line := fmt.Sprintf("%d,%d,%d,%d\n", u.Message.MessageID, u.Message.Date, u.Message.From.ID, len(u.Message.Text)+len(u.Message.Caption))

	_, err := fmt.Fprint(w, line)
	return line, err
}
