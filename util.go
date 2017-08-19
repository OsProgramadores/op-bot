package main

import (
	"encoding/json"
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// safeWriteJSON saves `data' (in json) to file.
func safeWriteJSON(data interface{}, file string) error {
	buf, err := json.Marshal(data)
	if err != nil {
		return err
	}
	datadir, err := dataDir()
	if err != nil {
		return err
	}

	tmpfile, err := ioutil.TempFile(datadir, "safe-write")
	if err != nil {
		log.Printf("safeWriteJSON: error creating temp file to save data: %v", err)
		return err
	}
	defer os.Remove(tmpfile.Name())

	if _, err = tmpfile.Write(buf); err != nil {
		log.Printf("safeWriteJSON: error writing data to temp file: %v", err)
		return err
	}

	if err = tmpfile.Close(); err != nil {
		log.Printf("safeWriteJSON: error closing temp file with data: %v", err)
		return err
	}

	f := filepath.Join(datadir, file)
	return os.Rename(tmpfile.Name(), f)
}

// formatName returns the user full name in the form "Firstname Lastname".
func formatName(user tgbotapi.User) string {
	firstName := user.FirstName
	lastName := user.LastName

	return strings.Trim(fmt.Sprintf("%s %s", firstName, lastName), " ")
}

// button returns a button with the specified message and label.
func button(msg, label string) tgbotapi.InlineKeyboardButton {
	return tgbotapi.NewInlineKeyboardButtonData(msg, label)
}

// buttonURL creates a button with an URL.
func buttonURL(msg, url string) tgbotapi.InlineKeyboardButton {
	return tgbotapi.NewInlineKeyboardButtonURL(msg, url)
}

// sendReply sends a reply to a specific MessageID.
func (x *opBot) sendReply(update tgbotapi.Update, text string) (tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, text)
	msg.ReplyToMessageID = update.Message.MessageID
	return x.bot.Send(msg)
}

// sendReplyWithMarkup sends a reply to a specific MessageID with markup.
func (x *opBot) sendReplyWithMarkup(update tgbotapi.Update, text string, markup tgbotapi.InlineKeyboardMarkup) (tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, text)
	msg.ReplyToMessageID = update.Message.MessageID
	msg.ReplyMarkup = &markup
	return x.bot.Send(msg)
}

// isPrivateChat returns true if a chat is private.
func isPrivateChat(chat *tgbotapi.Chat) bool {
	return chat.Type == "private"
}
