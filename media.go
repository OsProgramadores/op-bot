package main

import (
	"gopkg.in/telegram-bot-api.v4"
	"log"
	"sync"
)

const (
	mediaDB = "media.json"
)

// mediaList maps `media urls' -> `Telegram media IDs'.
type mediaList struct {
	sync.RWMutex
	Media map[string]string `json:"media"`
}

// loadMedia loads media list database from mediaDB file.
func loadMedia(m *mediaList) error {
	m.Lock()
	defer m.Unlock()

	return readJSONFromDataDir(&m.Media, mediaDB)
}

// sendMedia sends the media pointed out by `mediaURL' to the user/group
// indicated by `update'. If said media is not yet saved in the database, we do
// it so that we can reuse it in future requests.
func (x *opBot) sendMedia(bot *tgbotapi.BotAPI, update tgbotapi.Update, mediaURL string) error {
	x.modules.media.Lock()
	defer x.modules.media.Unlock()

	var document tgbotapi.DocumentConfig

	// Let's first see if we have this media available from a previous request.
	mediaID, ok := x.modules.media.Media[mediaURL]

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

	// Let's store the Telegram ID, if we do not yet have the requested media.
	if !ok {
		x.modules.media.Media[mediaURL] = message.Document.FileID
		return safeWriteJSON(x.modules.media.Media, mediaDB)
	}

	return err
}
