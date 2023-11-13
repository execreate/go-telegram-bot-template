package tables

import (
	"database/sql"
)

type TelegramUser struct {
	SoftDeleteModel

	FirstName    string         `db:"first_name"`
	LastName     string         `db:"last_name"`
	Username     sql.NullString `db:"username"`
	LanguageCode string         `db:"language_code"`

	IsAdmin bool `db:"is_admin"`

	AcceptedTermsAndConditionsOn     sql.NullTime `db:"accepted_terms_and_conditions_on"`
	AcceptedLatestTermsAndConditions bool         `db:"accepted_latest_terms_and_conditions"`
}

func (u *TelegramUser) FullName() string {
	name := u.FirstName
	if u.LastName != "" {
		name += " " + u.LastName
	}
	return name
}
