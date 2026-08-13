//go:build integration

// Package users_cache's integration suite runs the cache against a real PostgreSQL
// instance so the SQL, the NULL handling, and the deferred writes are exercised for
// real rather than against a fake. It is behind the `integration` build tag and needs
// a working Docker daemon:
//
//	go test -tags=integration ./...
package users_cache

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/execreate/go-telegram-bot-template/database/tables"
	"github.com/execreate/go-telegram-bot-template/internals/users_cache/user_container"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// testPool is the connection pool every test in this file shares. Tests clean up
// after themselves rather than each paying for a fresh container.
var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		fmt.Fprintln(os.Stderr, "skipping the integration suite under -short")
		return
	}

	ctx := context.Background()

	container, err := postgres.Run(
		ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("my_db"),
		postgres.WithUsername("user"),
		postgres.WithPassword("pass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(2*time.Minute),
		),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start postgres: %v\n", err)
		os.Exit(1)
	}

	code := run(ctx, m, container)

	if err := testcontainers.TerminateContainer(container); err != nil {
		fmt.Fprintf(os.Stderr, "failed to terminate postgres: %v\n", err)
	}

	os.Exit(code)
}

// run holds everything that needs unwinding before the container is torn down;
// TestMain cannot use defer because it ends in os.Exit.
func run(ctx context.Context, m *testing.M, container *postgres.PostgresContainer) int {
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to build the connection string: %v\n", err)
		return 1
	}

	if err := migrate(dsn); err != nil {
		fmt.Fprintf(os.Stderr, "failed to migrate: %v\n", err)
		return 1
	}

	testPool, err = pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create the pool: %v\n", err)
		return 1
	}
	defer testPool.Close()

	return m.Run()
}

// migrate applies the repository's goose migrations, so the schema under test is the
// same one a fork deploys rather than a copy that can drift.
func migrate(dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("failed to open a database/sql handle: %w", err)
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set the goose dialect: %w", err)
	}
	goose.SetLogger(goose.NopLogger())

	dir, err := filepath.Abs(filepath.Join("..", "..", "database", "migrations", "postgres"))
	if err != nil {
		return fmt.Errorf("failed to resolve the migrations directory: %w", err)
	}

	if err := goose.Up(db, dir); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	return nil
}

// newIntegrationCache returns a cache backed by the real pool, with the users table
// emptied first so tests do not see each other's rows.
func newIntegrationCache(t *testing.T, staleThreshold time.Duration) *TgUsersCache {
	t.Helper()

	truncateUsers(t)

	return &TgUsersCache{
		dbPool:         testPool,
		users:          make(map[int64]*user_container.TgUserContainer),
		staleThreshold: staleThreshold,
	}
}

func truncateUsers(t *testing.T) {
	t.Helper()

	if _, err := testPool.Exec(t.Context(), "truncate table telegram_users"); err != nil {
		t.Fatalf("failed to truncate telegram_users: %v", err)
	}
}

// readUser reads a row straight from the database, bypassing the cache.
func readUser(t *testing.T, id int64) tables.TelegramUser {
	t.Helper()

	var user tables.TelegramUser
	err := testPool.QueryRow(
		t.Context(),
		`select id, created_at, updated_at, deleted_at, first_name, last_name, username,
		        language_code, is_admin, is_owner,
		        accepted_terms_and_conditions_on, accepted_terms_and_conditions_version
		 from telegram_users where id = $1`,
		id,
	).Scan(
		&user.ID, &user.CreatedAt, &user.UpdatedAt, &user.DeletedAt,
		&user.FirstName, &user.LastName, &user.Username,
		&user.LanguageCode, &user.IsAdmin, &user.IsOwner,
		&user.AcceptedTermsAndConditionsOn, &user.AcceptedTermsAndConditionsVersion,
	)
	if err != nil {
		t.Fatalf("failed to read user %d: %v", id, err)
	}

	return user
}

// insertUser writes a row directly, for the cases that start from existing data.
func insertUser(t *testing.T, user tables.TelegramUser) {
	t.Helper()

	now := time.Now()
	_, err := testPool.Exec(
		t.Context(),
		`insert into telegram_users (
			id, created_at, updated_at, deleted_at, first_name, last_name, username,
			language_code, is_admin, is_owner
		) values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		user.ID, now, now, user.DeletedAt, user.FirstName, user.LastName, user.Username,
		user.LanguageCode, user.IsAdmin, user.IsOwner,
	)
	if err != nil {
		t.Fatalf("failed to insert user %d: %v", user.ID, err)
	}
}

func TestIntegrationGetInsertsOnMiss(t *testing.T) {
	cache := newIntegrationCache(t, 4*24*time.Hour)

	returned, err := cache.Get(&gotgbot.User{
		Id:           42,
		FirstName:    "Ada",
		LastName:     "Lovelace",
		Username:     "ada",
		LanguageCode: "en",
	})
	if err != nil {
		t.Fatalf("Get() unexpected error: %v", err)
	}

	stored := readUser(t, 42)

	if stored.FirstName != "Ada" || stored.LastName != "Lovelace" || stored.LanguageCode != "en" {
		t.Errorf("stored row = %+v, want the details from the update", stored)
	}
	if !stored.Username.Valid || stored.Username.String != "ada" {
		t.Errorf("stored username = %+v, want a valid \"ada\"", stored.Username)
	}
	if stored.DeletedAt.Valid {
		t.Error("the new row was inserted already soft-deleted")
	}
	if stored.IsAdmin || stored.IsOwner {
		t.Error("the new row was inserted with a role")
	}
	if stored.AcceptedTermsAndConditionsOn.Valid || stored.AcceptedTermsAndConditionsVersion.Valid {
		t.Error("the new row was inserted having already accepted the terms")
	}
	if returned.ID != stored.ID || returned.FirstName != stored.FirstName {
		t.Errorf("Get() returned %+v, which does not match the stored row %+v", returned, stored)
	}
}

func TestIntegrationGetInsertsNullUsername(t *testing.T) {
	cache := newIntegrationCache(t, 4*24*time.Hour)

	// Telegram sends an empty username for users who have none. It has to reach the
	// column as NULL: the unique index would otherwise reject the second such user.
	for _, id := range []int64{42, 43} {
		if _, err := cache.Get(&gotgbot.User{Id: id, FirstName: "Ada", LanguageCode: "en"}); err != nil {
			t.Fatalf("Get() for user %d unexpected error: %v", id, err)
		}

		if stored := readUser(t, id); stored.Username.Valid {
			t.Errorf("user %d stored username = %+v, want NULL", id, stored.Username)
		}
	}
}

func TestIntegrationDeferredUpdateReachesTheDatabase(t *testing.T) {
	cache := newIntegrationCache(t, 4*24*time.Hour)

	if _, err := cache.Get(&gotgbot.User{
		Id: 42, FirstName: "Ada", Username: "ada", LanguageCode: "en",
	}); err != nil {
		t.Fatalf("Get() unexpected error: %v", err)
	}

	// A rename: both usernames are non-empty, the case that used to be dropped.
	if _, err := cache.Get(&gotgbot.User{
		Id: 42, FirstName: "Ada", LastName: "Lovelace", Username: "ada_l", LanguageCode: "de",
	}); err != nil {
		t.Fatalf("Get() unexpected error: %v", err)
	}

	// The write is fired off in a goroutine, so there is nothing to synchronise on.
	stored := waitForUser(t, 42, func(u tables.TelegramUser) bool {
		return u.Username.Valid && u.Username.String == "ada_l"
	})

	if stored.LastName != "Lovelace" {
		t.Errorf("stored last name = %q, want %q", stored.LastName, "Lovelace")
	}
	if stored.LanguageCode != "de" {
		t.Errorf("stored language = %q, want %q", stored.LanguageCode, "de")
	}
}

func TestIntegrationDeferredUpdateClearsARemovedUsername(t *testing.T) {
	cache := newIntegrationCache(t, 4*24*time.Hour)

	if _, err := cache.Get(&gotgbot.User{
		Id: 42, FirstName: "Ada", Username: "ada", LanguageCode: "en",
	}); err != nil {
		t.Fatalf("Get() unexpected error: %v", err)
	}

	// The user dropped their username; the column has to end up NULL, not "".
	if _, err := cache.Get(&gotgbot.User{Id: 42, FirstName: "Ada", LanguageCode: "en"}); err != nil {
		t.Fatalf("Get() unexpected error: %v", err)
	}

	waitForUser(t, 42, func(u tables.TelegramUser) bool { return !u.Username.Valid })
}

func TestIntegrationGetDoesNotWriteWhenNothingChanged(t *testing.T) {
	cache := newIntegrationCache(t, 4*24*time.Hour)

	effectiveUser := &gotgbot.User{Id: 42, FirstName: "Ada", Username: "ada", LanguageCode: "en"}

	if _, err := cache.Get(effectiveUser); err != nil {
		t.Fatalf("Get() unexpected error: %v", err)
	}
	before := readUser(t, 42)

	// Rename out of band. An unchanged Get must not fire a deferred write, so this
	// value survives — if one fired, it would be overwritten with "ada".
	if _, err := testPool.Exec(
		t.Context(),
		"update telegram_users set username = 'touched_out_of_band' where id = $1",
		42,
	); err != nil {
		t.Fatalf("failed to update out of band: %v", err)
	}

	if _, err := cache.Get(effectiveUser); err != nil {
		t.Fatalf("Get() unexpected error: %v", err)
	}

	// Give any (incorrect) deferred write time to land before checking.
	time.Sleep(500 * time.Millisecond)

	after := readUser(t, 42)
	if after.Username.String != "touched_out_of_band" {
		t.Errorf("stored username = %q, want the out-of-band value — an unchanged Get wrote anyway", after.Username.String)
	}
	if !after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Error("an unchanged Get touched the row")
	}
}

func TestIntegrationUserHasAcceptedTermsAndConditions(t *testing.T) {
	cache := newIntegrationCache(t, 4*24*time.Hour)

	if _, err := cache.Get(&gotgbot.User{Id: 42, FirstName: "Ada", LanguageCode: "en"}); err != nil {
		t.Fatalf("Get() unexpected error: %v", err)
	}

	if err := cache.UserHasAcceptedTermsAndConditions(42, "v2.0.0"); err != nil {
		t.Fatalf("UserHasAcceptedTermsAndConditions() unexpected error: %v", err)
	}

	stored := readUser(t, 42)
	if !stored.AcceptedTermsAndConditionsOn.Valid {
		t.Error("the accepted-on timestamp was not written")
	}
	if !stored.AcceptedTermsAndConditionsVersion.Valid || stored.AcceptedTermsAndConditionsVersion.String != "v2.0.0" {
		t.Errorf("stored version = %+v, want v2.0.0", stored.AcceptedTermsAndConditionsVersion)
	}

	// The cached container has to agree, or the T&C gate keeps asking until the entry
	// goes stale even though the row says otherwise.
	cache.mu.RLock()
	container := cache.users[42]
	cache.mu.RUnlock()

	if container.GetRaw().MustAcceptTermsAndConditions("v2.0.0") {
		t.Error("the cached user still needs to accept the terms it just accepted")
	}
}

func TestIntegrationGetByUsername(t *testing.T) {
	cache := newIntegrationCache(t, 4*24*time.Hour)

	insertUser(t, tables.TelegramUser{
		SoftDeleteModel: tables.SoftDeleteModel{ID: 42},
		FirstName:       "Ada",
		Username:        sql.NullString{String: "ada", Valid: true},
		LanguageCode:    "en",
	})

	t.Run("strips the @ prefix", func(t *testing.T) {
		user, err := cache.GetByUsername("@ada")
		if err != nil {
			t.Fatalf("GetByUsername() unexpected error: %v", err)
		}
		if user.ID != 42 {
			t.Errorf("GetByUsername() = %+v, want user 42", user)
		}
	})

	t.Run("works without the prefix", func(t *testing.T) {
		user, err := cache.GetByUsername("ada")
		if err != nil {
			t.Fatalf("GetByUsername() unexpected error: %v", err)
		}
		if user.ID != 42 {
			t.Errorf("GetByUsername() = %+v, want user 42", user)
		}
	})

	t.Run("honours deleted_at is null", func(t *testing.T) {
		insertUser(t, tables.TelegramUser{
			SoftDeleteModel: tables.SoftDeleteModel{
				ID:        43,
				DeletedAt: sql.NullTime{Time: time.Now(), Valid: true},
			},
			FirstName:    "Grace",
			Username:     sql.NullString{String: "grace", Valid: true},
			LanguageCode: "en",
		})

		if user, err := cache.GetByUsername("grace"); err == nil {
			t.Errorf("GetByUsername() returned the soft-deleted user %+v", user)
		}
	})

	t.Run("unknown username", func(t *testing.T) {
		if user, err := cache.GetByUsername("nobody"); err == nil {
			t.Errorf("GetByUsername() returned %+v for an unknown username", user)
		}
	})
}

func TestIntegrationGetByID(t *testing.T) {
	cache := newIntegrationCache(t, 4*24*time.Hour)

	insertUser(t, tables.TelegramUser{
		SoftDeleteModel: tables.SoftDeleteModel{ID: 42},
		FirstName:       "Ada",
		Username:        sql.NullString{String: "ada", Valid: true},
		LanguageCode:    "en",
		IsAdmin:         true,
	})

	user, err := cache.GetByID(42)
	if err != nil {
		t.Fatalf("GetByID() unexpected error: %v", err)
	}
	if !user.IsAdmin || user.FirstName != "Ada" {
		t.Errorf("GetByID() = %+v, want the stored row", user)
	}

	insertUser(t, tables.TelegramUser{
		SoftDeleteModel: tables.SoftDeleteModel{
			ID:        43,
			DeletedAt: sql.NullTime{Time: time.Now(), Valid: true},
		},
		FirstName:    "Grace",
		LanguageCode: "en",
	})

	if user, err := cache.GetByID(43); err == nil {
		t.Errorf("GetByID() returned the soft-deleted user %+v", user)
	}
}

func TestIntegrationCleanUpStaleUsers(t *testing.T) {
	cache := newIntegrationCache(t, time.Hour)

	for _, id := range []int64{42, 43} {
		if _, err := cache.Get(&gotgbot.User{Id: id, FirstName: "Ada", LanguageCode: "en"}); err != nil {
			t.Fatalf("Get() for user %d unexpected error: %v", id, err)
		}
	}

	// Both entries are fresh, so nothing is evicted.
	cache.cleanUpStaleUsers()
	cache.mu.RLock()
	remaining := len(cache.users)
	cache.mu.RUnlock()
	if remaining != 2 {
		t.Fatalf("cache holds %d users after a no-op cleanup, want 2", remaining)
	}

	// Age one of them past the threshold by touching the other.
	if _, err := cache.Get(&gotgbot.User{Id: 43, FirstName: "Ada", LanguageCode: "en"}); err != nil {
		t.Fatalf("Get() unexpected error: %v", err)
	}
	cache.staleThreshold = 0

	cache.cleanUpStaleUsers()
	cache.mu.RLock()
	remaining = len(cache.users)
	cache.mu.RUnlock()
	if remaining != 0 {
		t.Errorf("cache holds %d users after cleanup, want 0", remaining)
	}

	// Eviction is cache-only: the rows are still there, so the next lookup refills
	// from the database rather than losing the user.
	if _, err := cache.GetByID(42); err != nil {
		t.Errorf("GetByID() after eviction failed, the cleanup deleted the row: %v", err)
	}
}

// waitForUser polls the row until cond holds, since the deferred update runs in its
// own goroutine with nothing to synchronise on.
func waitForUser(t *testing.T, id int64, cond func(tables.TelegramUser) bool) tables.TelegramUser {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for {
		user := readUser(t, id)
		if cond(user) {
			return user
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for the deferred write to land; last read %+v", user)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
