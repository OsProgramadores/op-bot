package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	tgbotapi "github.com/osprogramadores/telegram-bot-api"
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

	tmpfile, err := os.CreateTemp(datadir, "safe-write")
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

// readFromJSON loads the JSON content from `file' into `data'. Note that locks
// -- if needed -- are assumed to be taken care of outside this function.
func readFromJSON(data interface{}, file string) error {
	buf, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	return json.Unmarshal(buf, data)
}

// readJSONFromDataDir loads the JSON content from `file', which is located
// within the data dir. As with readFromJSON, locks are assumed to be handled
// externally.
func readJSONFromDataDir(data interface{}, file string) error {
	datadir, err := dataDir()
	if err != nil {
		return err
	}
	f := filepath.Join(datadir, file)
	return readFromJSON(data, f)
}

// formatName returns the markdown safe version of the user full name in the
// form "Firstname Lastname".
func formatName(user tgbotapi.User) string {
	firstName := markdownEscape(user.FirstName)
	lastName := markdownEscape(user.LastName)

	return strings.Trim(fmt.Sprintf("%s %s", firstName, lastName), " ")
}

// markdownEscape returns a version of the string with any markdown characters
// ("*", "_", "[", "]", and "`") escaped.
func markdownEscape(s string) string {
	s = strings.ReplaceAll(s, "*", "\\*")
	s = strings.ReplaceAll(s, "_", "\\_")
	s = strings.ReplaceAll(s, "[", "\\[")
	s = strings.ReplaceAll(s, "]", "\\]")
	s = strings.ReplaceAll(s, "`", "\\`")
	return s
}

// nameRef returns a name that contains a @Username reference when username is
// present and contains no Markdown characters, or just a printable "First
// Last" user names.
func nameRef(user tgbotapi.User) string {
	username := user.UserName
	if username == "" || username != markdownEscape(username) {
		return formatName(user)
	}

	return "@" + username
}

// button returns a button with the specified message and label.
func button(msg, label string) tgbotapi.InlineKeyboardButton {
	return tgbotapi.NewInlineKeyboardButtonData(msg, label)
}

// buttonURL creates a button with an URL.
func buttonURL(msg, url string) tgbotapi.InlineKeyboardButton {
	return tgbotapi.NewInlineKeyboardButtonURL(msg, url)
}

// sendPhotoReply sends a file/fileReader/fileBytes reply to a chatid/msgid.
func sendPhotoReply(bot sender, chatID int64, messageID int, file interface{}, caption string) (tgbotapi.Message, error) {
	photoConfig := tgbotapi.NewPhotoUpload(chatID, file)
	photoConfig.Caption = caption
	photoConfig.ParseMode = parseModeMarkdown
	if messageID != 0 {
		photoConfig.ReplyToMessageID = messageID
	}
	return bot.Send(photoConfig)
}

// sendReply sends a reply to a specific MessageID.
func sendReply(bot sender, chatid int64, messageid int, text string) (tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(chatid, text)
	msg.ReplyToMessageID = messageid
	msg.ParseMode = parseModeMarkdown
	return bot.Send(msg)
}

// sendMessage sends a message to a specific ChatID (channel, group, etc).
func sendMessage(bot sender, chatid int64, text string) (tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(chatid, text)
	msg.ParseMode = parseModeMarkdown
	return bot.Send(msg)
}

// deleteMessage deletes a given message id in a chat id. It returns nothing as there's
// not much to do other than log if the message deletion fails.
func deleteMessage(bot deleteMessager, chatid int64, msgid int) error {
	_, err := bot.DeleteMessage(tgbotapi.DeleteMessageConfig{ChatID: chatid, MessageID: msgid})
	if err != nil {
		log.Printf("Unable to delete message (chat_id=%d, message_id=%d). Something is wrong.", chatid, msgid)
	}
	log.Printf("Removed message_id %d from chat_id %d", msgid, chatid)
	return err
}

// sendReplyWithMarkup sends a reply to a specific MessageID with markup.
func sendReplyWithMarkup(bot sender, chatID int64, msgID int, text string, markup tgbotapi.InlineKeyboardMarkup) (tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyToMessageID = msgID
	msg.ReplyMarkup = &markup
	return bot.Send(msg)
}

// sendMessageWithMarkup sends a message with markup.
func sendMessageWithMarkup(bot sender, chatID int64, text string, markup tgbotapi.InlineKeyboardMarkup) (tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = &markup
	return bot.Send(msg)
}

// isPrivateChat returns true if a chat is private.
func isPrivateChat(chat *tgbotapi.Chat) bool {
	return chat.Type == "private"
}

// isForwarded returns true if the received message is forwarded; false
// otherwise.
func isForwarded(msg *tgbotapi.Message) bool {
	if msg == nil {
		return false
	}
	return msg.ForwardFrom != nil || msg.ForwardFromChat != nil || msg.ForwardDate > 0
}

// trDelete returns a copy of the string with all runes in substring removed.
func trDelete(s, substr string) string {
	ret := bytes.Buffer{}

	for _, r := range s {
		if strings.ContainsRune(substr, r) {
			continue
		}
		ret.WriteRune(r)
	}
	return ret.String()
}

// largestPhotoFromMessage returns the file ID of the photo with the largest
// dimension contained in the message.
func largestPhotoFromMessage(message *tgbotapi.Message) (string, error) {
	if message == nil {
		return "", fmt.Errorf("cannot process empty message")
	}

	if message.Photo == nil {
		return "", fmt.Errorf("message does not contain a photo")
	}

	// We get an array with different dimensions of the received photo, and
	// want to send the largest one. Let's first find out which one it is.
	largestPhotoDimension := 0
	indexLargestPhoto := 0
	dimension := 0
	for index, photoSize := range *message.Photo {
		dimension = photoSize.Width * photoSize.Height
		if dimension > largestPhotoDimension {
			largestPhotoDimension = dimension
			indexLargestPhoto = index
		}
	}
	return (*message.Photo)[indexLargestPhoto].FileID, nil
}

// nolint: gocyclo
// createMediaMessage returns a "Chattable" (which can be sent with bot.Send)
// targeted at `destination', made with the media received in the message passed
// as parameter, if there is any media in this message.
func createMediaMessage(message *tgbotapi.Message, destination int64, markup *tgbotapi.InlineKeyboardMarkup) (tgbotapi.Chattable, bool, error) {
	if message == nil {
		return nil, false, fmt.Errorf("cannot process empty message")
	}

	var chattable tgbotapi.Chattable
	switch {
	case message.Sticker != nil:
		sticker := tgbotapi.NewStickerShare(destination, message.Sticker.FileID)
		if markup != nil {
			sticker.ReplyMarkup = markup
		}
		chattable = sticker
	case message.Audio != nil:
		audio := tgbotapi.NewAudioShare(destination, message.Audio.FileID)
		if markup != nil {
			audio.ReplyMarkup = markup
		}
		chattable = audio
	case message.Document != nil:
		document := tgbotapi.NewDocumentShare(destination, message.Document.FileID)
		// Document, video and photo may have caption, up to 200 characters.
		document.Caption = message.Caption
		if markup != nil {
			document.ReplyMarkup = markup
		}
		chattable = document
	case message.Photo != nil:
		// The message contains the photo in different dimensions. We want to
		// use the largest one.
		photoID, err := largestPhotoFromMessage(message)
		if err != nil {
			return nil, false, err
		}
		photo := tgbotapi.NewPhotoShare(destination, photoID)
		// Document, video and photo may have caption, up to 200 characters.
		photo.Caption = message.Caption
		if markup != nil {
			photo.ReplyMarkup = markup
		}
		chattable = photo
	case message.Video != nil:
		video := tgbotapi.NewVideoShare(destination, message.Video.FileID)
		// Document, video and photo may have caption, up to 200 characters.
		video.Caption = message.Caption
		if markup != nil {
			video.ReplyMarkup = markup
		}
		chattable = video
	case message.Venue != nil:
		venue := tgbotapi.NewVenue(destination, message.Venue.Title, message.Venue.Address, message.Venue.Location.Latitude, message.Venue.Location.Longitude)
		if markup != nil {
			venue.ReplyMarkup = markup
		}
		chattable = venue
	case message.Location != nil:
		location := tgbotapi.NewLocation(destination, message.Location.Latitude, message.Location.Longitude)
		if markup != nil {
			location.ReplyMarkup = markup
		}
		chattable = location
	case message.Contact != nil:
		contact := tgbotapi.NewContact(destination, message.Contact.PhoneNumber, message.Contact.FirstName)
		if markup != nil {
			contact.ReplyMarkup = markup
		}
		chattable = contact
	default:
		// Simple text message or unhandled media type.
		return nil, false, nil
	}
	return chattable, true, nil
}
