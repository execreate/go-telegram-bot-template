package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/execreate/go-telegram-bot-template/locale"
	"github.com/spf13/viper"
)

// apiCall is one request the handler made to the Telegram API.
type apiCall struct {
	method string
	params map[string]string
}

// fakeTelegram is a stand-in for the Bot API. Handlers talk to it over HTTP through a
// real gotgbot client, so the request they build is exercised end to end.
type fakeTelegram struct {
	server *httptest.Server

	mu    sync.Mutex
	calls []apiCall
	// responses maps a method to the JSON `result` it answers with. A method with no
	// entry gets a generic success, or an error if it is listed in failures.
	responses map[string]string
	failures  map[string]string
}

func newFakeTelegram(t *testing.T) *fakeTelegram {
	t.Helper()

	fake := &fakeTelegram{
		responses: map[string]string{},
		failures:  map[string]string{},
	}

	fake.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// gotgbot posts to /bot<token>/<method>.
		method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]

		// gotgbot always encodes as multipart, so that a request carrying a file looks
		// the same as one that does not.
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Errorf("failed to parse the request form: %v", err)
		}
		params := make(map[string]string, len(r.MultipartForm.Value))
		for key, values := range r.MultipartForm.Value {
			if len(values) > 0 {
				params[key] = values[0]
			}
		}

		fake.mu.Lock()
		fake.calls = append(fake.calls, apiCall{method: method, params: params})
		description, failed := fake.failures[method]
		result, ok := fake.responses[method]
		fake.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		if failed {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"ok":false,"error_code":400,"description":%q}`, description)
			return
		}
		if !ok {
			result = `{"message_id":1,"date":0,"chat":{"id":1,"type":"private"}}`
		}
		fmt.Fprintf(w, `{"ok":true,"result":%s}`, result)
	}))
	t.Cleanup(fake.server.Close)

	return fake
}

// bot returns a gotgbot client pointed at the fake, with no network calls on startup.
func (f *fakeTelegram) bot(t *testing.T) *gotgbot.Bot {
	t.Helper()

	b, err := gotgbot.NewBot("123:TEST", &gotgbot.BotOpts{
		BotClient: &gotgbot.BaseBotClient{
			DefaultRequestOpts: &gotgbot.RequestOpts{APIURL: f.server.URL},
		},
		DisableTokenCheck: true,
	})
	if err != nil {
		t.Fatalf("failed to build the bot: %v", err)
	}
	b.User = gotgbot.User{Id: 1, IsBot: true, Username: "my_test_bot"}

	return b
}

func (f *fakeTelegram) recorded() []apiCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]apiCall(nil), f.calls...)
}

// only returns the single call made to method, failing if there was not exactly one.
func (f *fakeTelegram) only(t *testing.T, method string) apiCall {
	t.Helper()

	var found []apiCall
	for _, call := range f.recorded() {
		if call.method == method {
			found = append(found, call)
		}
	}
	if len(found) != 1 {
		t.Fatalf("the handler made %d %s calls, want 1 (all calls: %+v)", len(found), method, f.recorded())
	}

	return found[0]
}

func (f *fakeTelegram) failMethod(method, description string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failures[method] = description
}

func (f *fakeTelegram) respondWith(method, result string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.responses[method] = result
}

// useLocaleFixture points the locale package at a small set of translations.
func useLocaleFixture(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(dir, "en.yaml"),
		[]byte("hello: |-\n    Hello, %s!\n"),
		0o600,
	); err != nil {
		t.Fatalf("failed to write the fixture: %v", err)
	}

	locale.SetPath(dir)
}

func texts(t *testing.T) *viper.Viper {
	t.Helper()

	useLocaleFixture(t)

	resolved, err := locale.GetTextTranslations("en")
	if err != nil {
		t.Fatalf("failed to resolve texts: %v", err)
	}

	return resolved
}

// contextFor builds the context the dispatcher would hand a handler for a message.
func contextFor(b *gotgbot.Bot, msg *gotgbot.Message, data map[string]any) *ext.Context {
	if data == nil {
		data = map[string]any{}
	}
	return ext.NewContext(b, &gotgbot.Update{Message: msg}, data)
}

func privateMessage(text string) *gotgbot.Message {
	return &gotgbot.Message{
		MessageId: 10,
		Text:      text,
		Chat:      gotgbot.Chat{Id: 42, Type: "private"},
		From:      &gotgbot.User{Id: 42, FirstName: "Ada"},
	}
}

func TestHello(t *testing.T) {
	tests := []struct {
		name string
		msg  *gotgbot.Message
		// wantReply is empty when the handler should stay silent.
		wantReply string
	}{
		{
			name:      "private chat greets the user by first name",
			msg:       privateMessage("/start"),
			wantReply: "Hello, Ada!",
		},
		{
			name: "group chat is ignored",
			msg: &gotgbot.Message{
				MessageId: 10,
				Text:      "/start",
				Chat:      gotgbot.Chat{Id: -100123, Type: "supergroup"},
				From:      &gotgbot.User{Id: 42, FirstName: "Ada"},
			},
		},
		{
			name: "channel post is ignored",
			msg: &gotgbot.Message{
				MessageId: 10,
				Text:      "/start",
				Chat:      gotgbot.Chat{Id: -100123, Type: "channel"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeTelegram(t)
			b := fake.bot(t)

			ctx := contextFor(b, tt.msg, map[string]any{"texts": texts(t)})

			if err := Hello(b, ctx); err != nil {
				t.Fatalf("Hello() unexpected error: %v", err)
			}

			if tt.wantReply == "" {
				if calls := fake.recorded(); len(calls) != 0 {
					t.Errorf("the handler called the API %+v, want silence outside private chats", calls)
				}
				return
			}

			call := fake.only(t, "sendMessage")
			if call.params["text"] != tt.wantReply {
				t.Errorf("reply = %q, want %q", call.params["text"], tt.wantReply)
			}
			if call.params["chat_id"] != "42" {
				t.Errorf("reply went to chat %q, want 42", call.params["chat_id"])
			}
		})
	}
}

func TestHelloFallsBackToStranger(t *testing.T) {
	fake := newFakeTelegram(t)
	b := fake.bot(t)

	// A private message with no sender: rare, but the handler has a branch for it.
	msg := privateMessage("/start")
	msg.From = nil

	ctx := contextFor(b, msg, map[string]any{"texts": texts(t)})

	if err := Hello(b, ctx); err != nil {
		t.Fatalf("Hello() unexpected error: %v", err)
	}

	if got := fake.only(t, "sendMessage").params["text"]; got != "Hello, stranger!" {
		t.Errorf("reply = %q, want the stranger fallback", got)
	}
}

func TestHelloPropagatesSendFailures(t *testing.T) {
	fake := newFakeTelegram(t)
	fake.failMethod("sendMessage", "Bad Request: chat not found")
	b := fake.bot(t)

	ctx := contextFor(b, privateMessage("/start"), map[string]any{"texts": texts(t)})

	if err := Hello(b, ctx); err == nil {
		t.Error("Hello() returned no error when the send failed")
	}
}

func TestMyIDEscapesTheChatTitle(t *testing.T) {
	fake := newFakeTelegram(t)
	// Finding 3: a title with MarkdownV2 specials must be escaped, or Telegram rejects
	// the whole message and /my_id fails in exactly the groups it is most useful in.
	fake.respondWith("getChat", `{"id":-100123,"type":"supergroup","title":"Team (EU)_v2","linked_chat_id":-100999}`)
	b := fake.bot(t)

	msg := &gotgbot.Message{
		MessageId: 10,
		Text:      "/my_id",
		Chat:      gotgbot.Chat{Id: -100123, Type: "supergroup"},
		From:      &gotgbot.User{Id: 42, FirstName: "Ada"},
	}

	if err := MyID(b, contextFor(b, msg, nil)); err != nil {
		t.Fatalf("MyID() unexpected error: %v", err)
	}

	call := fake.only(t, "sendMessage")

	if call.params["parse_mode"] != gotgbot.ParseModeMarkdownV2 {
		t.Errorf("parse mode = %q, want MarkdownV2", call.params["parse_mode"])
	}
	if !strings.Contains(call.params["text"], `Team \(EU\)\_v2`) {
		t.Errorf("text %q does not contain the escaped title", call.params["text"])
	}
	if strings.Contains(call.params["text"], "Team (EU)_v2") {
		t.Errorf("text %q contains the raw, unescaped title", call.params["text"])
	}
	for _, want := range []string{"User ID: `42`", "Chat ID: `-100123`", "Linked Chat ID: `-100999`"} {
		if !strings.Contains(call.params["text"], want) {
			t.Errorf("text %q is missing %q", call.params["text"], want)
		}
	}
}

func TestMyIDFallsBackWhenGetChatFails(t *testing.T) {
	fake := newFakeTelegram(t)
	fake.failMethod("getChat", "Bad Request: chat not found")
	b := fake.bot(t)

	msg := &gotgbot.Message{
		MessageId: 10,
		Text:      "/my_id",
		Chat:      gotgbot.Chat{Id: -100123, Type: "supergroup", Title: "Fallback (title)"},
		From:      &gotgbot.User{Id: 42, FirstName: "Ada"},
	}

	// getChat failing is not fatal: the handler answers from the update's own chat.
	if err := MyID(b, contextFor(b, msg, nil)); err != nil {
		t.Fatalf("MyID() unexpected error: %v", err)
	}

	call := fake.only(t, "sendMessage")
	if !strings.Contains(call.params["text"], `Fallback \(title\)`) {
		t.Errorf("text %q does not carry the escaped title from the update", call.params["text"])
	}
	if !strings.Contains(call.params["text"], "Chat ID: `-100123`") {
		t.Errorf("text %q does not carry the chat id from the update", call.params["text"])
	}
	// There is no linked chat to report on the fallback path.
	if !strings.Contains(call.params["text"], "Linked Chat ID: `0`") {
		t.Errorf("text %q should report a zero linked chat id", call.params["text"])
	}
}

func TestMyIDWithoutAnEffectiveChat(t *testing.T) {
	fake := newFakeTelegram(t)
	b := fake.bot(t)

	// A poll answer has a user but no chat; the handler still answers with the user id.
	ctx := ext.NewContext(b, &gotgbot.Update{
		PollAnswer: &gotgbot.PollAnswer{PollId: "1", User: &gotgbot.User{Id: 42}},
	}, map[string]any{})
	ctx.EffectiveMessage = privateMessage("/my_id")

	if err := MyID(b, ctx); err != nil {
		t.Fatalf("MyID() unexpected error: %v", err)
	}

	call := fake.only(t, "sendMessage")
	if !strings.Contains(call.params["text"], "User ID: `42`") {
		t.Errorf("text %q does not carry the user id", call.params["text"])
	}
	if len(fake.recorded()) != 1 {
		t.Errorf("the handler called getChat with no effective chat: %+v", fake.recorded())
	}
}

// decodeReplyMarkup pulls the inline keyboard back out of a recorded sendMessage.
func decodeReplyMarkup(t *testing.T, call apiCall) gotgbot.InlineKeyboardMarkup {
	t.Helper()

	var markup gotgbot.InlineKeyboardMarkup
	if err := json.Unmarshal([]byte(call.params["reply_markup"]), &markup); err != nil {
		t.Fatalf("failed to decode reply_markup %q: %v", call.params["reply_markup"], err)
	}

	return markup
}
