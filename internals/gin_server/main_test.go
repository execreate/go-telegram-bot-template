package gin_server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func TestAddWebAppRequestHandler(t *testing.T) {
	useLocaleFixture(t)

	tests := []struct {
		name       string
		method     HandlerMethods
		httpMethod string
	}{
		{name: "GET", method: GET, httpMethod: http.MethodGet},
		{name: "POST", method: POST, httpMethod: http.MethodPost},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := NewGinServer(stubConfig{})

			called := false
			if err := srv.AddWebAppRequestHandler(
				tt.method,
				"/probe",
				func(c *gin.Context, _ *TgWebAppUser, _ *viper.Viper) {
					called = true
					c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte("OK"))
				},
			); err != nil {
				t.Fatalf("AddWebAppRequestHandler() unexpected error: %v", err)
			}

			// The route is registered for its own verb only.
			recorder := httptest.NewRecorder()
			target := "/probe?" + validInitData(t, "en").Encode()
			srv.router.ServeHTTP(recorder, httptest.NewRequest(tt.httpMethod, target, nil))

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body: %s", recorder.Code, recorder.Body)
			}
			if !called {
				t.Error("the handler was not invoked")
			}

			other := http.MethodPost
			if tt.httpMethod == http.MethodPost {
				other = http.MethodGet
			}
			recorder = httptest.NewRecorder()
			srv.router.ServeHTTP(recorder, httptest.NewRequest(other, target, nil))

			if recorder.Code == http.StatusOK {
				t.Errorf("the route also answered %s, want it registered for %s only", other, tt.httpMethod)
			}
		})
	}
}

func TestAddWebAppRequestHandlerRejectsAnUnknownMethod(t *testing.T) {
	srv := NewGinServer(stubConfig{})

	err := srv.AddWebAppRequestHandler(
		HandlerMethods("DELETE"),
		"/probe",
		func(c *gin.Context, _ *TgWebAppUser, _ *viper.Viper) {},
	)

	if err == nil {
		t.Fatal("AddWebAppRequestHandler() with an unknown method returned no error")
	}
	if !strings.Contains(err.Error(), "DELETE") {
		t.Errorf("error %q does not name the offending method", err)
	}
}

func TestAddStaticFileHandler(t *testing.T) {
	dir := t.TempDir()
	const body = "<!doctype html><title>terms</title>"

	if err := os.WriteFile(filepath.Join(dir, "terms_and_conditions.en.html"), []byte(body), 0o600); err != nil {
		t.Fatalf("failed to write the fixture: %v", err)
	}

	srv := NewGinServer(stubConfig{staticContentPath: dir})
	srv.AddStaticFileHandler("terms_and_conditions.en.html")

	recorder := httptest.NewRecorder()
	srv.router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/terms_and_conditions.en.html", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if recorder.Body.String() != body {
		t.Errorf("body = %q, want the file's contents", recorder.Body.String())
	}
}

func TestAddStaticFolderHandler(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o750); err != nil {
		t.Fatalf("failed to create the fixture directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "app.css"), []byte("body{}"), 0o600); err != nil {
		t.Fatalf("failed to write the fixture: %v", err)
	}

	srv := NewGinServer(stubConfig{staticContentPath: dir})
	srv.AddStaticFolderHandler("/assets", "assets")

	recorder := httptest.NewRecorder()
	srv.router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/assets/app.css", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if recorder.Body.String() != "body{}" {
		t.Errorf("body = %q, want the file's contents", recorder.Body.String())
	}
}

func TestGetServer(t *testing.T) {
	srv := NewGinServer(stubConfig{webAppPort: 9099})

	type ctxKey string
	const key ctxKey = "probe"

	ctx := context.WithValue(context.Background(), key, "value")
	server := srv.GetServer(ctx)

	if server.Addr != ":9099" {
		t.Errorf("Addr = %q, want %q", server.Addr, ":9099")
	}
	if server.Handler == nil {
		t.Error("GetServer() returned a server with no handler")
	}
	// Every request inherits this context, which is what lets main.go keep in-flight
	// requests alive past SIGTERM.
	if got := server.BaseContext(nil).Value(key); got != "value" {
		t.Errorf("BaseContext does not carry the context it was given, got %v", got)
	}
}

func TestNewGinServerDistrustsProxyHeaders(t *testing.T) {
	useLocaleFixture(t)

	srv := NewGinServer(stubConfig{})

	var clientIP string
	if err := srv.AddWebAppRequestHandler(
		GET,
		"/probe",
		func(c *gin.Context, _ *TgWebAppUser, _ *viper.Viper) {
			clientIP = c.ClientIP()
			c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte("OK"))
		},
	); err != nil {
		t.Fatalf("AddWebAppRequestHandler() unexpected error: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/probe?"+validInitData(t, "en").Encode(), nil)
	request.RemoteAddr = "10.0.0.1:1234"
	// With no trusted proxies, a spoofed forwarding header must not become the client
	// IP — anything rate limiting or logging by IP depends on that.
	request.Header.Set("X-Forwarded-For", "1.2.3.4")

	srv.router.ServeHTTP(httptest.NewRecorder(), request)

	if clientIP != "10.0.0.1" {
		t.Errorf("ClientIP() = %q, want the peer address rather than the forwarded header", clientIP)
	}
}
