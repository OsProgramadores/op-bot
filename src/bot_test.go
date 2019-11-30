// Unit tests for the bot module.
package main

import (
	"errors"
	"github.com/stretchr/testify/mock"
	"gopkg.in/telegram-bot-api.v4"
	"log"
	"os"
	"testing"
)

// MockTelebot defines an interface to the telegram libraries.
type MockTelebot struct {
	mock.Mock
}

func (m *MockTelebot) AnswerCallbackQuery(config tgbotapi.CallbackConfig) (tgbotapi.APIResponse, error) {
	args := m.Called(config)
	return args.Get(0).(tgbotapi.APIResponse), args.Error(1)
}

func (m *MockTelebot) DeleteMessage(config tgbotapi.DeleteMessageConfig) (tgbotapi.APIResponse, error) {
	args := m.Called(config)
	return args.Get(0).(tgbotapi.APIResponse), args.Error(1)
}

func (m *MockTelebot) GetChatAdministrators(config tgbotapi.ChatConfig) ([]tgbotapi.ChatMember, error) {
	args := m.Called(config)
	return args.Get(0).([]tgbotapi.ChatMember), args.Error(1)
}

func (m *MockTelebot) GetUpdatesChan(config tgbotapi.UpdateConfig) (tgbotapi.UpdatesChannel, error) {
	args := m.Called(config)
	return args.Get(0).(tgbotapi.UpdatesChannel), args.Error(1)
}

func (m *MockTelebot) KickChatMember(config tgbotapi.KickChatMemberConfig) (tgbotapi.APIResponse, error) {
	args := m.Called(config)
	return args.Get(0).(tgbotapi.APIResponse), args.Error(1)
}

func (m *MockTelebot) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	args := m.Called(c)
	return args.Get(0).(tgbotapi.Message), args.Error(1)
}

// MockBotMedia defines an interface to the media module.
type MockBotMedia struct {
	mock.Mock
}

func (m *MockBotMedia) loadMedia() error {
	args := &mock.Arguments{}
	return args.Error(0)
}

func (m *MockBotMedia) sendMedia(bot tgbotInterface, update tgbotapi.Update, mediaURL string) error {
	args := m.Called(bot, update, mediaURL)
	return args.Error(0)
}

// MockGeoLocations defines an interface to the geo locations module.
type MockGeoLocations struct {
	mock.Mock
	locationKey string
	locationDB  string
}

func (m *MockGeoLocations) readLocations() error {
	args := &mock.Arguments{}
	return args.Error(0)
}

func (m *MockGeoLocations) processLocation(id int, lat, lon float64) error {
	args := m.Called(id, lat, lon)
	return args.Error(0)
}

func (m *MockGeoLocations) serveLocations(port int) {
}

// Constants for the tests.

const (
	chatID      = 1234
	msgID       = 2222
	userID      = 3333
	locationKey = "test-key"
)

//
// Tests
//

func TestHelpHandler(t *testing.T) {
	type registry struct {
		cmd       string
		desc      string
		adminOnly bool
		pvtOnly   bool
		enabled   bool
	}

	caseTests := []struct {
		reg     []registry
		wantStr string
	}{
		// Basic test. Valid URI and return. OK.
		{
			reg: []registry{
				{
					cmd:     "foo",
					desc:    "mock_foo_command",
					pvtOnly: true,
					enabled: true,
				},
				{
					cmd:     "bar",
					desc:    "mock_bar_command",
					pvtOnly: true,
					enabled: true,
				},
			},
			wantStr: "/bar: mock_bar_command\n/foo: mock_foo_command",
		},
		// One regular command, one admin, one disable (should print one.)
		{
			reg: []registry{
				{
					cmd:     "foo",
					desc:    "mock_foo_command",
					pvtOnly: true,
					enabled: true,
				},
				{
					cmd:       "bar",
					desc:      "mock_bar_admin_command",
					adminOnly: true,
					pvtOnly:   true,
					enabled:   true,
				},
				{
					cmd:     "bar",
					desc:    "mock_bar_disabled_command",
					pvtOnly: true,
				},
			},
			wantStr: "/foo: mock_foo_command",
		},
	}

	// Built a new opBot with all commands to be registered.
	mockOpBot := opBot{}

	for _, tt := range caseTests {
		for _, reg := range tt.reg {
			mockOpBot.Register(reg.cmd, reg.desc, reg.adminOnly, reg.pvtOnly, reg.enabled, mockOpBot.helpHandler)
		}

		// test Update instance.
		mockUpdate := tgbotapi.Update{
			UpdateID: chatID,
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{
					ID: int64(chatID),
				},
				MessageID: msgID,
			},
		}

		// Construct the expected Message argument to Send.
		wantMsg := tgbotapi.NewMessage(int64(chatID), tt.wantStr)
		wantMsg.ReplyToMessageID = msgID

		mockTelebot := &MockTelebot{}
		mockTelebot.On("Send", wantMsg).Return(tgbotapi.Message{}, nil).Once()

		mockOpBot.helpHandler(mockTelebot, mockUpdate)
		mockTelebot.AssertExpectations(t)
	}
}

func TestHackerHandler(t *testing.T) {
	// Prep a mock BotMedia module and create a bot with it as media module.
	mockBotMedia := &MockBotMedia{}
	mockOpBot := opBot{
		media: mockBotMedia,
	}

	// test Update instance.
	mockUpdate := tgbotapi.Update{
		UpdateID: int(chatID),
		Message: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{
				ID: chatID,
			},
			MessageID: msgID,
		},
	}

	// Mock DeleteMessage.
	wantDeleteMsgConfig := tgbotapi.DeleteMessageConfig{
		ChatID:    chatID,
		MessageID: msgID,
	}

	mockTelebot := &MockTelebot{}
	mockTelebot.On("DeleteMessage", wantDeleteMsgConfig).Return(tgbotapi.APIResponse{}, nil).Once()
	mockBotMedia.On("sendMedia", mockTelebot, mockUpdate, mock.Anything).Return(nil)

	mockOpBot.hackerHandler(mockTelebot, mockUpdate)

	mockTelebot.AssertExpectations(t)
}

func TestProcessLocationRequest(t *testing.T) {
	caseTests := []struct {
		chatType           string
		processLocationRet error
		sendMsg            string
	}{
		// Private chat, no error.
		{
			chatType: "private",
			sendMsg:  T("location_success"),
		},
		// Private chat, unable to process location.
		{
			chatType:           "private",
			processLocationRet: errors.New("dummy"),
			sendMsg:            T("location_fail"),
		},
		// Non-private chat.
		{
			chatType: "whatever",
		},
	}

	for _, tt := range caseTests {
		// Mock Objects
		mockGeoLocations := &MockGeoLocations{
			locationKey: locationKey,
		}
		mockGeoLocations.locationDB = "/tmp/test"

		mockTelebot := &MockTelebot{}
		mockOpBot := opBot{
			geolocations: mockGeoLocations,
		}

		mockLocation := &tgbotapi.Location{
			Latitude:  12.34,
			Longitude: 56.78,
		}

		// test Update instance.
		mockUpdate := tgbotapi.Update{
			UpdateID: int(chatID),
			Message: &tgbotapi.Message{
				From: &tgbotapi.User{
					ID: userID,
				},
				Chat: &tgbotapi.Chat{
					ID:   chatID,
					Type: tt.chatType,
				},
				MessageID: msgID,
				Location:  mockLocation,
			},
		}

		// Mocked calls: Public (non-private) chat.
		mockGeoLocations.On("processLocation", userID, mockLocation.Latitude, mockLocation.Longitude).Return(tt.processLocationRet).Once()

		if tt.chatType == "private" {
			wantMsg := tgbotapi.NewMessage(int64(chatID), tt.sendMsg)
			wantMsg.ReplyToMessageID = msgID
			mockTelebot.On("Send", wantMsg).Return(tgbotapi.Message{}, nil).Once()
		}
		mockOpBot.processLocationRequest(mockTelebot, mockUpdate)
		mockTelebot.AssertExpectations(t)
	}
}

func TestProcessBotJoin(t *testing.T) {
	caseTests := []struct {
		kickBots     bool     // Should we kick bots?
		isBot        bool     // Is this user a bot?
		username     string   // User name (or bot name)
		botWhitelist []string // bot name whitelist.
		wantBan      bool     // Ban expected?
	}{
		// Kickbot enabled, Regular user (not bot): Do not ban.
		{
			kickBots: true,
		},
		// Kickbot enabled, Bot, no whitelist: Ban.
		{
			kickBots: true,
			isBot:    true,
			wantBan:  true,
		},
		// Kickbot enabled, Bot, whitelisted: No Ban.
		{
			kickBots:     true,
			isBot:        true,
			username:     "friend-bot",
			botWhitelist: []string{"friend-bot"},
		},
		// Kickbot enabled, Bot, not whitelisted: Ban.
		{
			kickBots:     true,
			isBot:        true,
			username:     "bad-bot",
			botWhitelist: []string{"friend-bot"},
			wantBan:      true,
		},
		// Kickbot disabled, Bot: Do not ban.
		{
			isBot: true,
		},
	}

	for _, tt := range caseTests {
		mockTelebot := &MockTelebot{}
		mockOpBot := opBot{
			config: botConfig{
				KickBots:     tt.kickBots,
				BotWhitelist: tt.botWhitelist,
			},
		}

		// test Update instance.
		mockUpdate := tgbotapi.Update{
			UpdateID: int(chatID),
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{
					ID: chatID,
				},
				NewChatMembers: &[]tgbotapi.User{
					{
						ID:       userID,
						UserName: tt.username,
						IsBot:    tt.isBot,
					},
				},
			},
		}

		wantKick := tgbotapi.KickChatMemberConfig{
			ChatMemberConfig: tgbotapi.ChatMemberConfig{
				ChatID: chatID,
				UserID: userID,
			},
		}
		mockTelebot.On("KickChatMember", wantKick).Return(tgbotapi.APIResponse{}, nil).Once()

		mockOpBot.processBotJoin(mockTelebot, mockUpdate)

		// Should a ban have happened?
		if tt.wantBan {
			log.Println("Asserting wantkick", wantKick)
			mockTelebot.AssertExpectations(t)
		} else {
			mockTelebot.AssertNumberOfCalls(t, "KickChatMember", 0)
		}
	}
}

func setup() error {
	var err error
	T, err = loadTranslation("../translations/en-us-all.toml")
	return err
}

func TestMain(m *testing.M) {
	if err := setup(); err != nil {
		log.Fatalf("Test setup error: %v", err)
	}
	code := m.Run()
	os.Exit(code)
}
