package contextual

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/execreate/go-telegram-bot-template/database/tables"
	"github.com/execreate/go-telegram-bot-template/internals/users_cache"
	"github.com/execreate/go-telegram-bot-template/locale"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/spf13/viper"
)

// useLocaleFixture points the locale package at translations for en and de.
func useLocaleFixture(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	files := map[string]string{
		"en.yaml": "hello: |-\n    Hello, %s!\n",
		"de.yaml": "hello: |-\n    Hallo, %s!\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatalf("failed to write fixture %s: %v", name, err)
		}
	}

	locale.SetPath(dir)
}

func contextFor(update *gotgbot.Update) *ext.Context {
	return ext.NewContext(&gotgbot.Bot{}, update, map[string]any{})
}

func messageUpdate(user *gotgbot.User) *gotgbot.Update {
	return &gotgbot.Update{Message: &gotgbot.Message{
		MessageId: 1,
		Chat:      gotgbot.Chat{Id: 42, Type: "private"},
		From:      user,
	}}
}

func TestMiscContextHandlerCheckUpdate(t *testing.T) {
	handler := NewMiscContextHandler("https://app.example.com")

	// Context enrichment runs for everything; there is nothing to filter on.
	if !handler.CheckUpdate(nil, contextFor(messageUpdate(nil))) {
		t.Error("CheckUpdate() = false, want every update enriched")
	}
	if !handler.CheckUpdate(nil, contextFor(&gotgbot.Update{})) {
		t.Error("CheckUpdate() = false for an empty update, want every update enriched")
	}
}

func TestMiscContextHandlerPopulatesTheContext(t *testing.T) {
	useLocaleFixture(t)

	tests := []struct {
		name      string
		user      *gotgbot.User
		wantHello string
	}{
		{
			name:      "user's language",
			user:      &gotgbot.User{Id: 42, FirstName: "Ada", LanguageCode: "de"},
			wantHello: "Hallo, %s!",
		},
		{
			name:      "unknown language falls back to English",
			user:      &gotgbot.User{Id: 42, FirstName: "Ada", LanguageCode: "kl"},
			wantHello: "Hello, %s!",
		},
		{
			name:      "no language falls back to English",
			user:      &gotgbot.User{Id: 42, FirstName: "Ada"},
			wantHello: "Hello, %s!",
		},
		{
			// A channel post has no sender, so there is no language to read.
			name:      "no effective user falls back to English",
			user:      nil,
			wantHello: "Hello, %s!",
		},
	}

	handler := NewMiscContextHandler("https://app.example.com")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := contextFor(messageUpdate(tt.user))

			if err := handler.HandleUpdate(nil, ctx); err != nil {
				t.Fatalf("HandleUpdate() unexpected error: %v", err)
			}

			if got := ctx.Data["webapp_domain"]; got != "https://app.example.com" {
				t.Errorf("webapp_domain = %v, want the configured domain", got)
			}

			texts, ok := ctx.Data["texts"].(*viper.Viper)
			if !ok {
				t.Fatalf("texts = %T, want *viper.Viper", ctx.Data["texts"])
			}
			if got := texts.GetString("hello"); got != tt.wantHello {
				t.Errorf("hello = %q, want %q", got, tt.wantHello)
			}
		})
	}
}

func TestMiscContextHandlerPropagatesTranslationFailures(t *testing.T) {
	// No locale files at all, so even the English fallback cannot resolve.
	locale.SetPath(t.TempDir())

	handler := NewMiscContextHandler("https://app.example.com")
	ctx := contextFor(messageUpdate(&gotgbot.User{Id: 42, LanguageCode: "en"}))

	if err := handler.HandleUpdate(nil, ctx); err == nil {
		t.Error("HandleUpdate() returned no error with no locale files")
	}
	if _, ok := ctx.Data["texts"]; ok {
		t.Error("HandleUpdate() populated texts despite failing")
	}
}

func TestMiscContextHandlerName(t *testing.T) {
	if got := NewMiscContextHandler("").Name(); got != "MiscContextHandler" {
		t.Errorf("Name() = %q, want %q", got, "MiscContextHandler")
	}
}

// stubQuerier is a users_cache.Querier that reports whatever the test needs. Query
// yields no rows, so the cache takes its insert-on-miss path.
type stubQuerier struct {
	execErr error
}

func (s *stubQuerier) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return &emptyRows{}, nil
}

func (s *stubQuerier) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	if s.execErr != nil {
		return pgconn.CommandTag{}, s.execErr
	}
	return pgconn.NewCommandTag("INSERT 1"), nil
}

// emptyRows yields nothing, which pgx.CollectOneRow turns into pgx.ErrNoRows.
type emptyRows struct{}

func (r *emptyRows) Close()                                       {}
func (r *emptyRows) Err() error                                   { return nil }
func (r *emptyRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *emptyRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *emptyRows) Next() bool                                   { return false }
func (r *emptyRows) Scan(...any) error                            { return pgx.ErrNoRows }
func (r *emptyRows) Values() ([]any, error)                       { return nil, pgx.ErrNoRows }
func (r *emptyRows) RawValues() [][]byte                          { return nil }
func (r *emptyRows) Conn() *pgx.Conn                              { return nil }

func TestUserContextHandlerCheckUpdate(t *testing.T) {
	handler := NewUserContextHandler(nil)

	tests := []struct {
		name   string
		update *gotgbot.Update
		want   bool
	}{
		{
			name:   "a message from a user",
			update: messageUpdate(&gotgbot.User{Id: 42}),
			want:   true,
		},
		{
			// A channel post has no sender, so there is no user to load.
			name: "a channel post",
			update: &gotgbot.Update{ChannelPost: &gotgbot.Message{
				MessageId: 1,
				Chat:      gotgbot.Chat{Id: -100123, Type: "channel"},
			}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := handler.CheckUpdate(nil, contextFor(tt.update)); got != tt.want {
				t.Errorf("CheckUpdate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserContextHandlerPopulatesDbUser(t *testing.T) {
	cache := users_cache.NewTgUsersCache(&stubQuerier{}, time.Hour, 4*24*time.Hour)
	handler := NewUserContextHandler(cache)

	ctx := contextFor(messageUpdate(&gotgbot.User{
		Id:           42,
		FirstName:    "Ada",
		LastName:     "Lovelace",
		Username:     "ada",
		LanguageCode: "en",
	}))

	if err := handler.HandleUpdate(nil, ctx); err != nil {
		t.Fatalf("HandleUpdate() unexpected error: %v", err)
	}

	user, ok := ctx.Data["db_user"].(*tables.TelegramUser)
	if !ok {
		t.Fatalf("db_user = %T, want *tables.TelegramUser", ctx.Data["db_user"])
	}
	if user.ID != 42 || user.FirstName != "Ada" {
		t.Errorf("db_user = %+v, want the user from the update", user)
	}
	if !user.Username.Valid || user.Username.String != "ada" {
		t.Errorf("db_user username = %+v, want a valid \"ada\"", user.Username)
	}
}

func TestUserContextHandlerPropagatesCacheErrors(t *testing.T) {
	wantErr := errors.New("database is down")

	cache := users_cache.NewTgUsersCache(&stubQuerier{execErr: wantErr}, time.Hour, 4*24*time.Hour)
	handler := NewUserContextHandler(cache)

	ctx := contextFor(messageUpdate(&gotgbot.User{Id: 42, FirstName: "Ada"}))

	// The error has to reach the dispatcher: continuing would leave later handlers
	// type-asserting a db_user that was never set.
	if err := handler.HandleUpdate(nil, ctx); !errors.Is(err, wantErr) {
		t.Errorf("HandleUpdate() error = %v, want %v", err, wantErr)
	}
	if _, ok := ctx.Data["db_user"]; ok {
		t.Error("HandleUpdate() populated db_user despite failing")
	}
}

func TestUserContextHandlerName(t *testing.T) {
	if got := NewUserContextHandler(nil).Name(); got != "UserContextHandler" {
		t.Errorf("Name() = %q, want %q", got, "UserContextHandler")
	}
}
