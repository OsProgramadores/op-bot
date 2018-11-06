package main

import (
	"github.com/stretchr/testify/mock"
	"gopkg.in/telegram-bot-api.v4"
	"testing"
)

// MockBotface defines a Mock botface struct.
type MockBotface struct {
	mock.Mock
}

func (m *MockBotface) AnswerCallbackQuery(config tgbotapi.CallbackConfig) (tgbotapi.APIResponse, error) {
	args := m.Called(config)
	return args.Get(0).(tgbotapi.APIResponse), args.Error(1)
}

func (m *MockBotface) DeleteMessage(config tgbotapi.DeleteMessageConfig) (tgbotapi.APIResponse, error) {
	args := m.Called(config)
	return args.Get(0).(tgbotapi.APIResponse), args.Error(1)
}

func (m *MockBotface) GetChatAdministrators(config tgbotapi.ChatConfig) ([]tgbotapi.ChatMember, error) {
	args := m.Called(config)
	return args.Get(0).([]tgbotapi.ChatMember), args.Error(1)
}

func (m *MockBotface) GetUpdatesChan(config tgbotapi.UpdateConfig) (tgbotapi.UpdatesChannel, error) {
	args := m.Called(config)
	return args.Get(0).(tgbotapi.UpdatesChannel), args.Error(1)
}

func (m *MockBotface) KickChatMember(config tgbotapi.KickChatMemberConfig) (tgbotapi.APIResponse, error) {
	args := m.Called(config)
	return args.Get(0).(tgbotapi.APIResponse), args.Error(1)
}

func (m *MockBotface) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	args := m.Called(c)
	return args.Get(0).(tgbotapi.Message), args.Error(1)
}

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
	b := opBot{}

	for _, tt := range caseTests {

		for _, reg := range tt.reg {
			b.Register(reg.cmd, reg.desc, reg.adminOnly, reg.pvtOnly, reg.enabled, b.helpHandler)
		}

		chatID := 1234
		msgID := 2222

		// test Update instance.
		u := tgbotapi.Update{
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

		testBotface := &MockBotface{}
		testBotface.On("Send", wantMsg).Return(tgbotapi.Message{}, nil)
		b.helpHandler(testBotface, u)
	}
}
