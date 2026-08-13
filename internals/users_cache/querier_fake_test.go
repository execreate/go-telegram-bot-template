package users_cache

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/execreate/go-telegram-bot-template/database/tables"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// call records one Query or Exec, so a test can assert on the statement and the
// arguments the cache sent.
type call struct {
	sql  string
	args []any
}

// fakeQuerier stands in for *pgxpool.Pool. By default Query reports "no rows", which
// is the miss path the cache reacts to; queue users to have it return them instead.
type fakeQuerier struct {
	mu      sync.Mutex
	queries []call
	execs   []call
	// users is dequeued one per Query. An empty queue means "no rows".
	users []tables.TelegramUser
	// execErr, when set, is returned by every Exec.
	execErr error
	// queryErr, when set, is surfaced through the returned rows.
	queryErr error
}

func (f *fakeQuerier) queueUsers(users ...tables.TelegramUser) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.users = append(f.users, users...)
}

func (f *fakeQuerier) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	f.mu.Lock()
	f.queries = append(f.queries, call{sql: sql, args: args})

	var next *tables.TelegramUser
	if len(f.users) > 0 {
		next = &f.users[0]
		f.users = f.users[1:]
	}
	queryErr := f.queryErr
	f.mu.Unlock()

	// pgxpool.Pool returns non-nil rows carrying the error rather than a nil interface,
	// and the cache relies on that: it defers rows.Close() before checking anything.
	if queryErr != nil {
		return &userRows{err: queryErr}, queryErr
	}
	if next == nil {
		return &userRows{}, nil
	}
	return &userRows{user: next}, nil
}

func (f *fakeQuerier) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.mu.Lock()
	f.execs = append(f.execs, call{sql: sql, args: args})
	execErr := f.execErr
	f.mu.Unlock()

	if execErr != nil {
		return pgconn.CommandTag{}, execErr
	}
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (f *fakeQuerier) recordedExecs() []call {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]call(nil), f.execs...)
}

func (f *fakeQuerier) recordedQueries() []call {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]call(nil), f.queries...)
}

// userColumns is what `select *` returns for telegram_users, in table order. The
// names have to match the struct's db tags or pgx.RowToStructByName refuses the row.
var userColumns = []string{
	"id",
	"created_at",
	"updated_at",
	"deleted_at",
	"first_name",
	"last_name",
	"username",
	"language_code",
	"is_admin",
	"is_owner",
	"accepted_terms_and_conditions_on",
	"accepted_terms_and_conditions_version",
}

func userValues(u *tables.TelegramUser) []any {
	return []any{
		u.ID,
		u.CreatedAt,
		u.UpdatedAt,
		u.DeletedAt,
		u.FirstName,
		u.LastName,
		u.Username,
		u.LanguageCode,
		u.IsAdmin,
		u.IsOwner,
		u.AcceptedTermsAndConditionsOn,
		u.AcceptedTermsAndConditionsVersion,
	}
}

// userRows is a pgx.Rows yielding at most one telegram_users row. With no user it
// yields nothing, which pgx.CollectOneRow turns into pgx.ErrNoRows.
type userRows struct {
	user     *tables.TelegramUser
	consumed bool
	err      error
}

func (r *userRows) Close() {}

func (r *userRows) Err() error { return r.err }

func (r *userRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }

func (r *userRows) FieldDescriptions() []pgconn.FieldDescription {
	descriptions := make([]pgconn.FieldDescription, 0, len(userColumns))
	for _, name := range userColumns {
		descriptions = append(descriptions, pgconn.FieldDescription{Name: name})
	}
	return descriptions
}

func (r *userRows) Next() bool {
	if r.err != nil || r.user == nil || r.consumed {
		return false
	}
	r.consumed = true
	return true
}

// Scan assigns into the pointers pgx built from the struct's fields. They arrive in
// FieldDescriptions order, so they line up with userValues.
func (r *userRows) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if r.user == nil {
		return pgx.ErrNoRows
	}

	values := userValues(r.user)
	if len(dest) != len(values) {
		return fmt.Errorf("scan target count %d does not match column count %d", len(dest), len(values))
	}

	for i, target := range dest {
		out := reflect.ValueOf(target)
		if out.Kind() != reflect.Pointer {
			return fmt.Errorf("scan target %d is %T, want a pointer", i, target)
		}
		value := reflect.ValueOf(values[i])
		if !value.Type().AssignableTo(out.Elem().Type()) {
			return fmt.Errorf("column %q is %s, cannot assign to %s", userColumns[i], value.Type(), out.Elem().Type())
		}
		out.Elem().Set(value)
	}

	return nil
}

func (r *userRows) Values() ([]any, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.user == nil {
		return nil, pgx.ErrNoRows
	}
	return userValues(r.user), nil
}

func (r *userRows) RawValues() [][]byte { return nil }

func (r *userRows) Conn() *pgx.Conn { return nil }

// compile-time checks that the fakes still satisfy what the cache expects.
var (
	_ Querier  = (*fakeQuerier)(nil)
	_ pgx.Rows = (*userRows)(nil)
)
