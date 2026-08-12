package users_cache

import (
	"database/sql"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/execreate/go-telegram-bot-template/database/tables"
	"github.com/execreate/go-telegram-bot-template/internals/users_cache/user_container"
)

// newTestCache builds a cache backed by db. Pass nil when the test only exercises the
// in-memory map.
func newTestCache(db Querier) *TgUsersCache {
	return &TgUsersCache{
		dbPool:         db,
		users:          make(map[int64]*user_container.TgUserContainer),
		staleThreshold: 4 * 24 * time.Hour,
	}
}

func containerFor(id int64, firstName string) *user_container.TgUserContainer {
	return user_container.NewTelegramUserContainer(&tables.TelegramUser{
		SoftDeleteModel: tables.SoftDeleteModel{ID: id},
		FirstName:       firstName,
	})
}

func TestCacheUserInsertsOnMiss(t *testing.T) {
	cache := newTestCache(nil)
	container := containerFor(1, "Ada")

	if got := cache.cacheUser(1, container); got != container {
		t.Error("cacheUser() did not return the container it inserted")
	}

	cache.mu.RLock()
	defer cache.mu.RUnlock()
	if cache.users[1] != container {
		t.Error("cacheUser() did not store the container in the cache")
	}
}

func TestCacheUserKeepsTheExistingEntry(t *testing.T) {
	cache := newTestCache(nil)
	first := containerFor(1, "Ada")
	second := containerFor(1, "Grace")

	cache.cacheUser(1, first)

	// A second miss for the same user must not replace a possibly fresher entry —
	// it may have been updated in place since it landed.
	if got := cache.cacheUser(1, second); got != first {
		t.Error("cacheUser() returned the new container, want the one already cached")
	}

	cache.mu.RLock()
	defer cache.mu.RUnlock()
	if cache.users[1] != first {
		t.Error("cacheUser() overwrote the cached container")
	}
}

func TestCacheUserConcurrentMisses(t *testing.T) {
	const goroutines = 50

	cache := newTestCache(nil)

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results = make([]*user_container.TgUserContainer, 0, goroutines)
	)

	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			got := cache.cacheUser(1, containerFor(1, "user"))
			mu.Lock()
			results = append(results, got)
			mu.Unlock()
		}()
	}
	wg.Wait()

	cache.mu.RLock()
	cached := cache.users[1]
	total := len(cache.users)
	cache.mu.RUnlock()

	if total != 1 {
		t.Errorf("cache holds %d users, want 1", total)
	}
	for _, got := range results {
		if got != cached {
			t.Fatal("cacheUser() returned a container that is not the cached one")
		}
	}
}

func TestGetInsertsOnMiss(t *testing.T) {
	db := &fakeQuerier{}
	cache := newTestCache(db)

	effectiveUser := &gotgbot.User{
		Id:           42,
		FirstName:    "Ada",
		LastName:     "Lovelace",
		Username:     "ada",
		LanguageCode: "en",
	}

	user, err := cache.Get(effectiveUser)
	if err != nil {
		t.Fatalf("Get() unexpected error: %v", err)
	}

	if user.ID != 42 || user.FirstName != "Ada" || user.LastName != "Lovelace" {
		t.Errorf("Get() = %+v, want the details from the update", user)
	}
	if !user.Username.Valid || user.Username.String != "ada" {
		t.Errorf("Get() username = %+v, want a valid \"ada\"", user.Username)
	}

	execs := db.recordedExecs()
	if len(execs) != 1 {
		t.Fatalf("Get() ran %d writes, want 1 insert", len(execs))
	}
	if !strings.Contains(execs[0].sql, "insert into telegram_users") {
		t.Errorf("Get() ran %q, want an insert", execs[0].sql)
	}
	if got := execs[0].args[0]; got != int64(42) {
		t.Errorf("insert bound id %v, want 42", got)
	}
	if got, want := execs[0].args[5], (sql.NullString{String: "ada", Valid: true}); got != want {
		t.Errorf("insert bound username %+v, want %+v", got, want)
	}

	// Finding 9: the entry must be cached before Get returns, since
	// UserHasAcceptedTermsAndConditions looks it up straight afterwards.
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	if _, ok := cache.users[42]; !ok {
		t.Error("Get() returned before the user landed in the cache")
	}
}

func TestGetInsertsNullUsernameWhenTelegramSendsNone(t *testing.T) {
	db := &fakeQuerier{}
	cache := newTestCache(db)

	user, err := cache.Get(&gotgbot.User{Id: 7, FirstName: "Ada", LanguageCode: "en"})
	if err != nil {
		t.Fatalf("Get() unexpected error: %v", err)
	}

	if user.Username.Valid {
		t.Errorf("Get() username = %+v, want it to be NULL", user.Username)
	}

	execs := db.recordedExecs()
	if len(execs) != 1 {
		t.Fatalf("Get() ran %d writes, want 1 insert", len(execs))
	}
	if got, want := execs[0].args[5], (sql.NullString{}); got != want {
		t.Errorf("insert bound username %+v, want %+v", got, want)
	}
}

func TestGetPropagatesInsertFailures(t *testing.T) {
	db := &fakeQuerier{execErr: errBoom}
	cache := newTestCache(db)

	if _, err := cache.Get(&gotgbot.User{Id: 42, FirstName: "Ada"}); err == nil {
		t.Fatal("Get() with a failing insert returned no error")
	}

	cache.mu.RLock()
	defer cache.mu.RUnlock()
	if len(cache.users) != 0 {
		t.Error("Get() cached a user whose insert failed")
	}
}

func TestGetServesFromCacheAndDefersTheUpdate(t *testing.T) {
	db := &fakeQuerier{}
	cache := newTestCache(db)

	cached := &tables.TelegramUser{
		SoftDeleteModel: tables.SoftDeleteModel{ID: 42},
		FirstName:       "Ada",
		Username:        sql.NullString{String: "ada", Valid: true},
		LanguageCode:    "en",
	}
	cache.cacheUser(42, user_container.NewTelegramUserContainer(cached))

	// Same details: served from memory, no query and no write.
	user, err := cache.Get(&gotgbot.User{Id: 42, FirstName: "Ada", Username: "ada", LanguageCode: "en"})
	if err != nil {
		t.Fatalf("Get() unexpected error: %v", err)
	}
	if user.FirstName != "Ada" {
		t.Errorf("Get() = %+v, want the cached user", user)
	}
	if queries := db.recordedQueries(); len(queries) != 0 {
		t.Errorf("Get() hit the database %d times for a cached user, want 0", len(queries))
	}
	if execs := db.recordedExecs(); len(execs) != 0 {
		t.Errorf("Get() wrote %d times for an unchanged user, want 0", len(execs))
	}

	// Renamed: the deferred update fires. It runs in a goroutine, so poll for it.
	if _, err := cache.Get(&gotgbot.User{Id: 42, FirstName: "Ada", Username: "ada_l", LanguageCode: "en"}); err != nil {
		t.Fatalf("Get() unexpected error: %v", err)
	}

	execs := waitForExecs(t, db, 1)
	if !strings.Contains(execs[0].sql, "update telegram_users") {
		t.Errorf("deferred write ran %q, want an update", execs[0].sql)
	}
	if got, want := execs[0].args[2], (sql.NullString{String: "ada_l", Valid: true}); got != want {
		t.Errorf("update bound username %+v, want %+v", got, want)
	}
}

func TestUserHasAcceptedTermsAndConditions(t *testing.T) {
	db := &fakeQuerier{}
	cache := newTestCache(db)

	cache.cacheUser(42, user_container.NewTelegramUserContainer(&tables.TelegramUser{
		SoftDeleteModel: tables.SoftDeleteModel{ID: 42},
		FirstName:       "Ada",
	}))

	if err := cache.UserHasAcceptedTermsAndConditions(42, "v2.0.0"); err != nil {
		t.Fatalf("UserHasAcceptedTermsAndConditions() unexpected error: %v", err)
	}

	execs := db.recordedExecs()
	if len(execs) != 1 {
		t.Fatalf("ran %d writes, want 1", len(execs))
	}
	if !strings.Contains(execs[0].sql, "accepted_terms_and_conditions_version") {
		t.Errorf("ran %q, want the acceptance update", execs[0].sql)
	}
	if got := execs[0].args[1]; got != "v2.0.0" {
		t.Errorf("update bound version %v, want v2.0.0", got)
	}

	// The cached container must reflect the acceptance too, or the T&C gate keeps
	// asking until the entry goes stale.
	cache.mu.RLock()
	container := cache.users[42]
	cache.mu.RUnlock()
	if container.GetRaw().MustAcceptTermsAndConditions("v2.0.0") {
		t.Error("the cached user still needs to accept the terms it just accepted")
	}
}

func TestUserHasAcceptedTermsAndConditionsForAnUncachedUser(t *testing.T) {
	db := &fakeQuerier{}
	cache := newTestCache(db)

	if err := cache.UserHasAcceptedTermsAndConditions(42, "v2.0.0"); err == nil {
		t.Fatal("UserHasAcceptedTermsAndConditions() for an uncached user returned no error")
	}
	if execs := db.recordedExecs(); len(execs) != 0 {
		t.Errorf("wrote %d times for an uncached user, want 0", len(execs))
	}
}

func TestCleanUpStaleUsers(t *testing.T) {
	cache := newTestCache(nil)
	cache.staleThreshold = time.Hour

	cache.cacheUser(1, containerFor(1, "fresh"))
	cache.cacheUser(2, containerFor(2, "also fresh"))

	cache.cleanUpStaleUsers()
	cache.mu.RLock()
	remaining := len(cache.users)
	cache.mu.RUnlock()
	if remaining != 2 {
		t.Fatalf("cache holds %d users after a no-op cleanup, want 2", remaining)
	}

	cache.staleThreshold = 0
	cache.cleanUpStaleUsers()
	cache.mu.RLock()
	remaining = len(cache.users)
	cache.mu.RUnlock()
	if remaining != 0 {
		t.Errorf("cache holds %d users after cleanup, want 0", remaining)
	}
}

// waitForExecs polls until db has recorded at least n writes. The deferred update runs
// in its own goroutine, so there is nothing to synchronise on.
func waitForExecs(t *testing.T, db *fakeQuerier, n int) []call {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for {
		if execs := db.recordedExecs(); len(execs) >= n {
			return execs
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %d writes, got %d", n, len(db.recordedExecs()))
		}
		time.Sleep(5 * time.Millisecond)
	}
}

var errBoom = errors.New("boom")
