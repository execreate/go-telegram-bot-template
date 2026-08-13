package gin_server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/execreate/go-telegram-bot-template/locale"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

const testToken = "123456:TEST-TOKEN"

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

// signInitData produces the `hash` Telegram would attach to these values, so a test
// can build a payload that genuinely passes validation. The scheme is the one from
// Telegram's WebApp docs: HMAC-SHA256 over the sorted "key=value" pairs joined by
// newlines, keyed by HMAC-SHA256("<token>") under the literal key "WebAppData".
func signInitData(t *testing.T, values url.Values) url.Values {
	t.Helper()

	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(testToken))

	pairs := make([]string, 0, len(values))
	for key, value := range values {
		if key == "hash" {
			continue
		}
		pairs = append(pairs, key+"="+value[0])
	}
	sort.Strings(pairs)

	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(strings.Join(pairs, "\n")))

	signed := url.Values{}
	for key, value := range values {
		signed.Set(key, value[0])
	}
	signed.Set("hash", hex.EncodeToString(mac.Sum(nil)))

	return signed
}

// validInitData is a freshly signed payload for a user with the given language.
func validInitData(t *testing.T, languageCode string) url.Values {
	t.Helper()

	return signInitData(t, url.Values{
		"auth_date": {fmt.Sprint(time.Now().Unix())},
		"query_id":  {"AAH-test-query-id"},
		"user": {fmt.Sprintf(
			`{"id":42,"first_name":"Ada","last_name":"Lovelace","username":"ada","language_code":%q}`,
			languageCode,
		)},
	})
}

// stubConfig satisfies the server's Config with a fixed token.
type stubConfig struct {
	staticContentPath string
	webAppPort        int
}

func (c stubConfig) GetStaticContentPath() string { return c.staticContentPath }
func (c stubConfig) GetWebAppPort() int           { return c.webAppPort }
func (c stubConfig) GetToken() string             { return testToken }

// callbackResult records what validateWebAppQuery handed the success callback.
type callbackResult struct {
	called bool
	user   *TgWebAppUser
	texts  *viper.Viper
}

// useLocaleFixture points the locale package at translations for en and de.
func useLocaleFixture(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	files := map[string]string{
		"en.yaml": "hello: |-\n    Hello, %s!\n",
		"de.yaml": "hello: |-\n    Hallo, %s!\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatalf("failed to write fixture %s: %v", name, err)
		}
	}

	locale.SetPath(dir)
}

// serveInitData runs one GET through validateWebAppQuery and reports the response
// alongside whatever the callback saw.
func serveInitData(t *testing.T, values url.Values) (*httptest.ResponseRecorder, *callbackResult) {
	t.Helper()

	srv := NewGinServer(stubConfig{})
	result := &callbackResult{}

	if err := srv.AddWebAppRequestHandler(
		GET,
		"/probe",
		func(c *gin.Context, user *TgWebAppUser, texts *viper.Viper) {
			result.called = true
			result.user = user
			result.texts = texts
			c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte("OK"))
		},
	); err != nil {
		t.Fatalf("AddWebAppRequestHandler() unexpected error: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/probe?"+values.Encode(), nil)
	srv.router.ServeHTTP(recorder, request)

	return recorder, result
}

func TestValidateWebAppQueryAcceptsValidInitData(t *testing.T) {
	useLocaleFixture(t)

	recorder, result := serveInitData(t, validInitData(t, "en"))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", recorder.Code, recorder.Body)
	}
	if !result.called {
		t.Fatal("the success callback was not invoked")
	}

	if result.user.ID != 42 {
		t.Errorf("user ID = %d, want 42", result.user.ID)
	}
	if result.user.FirstName != "Ada" || result.user.LastName != "Lovelace" {
		t.Errorf("user name = %q %q, want Ada Lovelace", result.user.FirstName, result.user.LastName)
	}
	if result.user.Username != "ada" {
		t.Errorf("username = %q, want %q", result.user.Username, "ada")
	}
	// query_id is not part of the user JSON; the handler copies it off the query.
	if result.user.QueryID != "AAH-test-query-id" {
		t.Errorf("query ID = %q, want it copied from the query", result.user.QueryID)
	}
	if got := result.texts.GetString("hello"); got != "Hello, %s!" {
		t.Errorf("texts resolved to %q, want the English fixture", got)
	}
}

func TestValidateWebAppQueryRejectsBadPayloads(t *testing.T) {
	useLocaleFixture(t)

	tests := []struct {
		name string
		// values is built fresh per case so the timestamps stay current.
		values func(t *testing.T) url.Values
		want   int
	}{
		{
			name: "tampered hash",
			values: func(t *testing.T) url.Values {
				values := validInitData(t, "en")
				values.Set("hash", strings.Repeat("0", len(values.Get("hash"))))
				return values
			},
			want: http.StatusBadRequest,
		},
		{
			name: "tampered payload under a valid-looking hash",
			values: func(t *testing.T) url.Values {
				values := validInitData(t, "en")
				// Signed as user 42, presented as user 1 — this is the attack the HMAC
				// exists to stop.
				values.Set("user", `{"id":1,"first_name":"Mallory","language_code":"en"}`)
				return values
			},
			want: http.StatusBadRequest,
		},
		{
			name: "missing hash",
			values: func(t *testing.T) url.Values {
				values := validInitData(t, "en")
				values.Del("hash")
				return values
			},
			want: http.StatusBadRequest,
		},
		{
			name: "empty query",
			values: func(t *testing.T) url.Values {
				return url.Values{}
			},
			want: http.StatusBadRequest,
		},
		{
			name: "missing auth_date",
			values: func(t *testing.T) url.Values {
				values := validInitData(t, "en")
				values.Del("auth_date")
				// Re-sign, so the payload fails on auth_date rather than on the hash.
				return signInitData(t, values)
			},
			want: http.StatusBadRequest,
		},
		{
			name: "non-numeric auth_date",
			values: func(t *testing.T) url.Values {
				values := validInitData(t, "en")
				values.Set("auth_date", "yesterday")
				return signInitData(t, values)
			},
			want: http.StatusBadRequest,
		},
		{
			name: "auth_date 13h old is outside the replay window",
			values: func(t *testing.T) url.Values {
				values := validInitData(t, "en")
				values.Set("auth_date", fmt.Sprint(time.Now().Add(-13*time.Hour).Unix()))
				return signInitData(t, values)
			},
			want: http.StatusUnauthorized,
		},
		{
			name: "malformed user JSON",
			values: func(t *testing.T) url.Values {
				values := validInitData(t, "en")
				values.Set("user", `{"id":`)
				return signInitData(t, values)
			},
			want: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder, result := serveInitData(t, tt.values(t))

			if recorder.Code != tt.want {
				t.Errorf("status = %d, want %d; body: %s", recorder.Code, tt.want, recorder.Body)
			}
			if result.called {
				t.Error("the success callback ran for a payload that should have been rejected")
			}
		})
	}
}

func TestValidateWebAppQueryAcceptsInitDataInsideTheReplayWindow(t *testing.T) {
	useLocaleFixture(t)

	values := validInitData(t, "en")
	values.Set("auth_date", fmt.Sprint(time.Now().Add(-11*time.Hour).Unix()))

	recorder, result := serveInitData(t, signInitData(t, values))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", recorder.Code, recorder.Body)
	}
	if !result.called {
		t.Error("an 11h old payload was rejected, the window is 12h")
	}
}

func TestValidateWebAppQueryFallsBackToEnglishTexts(t *testing.T) {
	useLocaleFixture(t)

	t.Run("known language", func(t *testing.T) {
		_, result := serveInitData(t, validInitData(t, "de"))
		if !result.called {
			t.Fatal("the success callback was not invoked")
		}
		if got := result.texts.GetString("hello"); got != "Hallo, %s!" {
			t.Errorf("texts resolved to %q, want the German fixture", got)
		}
	})

	t.Run("unknown language", func(t *testing.T) {
		_, result := serveInitData(t, validInitData(t, "kl"))
		if !result.called {
			t.Fatal("the success callback was not invoked")
		}
		if got := result.texts.GetString("hello"); got != "Hello, %s!" {
			t.Errorf("texts resolved to %q, want the English fallback", got)
		}
	})

	t.Run("no language", func(t *testing.T) {
		_, result := serveInitData(t, validInitData(t, ""))
		if !result.called {
			t.Fatal("the success callback was not invoked")
		}
		if got := result.texts.GetString("hello"); got != "Hello, %s!" {
			t.Errorf("texts resolved to %q, want the English fallback", got)
		}
	})
}

func TestValidateWebAppQueryFailsWhenTranslationsAreMissing(t *testing.T) {
	// No locale files at all, so even the English fallback cannot resolve.
	locale.SetPath(t.TempDir())

	recorder, result := serveInitData(t, validInitData(t, "en"))

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500; body: %s", recorder.Code, recorder.Body)
	}
	if result.called {
		t.Error("the success callback ran without resolved texts")
	}
}
