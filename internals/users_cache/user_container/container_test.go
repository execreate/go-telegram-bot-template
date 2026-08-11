package user_container

import (
	"database/sql"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/execreate/go-telegram-bot-template/database/tables"
)

func newUser(firstName, lastName, langCode string, username sql.NullString) *tables.TelegramUser {
	return &tables.TelegramUser{
		SoftDeleteModel: tables.SoftDeleteModel{ID: 42},
		FirstName:       firstName,
		LastName:        lastName,
		LanguageCode:    langCode,
		Username:        username,
	}
}

func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func TestContainerGet(t *testing.T) {
	tests := []struct {
		name          string
		cached        *tables.TelegramUser
		update        *gotgbot.User
		wantChanged   bool
		wantUsername  sql.NullString
		wantFirstName string
		wantLangCode  string
	}{
		{
			name:          "no changes",
			cached:        newUser("Ada", "Lovelace", "en", nullString("ada")),
			update:        &gotgbot.User{Id: 42, FirstName: "Ada", LastName: "Lovelace", LanguageCode: "en", Username: "ada"},
			wantChanged:   false,
			wantUsername:  nullString("ada"),
			wantFirstName: "Ada",
			wantLangCode:  "en",
		},
		{
			// Regression: a username swapped for another was silently dropped, leaving
			// the cache (and the database) on the stale value.
			name:          "username replaced by another",
			cached:        newUser("Ada", "Lovelace", "en", nullString("old")),
			update:        &gotgbot.User{Id: 42, FirstName: "Ada", LastName: "Lovelace", LanguageCode: "en", Username: "new"},
			wantChanged:   true,
			wantUsername:  nullString("new"),
			wantFirstName: "Ada",
			wantLangCode:  "en",
		},
		{
			name:          "username set for the first time",
			cached:        newUser("Ada", "Lovelace", "en", sql.NullString{}),
			update:        &gotgbot.User{Id: 42, FirstName: "Ada", LastName: "Lovelace", LanguageCode: "en", Username: "ada"},
			wantChanged:   true,
			wantUsername:  nullString("ada"),
			wantFirstName: "Ada",
			wantLangCode:  "en",
		},
		{
			name:          "username cleared",
			cached:        newUser("Ada", "Lovelace", "en", nullString("ada")),
			update:        &gotgbot.User{Id: 42, FirstName: "Ada", LastName: "Lovelace", LanguageCode: "en"},
			wantChanged:   true,
			wantUsername:  sql.NullString{},
			wantFirstName: "Ada",
			wantLangCode:  "en",
		},
		{
			name:          "first name changed",
			cached:        newUser("Ada", "Lovelace", "en", nullString("ada")),
			update:        &gotgbot.User{Id: 42, FirstName: "Augusta", LastName: "Lovelace", LanguageCode: "en", Username: "ada"},
			wantChanged:   true,
			wantUsername:  nullString("ada"),
			wantFirstName: "Augusta",
			wantLangCode:  "en",
		},
		{
			name:          "language code changed",
			cached:        newUser("Ada", "Lovelace", "en", nullString("ada")),
			update:        &gotgbot.User{Id: 42, FirstName: "Ada", LastName: "Lovelace", LanguageCode: "de", Username: "ada"},
			wantChanged:   true,
			wantUsername:  nullString("ada"),
			wantFirstName: "Ada",
			wantLangCode:  "de",
		},
		{
			name:          "no username before or after",
			cached:        newUser("Ada", "Lovelace", "en", sql.NullString{}),
			update:        &gotgbot.User{Id: 42, FirstName: "Ada", LastName: "Lovelace", LanguageCode: "en"},
			wantChanged:   false,
			wantUsername:  sql.NullString{},
			wantFirstName: "Ada",
			wantLangCode:  "en",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := NewTelegramUserContainer(tt.cached)

			got, changed := container.Get(tt.update)

			if changed != tt.wantChanged {
				t.Errorf("changed = %v, want %v", changed, tt.wantChanged)
			}
			if got.Username != tt.wantUsername {
				t.Errorf("Username = %+v, want %+v", got.Username, tt.wantUsername)
			}
			if got.FirstName != tt.wantFirstName {
				t.Errorf("FirstName = %q, want %q", got.FirstName, tt.wantFirstName)
			}
			if got.LanguageCode != tt.wantLangCode {
				t.Errorf("LanguageCode = %q, want %q", got.LanguageCode, tt.wantLangCode)
			}

			// The merged values must also be visible to the next reader of the cache,
			// since that is what the deferred database write is built from.
			cached := container.GetRaw()
			if cached.Username != tt.wantUsername {
				t.Errorf("cached Username = %+v, want %+v", cached.Username, tt.wantUsername)
			}
			if cached.FirstName != tt.wantFirstName {
				t.Errorf("cached FirstName = %q, want %q", cached.FirstName, tt.wantFirstName)
			}
		})
	}
}

func TestContainerGetReturnsCopy(t *testing.T) {
	container := NewTelegramUserContainer(newUser("Ada", "Lovelace", "en", nullString("ada")))

	got, _ := container.Get(&gotgbot.User{Id: 42, FirstName: "Ada", LastName: "Lovelace", LanguageCode: "en", Username: "ada"})
	got.FirstName = "mutated"
	got.Username = nullString("mutated")

	cached := container.GetRaw()
	if cached.FirstName != "Ada" {
		t.Errorf("mutating the returned user changed the cache: FirstName = %q, want %q", cached.FirstName, "Ada")
	}
	if cached.Username.String != "ada" {
		t.Errorf("mutating the returned user changed the cache: Username = %q, want %q", cached.Username.String, "ada")
	}
}

func TestContainerIsStale(t *testing.T) {
	container := NewTelegramUserContainer(newUser("Ada", "", "en", nullString("ada")))

	if container.IsStale(time.Hour) {
		t.Error("a freshly created container must not be stale")
	}
	if !container.IsStale(-time.Second) {
		t.Error("a container must be stale once the threshold has elapsed")
	}

	// Any access counts as activity and resets the staleness window.
	container.Get(&gotgbot.User{Id: 42, FirstName: "Ada", LanguageCode: "en", Username: "ada"})
	if container.IsStale(time.Hour) {
		t.Error("a container accessed just now must not be stale")
	}
}

func TestContainerTermsAndConditionsAccepted(t *testing.T) {
	container := NewTelegramUserContainer(newUser("Ada", "", "en", nullString("ada")))
	acceptedOn := time.Date(2026, time.August, 11, 12, 0, 0, 0, time.UTC)

	container.TermsAndConditionsAccepted(acceptedOn, "v2.0.0")

	user := container.GetRaw()
	if !user.AcceptedTermsAndConditionsOn.Valid || !user.AcceptedTermsAndConditionsOn.Time.Equal(acceptedOn) {
		t.Errorf("AcceptedTermsAndConditionsOn = %+v, want %v", user.AcceptedTermsAndConditionsOn, acceptedOn)
	}
	if !user.AcceptedTermsAndConditionsVersion.Valid || user.AcceptedTermsAndConditionsVersion.String != "v2.0.0" {
		t.Errorf("AcceptedTermsAndConditionsVersion = %+v, want %q", user.AcceptedTermsAndConditionsVersion, "v2.0.0")
	}
	if user.MustAcceptTermsAndConditions("v2.0.0") {
		t.Error("user must not be asked to re-accept the version they just accepted")
	}
}

func TestContainerConcurrentAccess(t *testing.T) {
	container := NewTelegramUserContainer(newUser("Ada", "Lovelace", "en", nullString("ada")))

	done := make(chan struct{})
	for i := range 8 {
		go func(i int) {
			defer func() { done <- struct{}{} }()
			for range 100 {
				container.Get(&gotgbot.User{Id: 42, FirstName: "Ada", LastName: "Lovelace", LanguageCode: "en", Username: "ada"})
				container.GetRaw()
				container.IsStale(time.Hour)
				container.TermsAndConditionsAccepted(time.Now(), "v1.0.0")
			}
		}(i)
	}
	for range 8 {
		<-done
	}
}
