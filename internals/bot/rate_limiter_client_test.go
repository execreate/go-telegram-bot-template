package bot

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/execreate/go-telegram-bot-template/internals/limiters"
	"golang.org/x/time/rate"
)

// stubBotClient is the innermost client in the chain. It records what reached it so a
// test can tell a passthrough from a rejection.
type stubBotClient struct {
	gotgbot.BotClient
	calls  atomic.Int64
	method atomic.Value
	err    error
}

func (s *stubBotClient) RequestWithContext(
	_ context.Context,
	_ string,
	method string,
	_ map[string]any,
	_ *gotgbot.RequestOpts,
) (json.RawMessage, error) {
	s.calls.Add(1)
	s.method.Store(method)
	if s.err != nil {
		return nil, s.err
	}
	return json.RawMessage(`{"ok":true}`), nil
}

// newTestRateLimitingClient wires a middleware over a stub. The two pools are
// deliberately lopsided: the group pool admits one request per chat and then makes
// the next wait out a 30s window, while the private pool is effectively unlimited.
// Which pool a request was routed to is therefore observable from the outside — a
// second call with a short deadline succeeds on the private path and fails on the
// group one — without the test reaching into the pools' internals.
func newTestRateLimitingClient(t *testing.T, settings *Settings) (*rateLimitingBotClient, *stubBotClient) {
	t.Helper()

	privateChatLimiters, err := limiters.NewRateLimiterPool[*limiters.TokenBucketRateLimiter, *limiters.TokenBucketRateLimiterConfig](
		limiters.NewTokenBucketRateLimiter,
		&limiters.TokenBucketRateLimiterConfig{Limit: rate.Every(time.Microsecond), Burst: 100},
		time.Hour,
		time.Hour,
	)
	if err != nil {
		t.Fatalf("failed to build the private chat limiter pool: %v", err)
	}

	groupChatLimiters, err := limiters.NewRateLimiterPool[*limiters.SlidingWindowRateLimiter, *limiters.SlidingWindowRateLimiterConfig](
		limiters.NewSlidingWindowRateLimiter,
		&limiters.SlidingWindowRateLimiterConfig{Window: 30 * time.Second, MaxN: 1},
		time.Hour,
		time.Hour,
	)
	if err != nil {
		t.Fatalf("failed to build the group chat limiter pool: %v", err)
	}

	inner := &stubBotClient{}

	return &rateLimitingBotClient{
		BotClient:           inner,
		settings:            settings,
		privateChatLimiters: privateChatLimiters,
		groupChatLimiters:   groupChatLimiters,
	}, inner
}

// sendTwice issues the same request twice. The second carries a short deadline, so it
// fails rather than sleeping if it landed on the throttled group pool.
func sendTwice(t *testing.T, client *rateLimitingBotClient, params map[string]any) (first, second error) {
	t.Helper()

	_, first = client.RequestWithContext(t.Context(), "token", "sendMessage", params, nil)

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	_, second = client.RequestWithContext(ctx, "token", "sendMessage", params, nil)

	return first, second
}

func TestRequestWithContextRoutesRegisteredChatsToTheGroupLimiter(t *testing.T) {
	settings := NewSettings()
	settings.RegisterGroupChat(-1001234567890)

	client, inner := newTestRateLimitingClient(t, settings)

	first, second := sendTwice(t, client, map[string]any{"chat_id": "-1001234567890", "text": "hi"})

	if first != nil {
		t.Fatalf("the first request errored: %v", first)
	}
	if second == nil {
		t.Error("the second request was admitted, want the group limiter to hold it")
	}
	if inner.calls.Load() != 1 {
		t.Errorf("the inner client saw %d calls, want only the first", inner.calls.Load())
	}
}

func TestRequestWithContextRoutesEverythingElseToThePrivateLimiter(t *testing.T) {
	// The chat is not registered as a group, so it takes the private path even though
	// its ID looks like a supergroup's.
	client, inner := newTestRateLimitingClient(t, NewSettings())

	first, second := sendTwice(t, client, map[string]any{"chat_id": "-1001234567890"})

	if first != nil {
		t.Fatalf("the first request errored: %v", first)
	}
	if second != nil {
		t.Errorf("the second request errored: %v — it was routed to the group limiter", second)
	}
	if inner.calls.Load() != 2 {
		t.Errorf("the inner client saw %d calls, want 2", inner.calls.Load())
	}
}

func TestRequestWithContextLimitsEachChatSeparately(t *testing.T) {
	settings := NewSettings()
	settings.RegisterGroupChat(-100111)
	settings.RegisterGroupChat(-100222)

	client, inner := newTestRateLimitingClient(t, settings)

	// One chat spending its budget must not throttle another.
	for _, chatID := range []string{"-100111", "-100222"} {
		if _, err := client.RequestWithContext(
			t.Context(), "token", "sendMessage",
			map[string]any{"chat_id": chatID},
			nil,
		); err != nil {
			t.Fatalf("the request to %s errored: %v", chatID, err)
		}
	}

	if inner.calls.Load() != 2 {
		t.Errorf("the inner client saw %d calls, want 2", inner.calls.Load())
	}
}

func TestRequestWithContextPassesThroughWithoutAChatID(t *testing.T) {
	client, inner := newTestRateLimitingClient(t, NewSettings())

	// getMe and friends are not chat-scoped, so there is nothing to rate limit.
	if _, err := client.RequestWithContext(t.Context(), "token", "getMe", map[string]any{}, nil); err != nil {
		t.Fatalf("RequestWithContext() unexpected error: %v", err)
	}

	if inner.calls.Load() != 1 {
		t.Errorf("the inner client saw %d calls, want 1", inner.calls.Load())
	}
	if got := inner.method.Load(); got != "getMe" {
		t.Errorf("the inner client saw method %v, want getMe", got)
	}
}

func TestRequestWithContextRejectsAnUnparsableChatID(t *testing.T) {
	client, inner := newTestRateLimitingClient(t, NewSettings())

	// A chat addressed by @username has no ID to key a limiter on. The request fails
	// rather than going out unlimited.
	if _, err := client.RequestWithContext(
		t.Context(), "token", "sendMessage",
		map[string]any{"chat_id": "@my_channel"},
		nil,
	); err == nil {
		t.Error("RequestWithContext() with an unparsable chat_id returned no error")
	}

	if inner.calls.Load() != 0 {
		t.Error("the request reached the inner client despite the error")
	}
}

func TestRequestWithContextPassesThroughAnUnexpectedChatIDType(t *testing.T) {
	client, inner := newTestRateLimitingClient(t, NewSettings())

	// Finding 7: an unknown dynamic type cannot be rate limited, but dropping the
	// request would be worse than sending it unlimited. It goes through, logged as a
	// warning rather than silently.
	if _, err := client.RequestWithContext(
		t.Context(), "token", "sendMessage",
		map[string]any{"chat_id": []string{"42"}},
		nil,
	); err != nil {
		t.Fatalf("RequestWithContext() unexpected error: %v", err)
	}

	if inner.calls.Load() != 1 {
		t.Errorf("the inner client saw %d calls, want the request to pass through", inner.calls.Load())
	}
}

func TestRequestWithContextRoutesANumericChatID(t *testing.T) {
	settings := NewSettings()
	settings.RegisterGroupChat(-100999)

	client, inner := newTestRateLimitingClient(t, settings)

	// gotgbot stringifies params today; an int64 has to route the same way if it ever
	// stops doing so.
	first, second := sendTwice(t, client, map[string]any{"chat_id": int64(-100999)})

	if first != nil {
		t.Fatalf("the first request errored: %v", first)
	}
	if second == nil {
		t.Error("the second request was admitted, want the int64 id routed to the group limiter")
	}
	if inner.calls.Load() != 1 {
		t.Errorf("the inner client saw %d calls, want only the first", inner.calls.Load())
	}
}

func TestRequestWithContextPropagatesInnerFailures(t *testing.T) {
	wantErr := errors.New("telegram said no")

	client, inner := newTestRateLimitingClient(t, NewSettings())
	inner.err = wantErr

	_, err := client.RequestWithContext(
		t.Context(), "token", "sendMessage",
		map[string]any{"chat_id": "42"},
		nil,
	)

	if !errors.Is(err, wantErr) {
		t.Errorf("RequestWithContext() error = %v, want %v", err, wantErr)
	}
}

func TestRequestWithContextReturnsWhenTheContextIsCancelled(t *testing.T) {
	settings := NewSettings()
	settings.RegisterGroupChat(-100999)

	client, inner := newTestRateLimitingClient(t, settings)

	params := map[string]any{"chat_id": "-100999"}

	if _, err := client.RequestWithContext(t.Context(), "token", "sendMessage", params, nil); err != nil {
		t.Fatalf("the first RequestWithContext() errored: %v", err)
	}

	// No deadline this time, so the limiter sleeps rather than failing fast — the
	// caller has to come back on cancellation alone.
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		_, err := client.RequestWithContext(ctx, "token", "sendMessage", params, nil)
		done <- err
	}()

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("RequestWithContext() error = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RequestWithContext() kept waiting on the limiter after its context was cancelled")
	}

	if inner.calls.Load() != 1 {
		t.Errorf("the inner client saw %d calls, want only the first", inner.calls.Load())
	}
}
