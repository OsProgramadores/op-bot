package main

import (
	"github.com/go-telegram-bot-api/telegram-bot-api"
)

// mediaInterface defines the interface between opbot and the media module.
type mediaInterface interface {
	loadMedia() error
	sendMedia(sender, tgbotapi.Update, string) error
}

// notificationsInterface defines the interface between opbot and notifications.
type notificationsInterface interface {
	loadNotificationSettings() error
	manageNotifications(sender, tgbotapi.Update) error
	notificationHandler(tgbotInterface, tgbotapi.Update) error
}

// bansInterface defines the interface between opbot and bans.
type bansInterface interface {
	banRequestHandler(tgbotInterface, tgbotapi.Update) error
	deleteMessageFromBanRequest(tgbotInterface, *tgbotapi.User, string, bool) error
	loadBanRequestsInfo() error
}

// geoLocationsInterface defines the interface between opbot and geo locations.
type geoLocationsInterface interface {
	processLocation(int, float64, float64) error
	readLocations() error
	serveLocations(int)
}

// tgbotInterface defines our main interface to the bot, via tgbotapi. All functions which need to
// perform operations using the bot api will use this interface. This allows us to easily
// mock the calls for testing.
type tgbotInterface interface {
	AnswerCallbackQuery(tgbotapi.CallbackConfig) (tgbotapi.APIResponse, error)
	DeleteMessage(tgbotapi.DeleteMessageConfig) (tgbotapi.APIResponse, error)
	GetChatAdministrators(tgbotapi.ChatConfig) ([]tgbotapi.ChatMember, error)
	GetChatMember(tgbotapi.ChatConfigWithUser) (tgbotapi.ChatMember, error)
	GetUpdatesChan(tgbotapi.UpdateConfig) (tgbotapi.UpdatesChannel, error)
	KickChatMember(tgbotapi.KickChatMemberConfig) (tgbotapi.APIResponse, error)
	UnbanChatMember(tgbotapi.ChatMemberConfig) (tgbotapi.APIResponse, error)
	Send(tgbotapi.Chattable) (tgbotapi.Message, error)
}

type sender interface {
	Send(tgbotapi.Chattable) (tgbotapi.Message, error)
}

type deleteMessager interface {
	DeleteMessage(tgbotapi.DeleteMessageConfig) (tgbotapi.APIResponse, error)
}

type sendDeleteMessager interface {
	Send(tgbotapi.Chattable) (tgbotapi.Message, error)
	DeleteMessage(tgbotapi.DeleteMessageConfig) (tgbotapi.APIResponse, error)
}

type getChatMemberer interface {
	GetChatMember(tgbotapi.ChatConfigWithUser) (tgbotapi.ChatMember, error)
}

type kickChatMemberer interface {
	KickChatMember(tgbotapi.KickChatMemberConfig) (tgbotapi.APIResponse, error)
}

type kickUnbanChatMemberer interface {
	KickChatMember(tgbotapi.KickChatMemberConfig) (tgbotapi.APIResponse, error)
	UnbanChatMember(tgbotapi.ChatMemberConfig) (tgbotapi.APIResponse, error)
}
