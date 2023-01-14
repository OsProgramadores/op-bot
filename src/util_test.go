// Unit tests for the util module.
package main

import (
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func TestIsForwarded(t *testing.T) {
	caseTests := []struct {
		message  *tgbotapi.Message // Message from the received update.
		expected bool              // Result expected.
	}{
		{
			// nil message.
			message:  nil,
			expected: false,
		},
		{
			// Regular message.
			message: &tgbotapi.Message{
				Text: "something here",
			},
			expected: false,
		},
		{
			// Forwarded message with ForwardFrom.
			message: &tgbotapi.Message{
				ForwardFrom: &tgbotapi.User{
					ID:        42,
					FirstName: "Foo",
					LastName:  "Bar",
				},
				Text: "A simple forwarded message",
			},
			expected: true,
		},
		{
			// Forwarded message with ForwardFromChat.
			message: &tgbotapi.Message{
				ForwardFromChat: &tgbotapi.Chat{
					ID:    42,
					Type:  "channel",
					Title: "Foo Bar",
				},
				Text: "Another forwarded message",
			},
			expected: true,
		},
	}

	for _, tt := range caseTests {
		if isForwarded(tt.message) != tt.expected {
			t.Errorf("isForwarded handled %v incorrectly; expected: %v, got: %v", tt.message, tt.expected, !tt.expected)
		}
	}
}
