package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/execreate/go-telegram-bot-template/database/tables"
	"github.com/execreate/go-telegram-bot-template/locale"
)

// useLocaleFixture writes command translations to a temp directory and points the
// locale package at it. Without locale.SetPath the package would resolve ./locale
// relative to this directory and silently fall back.
func useLocaleFixture(t *testing.T, files map[string]string) {
	t.Helper()

	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatalf("failed to write fixture %s: %v", name, err)
		}
	}

	locale.SetPath(dir)
}

// fullFixture covers every scope the template publishes, with a distinct description
// per scope so a test can tell which key a command came from.
const fullFixture = `default:
    hello: default hello
all_private_chats:
    hello: private hello
all_group_chats:
    hello: group hello
all_chat_administrators:
    hello: chat admin hello
bot_admin:
    hello: bot admin hello
    my_id: bot admin my_id
bot_owner:
    hello: bot owner hello
    my_id: bot owner my_id
`

func commandMap(cmds []gotgbot.BotCommand) map[string]string {
	result := make(map[string]string, len(cmds))
	for _, cmd := range cmds {
		result[cmd.Command] = cmd.Description
	}
	return result
}

func TestGetCommandsPublishesEveryPopulatedScope(t *testing.T) {
	useLocaleFixture(t, map[string]string{"en_commands.yaml": fullFixture})

	got, err := GetCommands(nil, "en")
	if err != nil {
		t.Fatalf("GetCommands() unexpected error: %v", err)
	}

	// The four global scopes, in the order the locale keys are handled.
	wantScopes := []struct {
		scope       gotgbot.BotCommandScope
		description string
	}{
		{&gotgbot.BotCommandScopeDefault{}, "default hello"},
		{&gotgbot.BotCommandScopeAllPrivateChats{}, "private hello"},
		{&gotgbot.BotCommandScopeAllGroupChats{}, "group hello"},
		{&gotgbot.BotCommandScopeAllChatAdministrators{}, "chat admin hello"},
	}

	if len(got) != len(wantScopes) {
		t.Fatalf("GetCommands() returned %d entries, want %d", len(got), len(wantScopes))
	}

	for i, want := range wantScopes {
		if got[i].Opts.Scope.GetType() != want.scope.GetType() {
			t.Errorf("entry %d scope = %q, want %q", i, got[i].Opts.Scope.GetType(), want.scope.GetType())
		}
		if got[i].Opts.LanguageCode != "en" {
			t.Errorf("entry %d language = %q, want %q", i, got[i].Opts.LanguageCode, "en")
		}
		if got := commandMap(got[i].Commands)["hello"]; got != want.description {
			t.Errorf("entry %d hello = %q, want %q", i, got, want.description)
		}
	}
}

func TestGetCommandsSkipsEmptyScopes(t *testing.T) {
	// Only two scopes are populated; the rest must not be published as empty lists,
	// which would wipe whatever is already registered with Telegram.
	useLocaleFixture(t, map[string]string{
		"en_commands.yaml": "all_private_chats:\n    hello: private hello\nall_group_chats:\n    hello: group hello\n",
	})

	got, err := GetCommands(nil, "en")
	if err != nil {
		t.Fatalf("GetCommands() unexpected error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("GetCommands() returned %d entries, want 2", len(got))
	}
	if got[0].Opts.Scope.GetType() != (&gotgbot.BotCommandScopeAllPrivateChats{}).GetType() {
		t.Errorf("first entry scope = %q, want all_private_chats", got[0].Opts.Scope.GetType())
	}
	if got[1].Opts.Scope.GetType() != (&gotgbot.BotCommandScopeAllGroupChats{}).GetType() {
		t.Errorf("second entry scope = %q, want all_group_chats", got[1].Opts.Scope.GetType())
	}
}

func TestGetCommandsScopesAdminsAndOwnersToTheirChats(t *testing.T) {
	useLocaleFixture(t, map[string]string{"en_commands.yaml": fullFixture})

	admin := &tables.TelegramUser{SoftDeleteModel: tables.SoftDeleteModel{ID: 11}, IsAdmin: true}
	owner := &tables.TelegramUser{SoftDeleteModel: tables.SoftDeleteModel{ID: 22}, IsOwner: true}
	both := &tables.TelegramUser{SoftDeleteModel: tables.SoftDeleteModel{ID: 33}, IsAdmin: true, IsOwner: true}

	got, err := GetCommands([]*tables.TelegramUser{admin, owner, both}, "en")
	if err != nil {
		t.Fatalf("GetCommands() unexpected error: %v", err)
	}

	// Four global scopes, then one entry for the admin, one for the owner, and two for
	// the user who is both.
	const globalScopes = 4
	perUser := got[globalScopes:]
	if len(perUser) != 4 {
		t.Fatalf("GetCommands() returned %d per-user entries, want 4", len(perUser))
	}

	wantPerUser := []struct {
		chatID      int64
		description string
	}{
		{11, "bot admin my_id"},
		{22, "bot owner my_id"},
		{33, "bot admin my_id"},
		{33, "bot owner my_id"},
	}

	for i, want := range wantPerUser {
		scope, ok := perUser[i].Opts.Scope.(*gotgbot.BotCommandScopeChat)
		if !ok {
			t.Fatalf("per-user entry %d scope is %T, want *gotgbot.BotCommandScopeChat", i, perUser[i].Opts.Scope)
		}
		if scope.ChatId != want.chatID {
			t.Errorf("per-user entry %d scoped to chat %d, want %d", i, scope.ChatId, want.chatID)
		}
		if got := commandMap(perUser[i].Commands)["my_id"]; got != want.description {
			t.Errorf("per-user entry %d my_id = %q, want %q", i, got, want.description)
		}
	}
}

func TestGetCommandsFallsBackToEnglish(t *testing.T) {
	useLocaleFixture(t, map[string]string{"en_commands.yaml": fullFixture})

	got, err := GetCommands(nil, "kl")
	if err != nil {
		t.Fatalf("GetCommands() unexpected error: %v", err)
	}

	if len(got) == 0 {
		t.Fatal("GetCommands() returned no entries for an unknown language")
	}
	if desc := commandMap(got[0].Commands)["hello"]; desc != "default hello" {
		t.Errorf("hello = %q, want the English fallback", desc)
	}
	// The scope still carries the requested language, so Telegram serves these
	// descriptions to users with that language_code.
	if got[0].Opts.LanguageCode != "kl" {
		t.Errorf("language = %q, want %q", got[0].Opts.LanguageCode, "kl")
	}
}

func TestGetCommandsWithoutLocaleFiles(t *testing.T) {
	useLocaleFixture(t, map[string]string{})

	if _, err := GetCommands(nil, "en"); err == nil {
		t.Error("GetCommands() with no locale files returned no error")
	}
}

func TestGetUserCommands(t *testing.T) {
	useLocaleFixture(t, map[string]string{
		"en_commands.yaml": fullFixture,
		"de_commands.yaml": "default:\n    hello: hallo\nbot_admin:\n    hello: hallo admin\n",
	})

	tests := []struct {
		name string
		user *tables.TelegramUser
		want map[string]string
	}{
		{
			name: "regular user gets the default set",
			user: &tables.TelegramUser{LanguageCode: "en"},
			want: map[string]string{"hello": "default hello"},
		},
		{
			name: "admin gets the bot_admin set",
			user: &tables.TelegramUser{LanguageCode: "en", IsAdmin: true},
			want: map[string]string{"hello": "bot admin hello", "my_id": "bot admin my_id"},
		},
		{
			name: "owner gets the bot_owner set",
			user: &tables.TelegramUser{LanguageCode: "en", IsOwner: true},
			want: map[string]string{"hello": "bot owner hello", "my_id": "bot owner my_id"},
		},
		{
			// IsAdmin is checked first, so a user who is both gets the admin set.
			name: "admin and owner gets the bot_admin set",
			user: &tables.TelegramUser{LanguageCode: "en", IsAdmin: true, IsOwner: true},
			want: map[string]string{"hello": "bot admin hello", "my_id": "bot admin my_id"},
		},
		{
			name: "descriptions come from the user's locale",
			user: &tables.TelegramUser{LanguageCode: "de", IsAdmin: true},
			want: map[string]string{"hello": "hallo admin"},
		},
		{
			name: "unknown locale falls back to English",
			user: &tables.TelegramUser{LanguageCode: "kl"},
			want: map[string]string{"hello": "default hello"},
		},
		{
			name: "empty locale falls back to English",
			user: &tables.TelegramUser{},
			want: map[string]string{"hello": "default hello"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmds, err := GetUserCommands(tt.user)
			if err != nil {
				t.Fatalf("GetUserCommands() unexpected error: %v", err)
			}

			got := commandMap(cmds)
			if len(got) != len(tt.want) {
				t.Fatalf("GetUserCommands() = %v, want %v", got, tt.want)
			}
			for cmd, description := range tt.want {
				if got[cmd] != description {
					t.Errorf("command %q = %q, want %q", cmd, got[cmd], description)
				}
			}
		})
	}
}
