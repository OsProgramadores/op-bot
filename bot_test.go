package main

import (
	"github.com/stretchr/testify/mock"
	"gopkg.in/telegram-bot-api.v4"
	"testing"
)

// Test objects.
type MockBotAPI struct {
	mock.Mock
}

func (m *MockBotAPI) Send(msg tgbotapi.MessageConfig) (tgbotapi.MessageConfig, error) {
	args := m.Called(msg)
	return args.Get(0).(tgbotapi.MessageConfig), args.Error(1)
}

func TestHelpHandler(t *testing.T) {
	b := opBot{}
	b.Register("foo", "mock_foo_command", false, true, true, b.helpHandler)
	b.Register("bar", "mock_bar_command", false, true, true, b.helpHandler)

	chatID := 1234
	msgID := 2222

	u := tgbotapi.Update{
		UpdateID: chatID,
		Message: &tgbotapi.Message{
			MessageID: msgID,
		},
	}
	msg := tgbotapi.NewMessage(int64(chatID), "mock_foo_command\nmock_bar_command")
	msg.ReplyToMessageID = msgID
	msg.ReplyMarkup = "markdown"

	testBot := MockBotAPI{}
	testBot.On("Send", mock.Anything).Return(mock.Anything, nil)
	b.helpHandler(testBot, u)
}
