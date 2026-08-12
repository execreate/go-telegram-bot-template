package helpers

import (
	"database/sql"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/execreate/go-telegram-bot-template/database/tables"
)

// contextWithMessage wraps msg in the update shape a private text message arrives in.
func contextWithMessage(msg *gotgbot.Message) *ext.Context {
	return ext.NewContext(&gotgbot.Bot{}, &gotgbot.Update{Message: msg}, nil)
}

func TestContainsMessageViaBot(t *testing.T) {
	const (
		text        = "shared via inline mode"
		botUsername = "my_test_bot"
	)

	tests := []struct {
		name string
		msg  *gotgbot.Message
		want bool
	}{
		{
			name: "sent via the bot and not forwarded",
			msg: &gotgbot.Message{
				Text:   text,
				ViaBot: &gotgbot.User{Username: botUsername},
			},
			want: true,
		},
		{
			name: "forwarded copy of a message sent via the bot",
			msg: &gotgbot.Message{
				Text:          text,
				ViaBot:        &gotgbot.User{Username: botUsername},
				ForwardOrigin: gotgbot.MessageOriginUser{Date: 1700000000},
			},
			want: false,
		},
		{
			name: "not sent via any bot",
			msg:  &gotgbot.Message{Text: text},
			want: false,
		},
		{
			name: "sent via a different bot",
			msg: &gotgbot.Message{
				Text:   text,
				ViaBot: &gotgbot.User{Username: "someone_elses_bot"},
			},
			want: false,
		},
		{
			name: "text does not match",
			msg: &gotgbot.Message{
				Text:   "something else",
				ViaBot: &gotgbot.User{Username: botUsername},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContainsMessageViaBot(text, botUsername, contextWithMessage(tt.msg)); got != tt.want {
				t.Errorf("ContainsMessageViaBot() = %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("no effective message", func(t *testing.T) {
		ctx := ext.NewContext(&gotgbot.Bot{}, &gotgbot.Update{}, nil)
		if ContainsMessageViaBot(text, botUsername, ctx) {
			t.Error("ContainsMessageViaBot() = true, want false")
		}
	})
}

func TestFormDataHasKeys(t *testing.T) {
	tests := []struct {
		name     string
		keys     []string
		formData map[string][]string
		want     bool
	}{
		{
			name:     "all keys present",
			keys:     []string{"user", "hash"},
			formData: map[string][]string{"user": {"{}"}, "hash": {"abc"}, "extra": {"1"}},
			want:     true,
		},
		{
			name:     "nil form data",
			keys:     []string{"user"},
			formData: nil,
			want:     false,
		},
		{
			name:     "no keys required",
			keys:     nil,
			formData: map[string][]string{},
			want:     true,
		},
		{
			name:     "missing key",
			keys:     []string{"user", "hash"},
			formData: map[string][]string{"user": {"{}"}},
			want:     false,
		},
		{
			name:     "key present with no values",
			keys:     []string{"hash"},
			formData: map[string][]string{"hash": {}},
			want:     false,
		},
		{
			name:     "key present with an empty value",
			keys:     []string{"hash"},
			formData: map[string][]string{"hash": {""}},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormDataHasKeys(tt.keys, tt.formData); got != tt.want {
				t.Errorf("FormDataHasKeys() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetUserMention(t *testing.T) {
	tests := []struct {
		name string
		user *tables.TelegramUser
		want string
	}{
		{
			name: "nil user",
			user: nil,
			want: "",
		},
		{
			name: "username set",
			user: &tables.TelegramUser{Username: sql.NullString{String: "durov", Valid: true}},
			want: "@durov",
		},
		{
			name: "no username falls back to an HTML link",
			user: &tables.TelegramUser{
				SoftDeleteModel: tables.SoftDeleteModel{ID: 42},
				FirstName:       "Ada",
				LastName:        "Lovelace",
			},
			want: `<a href="tg://user?id=42">Ada Lovelace</a>`,
		},
		{
			name: "no username and no last name",
			user: &tables.TelegramUser{
				SoftDeleteModel: tables.SoftDeleteModel{ID: 7},
				FirstName:       "Ada",
			},
			want: `<a href="tg://user?id=7">Ada</a>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetUserMention(tt.user); got != tt.want {
				t.Errorf("GetUserMention() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEscapeMarkdownChars(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "empty string",
			text: "",
			want: "",
		},
		{
			name: "nothing to escape",
			text: "plain text 123",
			want: "plain text 123",
		},
		{
			name: "every special character",
			text: `_*[]()~` + "`" + `>#+-=|{}.!`,
			want: `\_\*\[\]\(\)\~` + "\\`" + `\>\#\+\-\=\|\{\}\.\!`,
		},
		{
			name: "backslash is escaped",
			text: `a\b`,
			want: `a\\b`,
		},
		{
			// The backslash must be escaped before the character it precedes, otherwise
			// the output leaves a lone backslash escaping the escape.
			name: "backslash followed by a special character",
			text: `\_`,
			want: `\\\_`,
		},
		{
			name: "already escaped input is escaped again",
			text: `\.`,
			want: `\\\.`,
		},
		{
			name: "group title",
			text: "Team (EU)",
			want: `Team \(EU\)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EscapeMarkdownChars(tt.text); got != tt.want {
				t.Errorf("EscapeMarkdownChars(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}
