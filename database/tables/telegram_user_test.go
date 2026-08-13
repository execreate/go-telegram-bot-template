package tables

import (
	"database/sql"
	"testing"
	"time"
)

func TestFullName(t *testing.T) {
	tests := []struct {
		name string
		user TelegramUser
		want string
	}{
		{
			name: "both names",
			user: TelegramUser{FirstName: "Ada", LastName: "Lovelace"},
			want: "Ada Lovelace",
		},
		{
			name: "no last name",
			user: TelegramUser{FirstName: "Ada"},
			want: "Ada",
		},
		{
			name: "neither name",
			user: TelegramUser{},
			want: "",
		},
		{
			// Telegram marks first_name as required, so this shape does not arrive from
			// an update; the leading space is documented here rather than guarded against.
			name: "last name only",
			user: TelegramUser{LastName: "Lovelace"},
			want: " Lovelace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.user.FullName(); got != tt.want {
				t.Errorf("FullName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMustAcceptTermsAndConditions(t *testing.T) {
	const current = "v2.0.0"

	acceptedOn := sql.NullTime{Time: time.Now(), Valid: true}

	tests := []struct {
		name string
		user TelegramUser
		want bool
	}{
		{
			name: "never accepted anything",
			user: TelegramUser{},
			want: true,
		},
		{
			name: "accepted the current version",
			user: TelegramUser{
				AcceptedTermsAndConditionsOn:      acceptedOn,
				AcceptedTermsAndConditionsVersion: sql.NullString{String: current, Valid: true},
			},
			want: false,
		},
		{
			name: "accepted an older version",
			user: TelegramUser{
				AcceptedTermsAndConditionsOn:      acceptedOn,
				AcceptedTermsAndConditionsVersion: sql.NullString{String: "v1.0.0", Valid: true},
			},
			want: true,
		},
		{
			// Rolling the version back counts as a mismatch too — the check is equality,
			// not ordering, so it never tries to compare version strings.
			name: "accepted a newer version",
			user: TelegramUser{
				AcceptedTermsAndConditionsOn:      acceptedOn,
				AcceptedTermsAndConditionsVersion: sql.NullString{String: "v3.0.0", Valid: true},
			},
			want: true,
		},
		{
			// Only the version column decides. A timestamp with no version behind it —
			// a half-written row, or a fork that stopped recording versions — still
			// gates the user.
			name: "timestamp set but version NULL",
			user: TelegramUser{AcceptedTermsAndConditionsOn: acceptedOn},
			want: true,
		},
		{
			// The mirror image: the version alone is enough, the timestamp is only used
			// to pick the "terms changed" wording in the handler.
			name: "version set but timestamp NULL",
			user: TelegramUser{
				AcceptedTermsAndConditionsVersion: sql.NullString{String: current, Valid: true},
			},
			want: false,
		},
		{
			// A NULL version is not an empty version: Valid is false, so the stored
			// string is ignored entirely.
			name: "invalid version holding the current string",
			user: TelegramUser{
				AcceptedTermsAndConditionsVersion: sql.NullString{String: current, Valid: false},
			},
			want: true,
		},
		{
			name: "accepted an empty version string",
			user: TelegramUser{
				AcceptedTermsAndConditionsVersion: sql.NullString{String: "", Valid: true},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.user.MustAcceptTermsAndConditions(current); got != tt.want {
				t.Errorf("MustAcceptTermsAndConditions(%q) = %v, want %v", current, got, tt.want)
			}
		})
	}
}

func TestMustAcceptTermsAndConditionsWhenTheVersionIsBumped(t *testing.T) {
	user := TelegramUser{
		AcceptedTermsAndConditionsOn:      sql.NullTime{Time: time.Now(), Valid: true},
		AcceptedTermsAndConditionsVersion: sql.NullString{String: "v1.0.0", Valid: true},
	}

	if user.MustAcceptTermsAndConditions("v1.0.0") {
		t.Error("the user is gated on the version they accepted")
	}
	if !user.MustAcceptTermsAndConditions("v1.0.1") {
		t.Error("bumping the version did not re-gate the user")
	}
}
