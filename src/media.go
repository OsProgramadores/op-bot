// Media operations for the bot.

package main

import (
	"errors"
	"gopkg.in/telegram-bot-api.v4"
	"log"
	"path"
	"sync"
)

const (
	mediaDBCache = "media.json"
)

// botMedia contains all media related operations on the bot.
type botMedia struct {
	sync.RWMutex
	URLToMediaID map[string]string `json:"media"`
	cfile        string
}

// newBotMedia creates a new bot media type.
func newBotMedia() (*botMedia, error) {
	ddir, err := dataDir()
	if err != nil {
		return nil, err
	}
	return &botMedia{
		URLToMediaID: map[string]string{},
		cfile:        path.Join(ddir, mediaDBCache),
	}, nil
}

// loadMedia loads media list database from mediaDB file.
func (m *botMedia) loadMedia() error {
	m.Lock()
	defer m.Unlock()
	return readJSONFromDataDir(&m.URLToMediaID, m.cfile)
}

// sendMedia sends the media pointed out by `mediaURL' to the user/group
// indicated by `update'. If said media is not yet saved in the database, we do
// it so that we can reuse it in future requests.
func (m *botMedia) sendMedia(bot tgbotInterface, update tgbotapi.Update, mediaURL string) error {
	m.Lock()
	defer m.Unlock()

	var document tgbotapi.DocumentConfig

	// Let's first see if we have this media available from a previous request.
	mediaID, ok := m.URLToMediaID[mediaURL]

	if ok {
		document = tgbotapi.NewDocumentShare(update.Message.Chat.ID, mediaID)
	} else {
		// Issue #74 is at play here, preventing us to upload via URL:
		// https://github.com/go-telegram-bot-api/telegram-bot-api/issues/74
		// We use the workaround described in https://goo.gl/1F9W1U.
		document = tgbotapi.NewDocumentUpload(update.Message.Chat.ID, nil)
		document.FileID = mediaURL
		document.UseExisting = true
	}

	// Reply to quoted message, if any.
	if update.Message.ReplyToMessage != nil {
		document.ReplyToMessageID = update.Message.ReplyToMessage.MessageID
	}

	message, err := bot.Send(document)
	if err != nil {
		log.Printf("Error sending media (url: %s, existing: %v): %v", mediaURL, ok, err)
		return err
	}

	// Sanity check: Don't allow nil document in return message from Send.
	if message.Document == nil {
		err = errors.New("internal error: bot.Send received a nil Document as a response")
		log.Print(err)
		return err
	}

	// Store the Telegram ID, if we do not yet have the requested media.
	if !ok {
		m.URLToMediaID[mediaURL] = message.Document.FileID
		return safeWriteJSON(m.URLToMediaID, m.cfile)
	}

	return err
}
