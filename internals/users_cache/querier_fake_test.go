package users_cache

import (
	"context"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// call records one Query or Exec, so a test can assert on the statement and the
// arguments the cache sent.
type call struct {
	sql  string
	args []any
}

// fakeQuerier stands in for *pgxpool.Pool. Query always reports "no rows", which is
// the miss path the cache reacts to; Exec records the write and reports success.
type fakeQuerier struct {
	mu      sync.Mutex
	queries []call
	execs   []call
	// execErr, when set, is returned by every Exec.
	execErr error
	// queryErr, when set, is surfaced through the returned rows.
	queryErr error
}

func (f *fakeQuerier) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	f.mu.Lock()
	f.queries = append(f.queries, call{sql: sql, args: args})
	f.mu.Unlock()

	// pgxpool.Pool returns non-nil rows carrying the error rather than a nil interface,
	// and the cache relies on that: it defers rows.Close() before checking anything.
	return &emptyRows{err: f.queryErr}, f.queryErr
}

func (f *fakeQuerier) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.mu.Lock()
	f.execs = append(f.execs, call{sql: sql, args: args})
	f.mu.Unlock()

	if f.execErr != nil {
		return pgconn.CommandTag{}, f.execErr
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

// emptyRows is a pgx.Rows that yields nothing. pgx.CollectOneRow turns that into
// pgx.ErrNoRows, or into err when one is set.
type emptyRows struct {
	err error
}

func (r *emptyRows) Close()                                       {}
func (r *emptyRows) Err() error                                   { return r.err }
func (r *emptyRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *emptyRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *emptyRows) Next() bool                                   { return false }
func (r *emptyRows) Scan(...any) error                            { return r.err }
func (r *emptyRows) Values() ([]any, error)                       { return nil, r.err }
func (r *emptyRows) RawValues() [][]byte                          { return nil }
func (r *emptyRows) Conn() *pgx.Conn                              { return nil }

// compile-time checks that the fakes still satisfy what the cache expects.
var (
	_ Querier  = (*fakeQuerier)(nil)
	_ pgx.Rows = (*emptyRows)(nil)
)
