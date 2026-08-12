package handlers

import (
	"database/sql"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/execreate/go-telegram-bot-template/database/tables"
)

const testTermsVersion = "v1.0.0"

// acceptedUser builds a user who has accepted the given terms version.
func acceptedUser(version string) *tables.TelegramUser {
	return &tables.TelegramUser{
		AcceptedTermsAndConditionsOn:      sql.NullTime{Time: time.Now(), Valid: true},
		AcceptedTermsAndConditionsVersion: sql.NullString{String: version, Valid: true},
	}
}

func TestTermsAndConditionsHandlerCheckUpdate(t *testing.T) {
	tests := []struct {
		name     string
		chatType string
		// noUser omits EffectiveUser from the update.
		noUser bool
		// dbUser is placed in ctx.Data["db_user"] as-is; a nil value leaves the key unset.
		dbUser any
		want   bool
	}{
		{
			name:     "private chat, terms never accepted",
			chatType: "private",
			dbUser:   &tables.TelegramUser{},
			want:     true,
		},
		{
			name:     "private chat, accepted an older version",
			chatType: "private",
			dbUser:   acceptedUser("v0.9.0"),
			want:     true,
		},
		{
			name:     "private chat, accepted the current version",
			chatType: "private",
			dbUser:   acceptedUser(testTermsVersion),
			want:     false,
		},
		{
			name:     "group chat is not gated",
			chatType: "supergroup",
			dbUser:   &tables.TelegramUser{},
			want:     false,
		},
		{
			name:     "no effective user",
			chatType: "private",
			noUser:   true,
			dbUser:   &tables.TelegramUser{},
			want:     false,
		},
		{
			// A fork that registers a handler at group -1 or lower, or reorders the
			// groups, can reach this before UserContextHandler has run. The update is
			// skipped rather than panicking the dispatcher.
			name:     "db_user missing from the context",
			chatType: "private",
			dbUser:   nil,
			want:     false,
		},
		{
			name:     "db_user holds an unexpected type",
			chatType: "private",
			dbUser:   "not a user",
			want:     false,
		},
	}

	handler := &TermsAndConditionsHandler{version: testTermsVersion}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &gotgbot.Message{Chat: gotgbot.Chat{Id: 1, Type: tt.chatType}}
			if !tt.noUser {
				msg.From = &gotgbot.User{Id: 42}
			}

			data := map[string]any{}
			if tt.dbUser != nil {
				data["db_user"] = tt.dbUser
			}

			ctx := ext.NewContext(&gotgbot.Bot{}, &gotgbot.Update{Message: msg}, data)

			if got := handler.CheckUpdate(nil, ctx); got != tt.want {
				t.Errorf("CheckUpdate() = %v, want %v", got, tt.want)
			}
		})
	}
}
