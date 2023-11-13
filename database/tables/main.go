package tables

import (
	"database/sql"
	"time"
)

type SoftDeleteModel struct {
	ID        int64        `db:"id"`
	CreatedAt time.Time    `db:"created_at"`
	UpdatedAt time.Time    `db:"updated_at"`
	DeletedAt sql.NullTime `db:"deleted_at"`
}
