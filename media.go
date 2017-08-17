package main

import (
	"encoding/json"
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
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

// saveMedia saves media list to mediaDB file.
func saveMedia(media map[string]string) error {
	buf, err := json.Marshal(media)
	if err != nil {
		return err
	}
	datadir, err := dataDir()
	if err != nil {
		return err
	}

	tmpfile, err := ioutil.TempFile(datadir, "temp-media")
	if err != nil {
		log.Printf("Error creating temp file to save media list: %v", err)
		return err
	}
	defer os.Remove(tmpfile.Name())

	if _, err = tmpfile.Write(buf); err != nil {
		log.Printf("Error writing media list to temp file: %v", err)
		return err
	}

	if err = tmpfile.Close(); err != nil {
		log.Printf("Error closing temp file with media list: %v", err)
		return err
	}

	f := filepath.Join(datadir, mediaDB)
	return os.Rename(tmpfile.Name(), f)
}

// loadMedia loads media list database from mediaDB file.
func loadMedia(m *mediaList) error {
	m.Lock()
	defer m.Unlock()

	datadir, err := dataDir()
	if err != nil {
		return err
	}
	f := filepath.Join(datadir, mediaDB)

	buf, err := ioutil.ReadFile(f)
	if err != nil {
		return err
	}

	return json.Unmarshal(buf, &m.Media)
}

// sendMedia sends the media pointed out by `mediaURL' to the user/group
// indicated by `update'. If said media is not yet saved in the database, we do
// it so that we can reuse it in future requests.
func sendMedia(x *opBot, update tgbotapi.Update, mediaURL string) error {
	x.media.Lock()
	defer x.media.Unlock()

	var document tgbotapi.DocumentConfig

	// Let's first see if we have this media available from a previous request.
	mediaID, ok := x.media.Media[mediaURL]

	if ok {
		fmt.Println("SUCESS, existing")
		document = tgbotapi.NewDocumentShare(update.Message.Chat.ID, mediaID)
	} else {
		fmt.Println("OOPS, DOES NOT exist")

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

	message, err := x.bot.Send(document)
	if err != nil {
		log.Printf("Error sending media (url: %s, existing: %v): %v", mediaURL, ok, err)
		return err
	}

	// Let's store the Telegram ID, if we do not yet have the requested media.
	if !ok {
		x.media.Media[mediaURL] = message.Document.FileID
		return saveMedia(x.media.Media)
	}

	return err
}
