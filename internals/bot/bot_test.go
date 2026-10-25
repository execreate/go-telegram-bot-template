package bot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/jackc/pgx/v5/pgxpool"
)

// blockingHandler holds every update until release is closed, standing in for a
// handler that is still running when the shutdown signal arrives.
type blockingHandler struct {
	started chan struct{}
	release chan struct{}
}

func (h *blockingHandler) CheckUpdate(*gotgbot.Bot, *ext.Context) bool { return true }

func (h *blockingHandler) HandleUpdate(*gotgbot.Bot, *ext.Context) error {
	close(h.started)
	<-h.release
	return nil
}

func (h *blockingHandler) Name() string { return "blockingHandler" }

// newTestBot wires a MyBot with a real dispatcher and updater. The pool is lazy, so
// it never dials the unreachable address; CleanUp only needs something to close.
func newTestBot(t *testing.T) *MyBot {
	t.Helper()

	pool, err := pgxpool.New(context.Background(), "postgres://user:pass@127.0.0.1:1/db")
	if err != nil {
		t.Fatalf("pgxpool.New() unexpected error: %v", err)
	}

	// A bounded MaxRoutines matters: the limiter channel it creates is what a double
	// Dispatcher.Stop used to close twice.
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: ext.DefaultMaxRoutines})

	return &MyBot{
		DB:         pool,
		bot:        &gotgbot.Bot{Token: "test-token"},
		dispatcher: dispatcher,
		updater:    ext.NewUpdater(dispatcher, nil),
	}
}

func TestCleanUpWithoutRun(t *testing.T) {
	b := newTestBot(t)

	// Shutting down before the webhook ever started must not panic.
	b.CleanUp()
}

func TestCleanUpDrainsInFlightUpdates(t *testing.T) {
	b := newTestBot(t)

	handler := &blockingHandler{started: make(chan struct{}), release: make(chan struct{})}
	b.AddHandler(handler)

	if err := b.updater.AddWebhook(b.bot, "hook", nil); err != nil {
		t.Fatalf("AddWebhook() unexpected error: %v", err)
	}

	// Deliver one update through the webhook handler, as Telegram would.
	webhook := b.updater.GetHandlerFunc("/")
	go webhook(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/hook", strings.NewReader(`{"update_id":1}`)),
	)

	select {
	case <-handler.started:
	case <-time.After(2 * time.Second):
		t.Fatal("the update never reached the handler")
	}

	cleanedUp := make(chan struct{})
	go func() {
		defer close(cleanedUp)
		b.CleanUp()
	}()

	select {
	case <-cleanedUp:
		t.Fatal("CleanUp() returned while an update was still being handled")
	case <-time.After(100 * time.Millisecond):
	}

	close(handler.release)

	select {
	case <-cleanedUp:
	case <-time.After(2 * time.Second):
		t.Fatal("CleanUp() did not return after the in-flight update finished")
	}
}
