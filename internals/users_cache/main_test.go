package users_cache

import (
	"sync"
	"testing"

	"github.com/execreate/go-telegram-bot-template/database/tables"
	"github.com/execreate/go-telegram-bot-template/internals/users_cache/user_container"
)

// newTestCache builds a cache with no database behind it. Only cacheUser and the
// cleanup pass are exercised here; anything that queries needs the integration suite.
func newTestCache() *TgUsersCache {
	return &TgUsersCache{
		users: make(map[int64]*user_container.TgUserContainer),
	}
}

func containerFor(id int64, firstName string) *user_container.TgUserContainer {
	return user_container.NewTelegramUserContainer(&tables.TelegramUser{
		SoftDeleteModel: tables.SoftDeleteModel{ID: id},
		FirstName:       firstName,
	})
}

func TestCacheUserInsertsOnMiss(t *testing.T) {
	cache := newTestCache()
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
	cache := newTestCache()
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

	cache := newTestCache()

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
