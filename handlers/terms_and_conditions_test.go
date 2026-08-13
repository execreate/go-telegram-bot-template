package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/execreate/go-telegram-bot-template/database/tables"
	"github.com/execreate/go-telegram-bot-template/internals/gin_server"
	"github.com/execreate/go-telegram-bot-template/locale"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
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

// stubAcceptor stands in for the users cache in the accept-terms flow.
type stubAcceptor struct {
	err       error
	userID    int64
	version   string
	callCount int
}

func (s *stubAcceptor) UserHasAcceptedTermsAndConditions(userID int64, version string) error {
	s.callCount++
	s.userID = userID
	s.version = version
	return s.err
}

// termsTexts resolves the T&C strings from a fixture.
func termsTexts(t *testing.T) *viper.Viper {
	t.Helper()

	dir := t.TempDir()
	content := `terms_and_conditions:
    request: Please accept the terms.
    changed: The terms have changed, please accept them again.
    accepted: Thank you for accepting.
    failed_to_accept: Could not record your acceptance.
`
	if err := os.WriteFile(filepath.Join(dir, "en.yaml"), []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write the fixture: %v", err)
	}
	locale.SetPath(dir)

	resolved, err := locale.GetTextTranslations("en")
	if err != nil {
		t.Fatalf("failed to resolve texts: %v", err)
	}

	return resolved
}

func TestTermsAndConditionsHandleUpdate(t *testing.T) {
	tests := []struct {
		name         string
		user         *tables.TelegramUser
		languages    []string
		wantText     string
		wantWebAppTo string
	}{
		{
			name:         "a user who never accepted gets the request copy",
			user:         &tables.TelegramUser{LanguageCode: "en"},
			languages:    []string{"en"},
			wantText:     "Please accept the terms.",
			wantWebAppTo: "https://app.example.com/terms_and_conditions.en.html",
		},
		{
			// Having accepted before but on an older version is the "changed" case.
			name: "a user who accepted an older version gets the changed copy",
			user: &tables.TelegramUser{
				LanguageCode:                      "en",
				AcceptedTermsAndConditionsOn:      sql.NullTime{Time: time.Now(), Valid: true},
				AcceptedTermsAndConditionsVersion: sql.NullString{String: "v0.9.0", Valid: true},
			},
			languages:    []string{"en"},
			wantText:     "The terms have changed, please accept them again.",
			wantWebAppTo: "https://app.example.com/terms_and_conditions.en.html",
		},
		{
			name:         "the page is served in the user's language when supported",
			user:         &tables.TelegramUser{LanguageCode: "de"},
			languages:    []string{"en", "de"},
			wantText:     "Please accept the terms.",
			wantWebAppTo: "https://app.example.com/terms_and_conditions.de.html",
		},
		{
			// An unsupported language would 404 on the static file, so it falls back.
			name:         "an unsupported language falls back to the English page",
			user:         &tables.TelegramUser{LanguageCode: "kl"},
			languages:    []string{"en", "de"},
			wantText:     "Please accept the terms.",
			wantWebAppTo: "https://app.example.com/terms_and_conditions.en.html",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeTelegram(t)
			b := fake.bot(t)

			handler := &TermsAndConditionsHandler{
				htmlFilename:       "terms_and_conditions",
				version:            testTermsVersion,
				supportedLanguages: tt.languages,
			}

			ctx := contextFor(b, privateMessage("hi"), map[string]any{
				"texts":         termsTexts(t),
				"db_user":       tt.user,
				"webapp_domain": "https://app.example.com",
			})

			err := handler.HandleUpdate(b, ctx)

			// The gate ends the handler groups so nothing downstream runs for a user
			// who has not accepted.
			if !errors.Is(err, ext.EndGroups) {
				t.Fatalf("HandleUpdate() = %v, want ext.EndGroups", err)
			}

			call := fake.only(t, "sendMessage")
			if call.params["text"] != tt.wantText {
				t.Errorf("reply = %q, want %q", call.params["text"], tt.wantText)
			}

			markup := decodeReplyMarkup(t, call)
			if len(markup.InlineKeyboard) != 1 || len(markup.InlineKeyboard[0]) != 1 {
				t.Fatalf("reply markup = %+v, want a single button", markup.InlineKeyboard)
			}
			button := markup.InlineKeyboard[0][0]
			if button.WebApp == nil {
				t.Fatal("the button has no WebApp payload")
			}
			if button.WebApp.Url != tt.wantWebAppTo {
				t.Errorf("WebApp URL = %q, want %q", button.WebApp.Url, tt.wantWebAppTo)
			}
		})
	}
}

func TestTermsAndConditionsHandleUpdatePropagatesSendFailures(t *testing.T) {
	fake := newFakeTelegram(t)
	fake.failMethod("sendMessage", "Bad Request: chat not found")
	b := fake.bot(t)

	handler := &TermsAndConditionsHandler{
		htmlFilename:       "terms_and_conditions",
		version:            testTermsVersion,
		supportedLanguages: []string{"en"},
	}

	ctx := contextFor(b, privateMessage("hi"), map[string]any{
		"texts":         termsTexts(t),
		"db_user":       &tables.TelegramUser{LanguageCode: "en"},
		"webapp_domain": "https://app.example.com",
	})

	err := handler.HandleUpdate(b, ctx)

	if err == nil || errors.Is(err, ext.EndGroups) {
		t.Errorf("HandleUpdate() = %v, want the send failure rather than EndGroups", err)
	}
}

func TestHandleAcceptTermsAndConditions(t *testing.T) {
	tests := []struct {
		name        string
		acceptorErr error
		wantMessage string
	}{
		{
			name:        "acceptance recorded",
			wantMessage: "Thank you for accepting.",
		},
		{
			// The WebApp still gets a 200 — the user is told over Telegram instead.
			name:        "acceptance failed",
			acceptorErr: errors.New("database is down"),
			wantMessage: "Could not record your acceptance.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeTelegram(t)
			acceptor := &stubAcceptor{err: tt.acceptorErr}

			handler := &TermsAndConditionsHandler{
				users:              acceptor,
				sender:             fake.bot(t),
				htmlFilename:       "terms_and_conditions",
				version:            testTermsVersion,
				supportedLanguages: []string{"en"},
			}

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)

			handler.handleAcceptTermsAndConditions(
				c,
				&gin_server.TgWebAppUser{ID: 42, FirstName: "Ada"},
				termsTexts(t),
			)

			if acceptor.callCount != 1 {
				t.Fatalf("the acceptor was called %d times, want 1", acceptor.callCount)
			}
			if acceptor.userID != 42 {
				t.Errorf("acceptance recorded for user %d, want 42", acceptor.userID)
			}
			if acceptor.version != testTermsVersion {
				t.Errorf("acceptance recorded for version %q, want %q", acceptor.version, testTermsVersion)
			}

			if recorder.Code != http.StatusOK {
				t.Errorf("status = %d, want 200 — the WebApp is answered either way", recorder.Code)
			}

			if got := fake.only(t, "sendMessage").params["text"]; got != tt.wantMessage {
				t.Errorf("message = %q, want %q", got, tt.wantMessage)
			}
		})
	}
}

func TestHandleAcceptTermsAndConditionsAnswersEvenIfTheNotificationFails(t *testing.T) {
	fake := newFakeTelegram(t)
	// The user may have blocked the bot; the WebApp still needs an answer.
	fake.failMethod("sendMessage", "Forbidden: bot was blocked by the user")

	handler := &TermsAndConditionsHandler{
		users:              &stubAcceptor{},
		sender:             fake.bot(t),
		htmlFilename:       "terms_and_conditions",
		version:            testTermsVersion,
		supportedLanguages: []string{"en"},
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	handler.handleAcceptTermsAndConditions(
		c,
		&gin_server.TgWebAppUser{ID: 42},
		termsTexts(t),
	)

	if recorder.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", recorder.Code)
	}
}

func TestNameIsStable(t *testing.T) {
	handler := &TermsAndConditionsHandler{}
	if got := handler.Name(); got != "TermsAndConditionsHandler" {
		t.Errorf("Name() = %q, want %q", got, "TermsAndConditionsHandler")
	}
}
