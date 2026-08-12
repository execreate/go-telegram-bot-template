package locale

import (
	"os"
	"path/filepath"
	"testing"
)

// writeLocaleFixture creates a locale directory with the given files and points the
// package at it. The path is restored when the test ends.
func writeLocaleFixture(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatalf("failed to write fixture %s: %v", name, err)
		}
	}

	previous := localesConfig.GetString("locale-path")
	t.Cleanup(func() { SetPath(previous) })
	SetPath(dir)

	return dir
}

func TestSetPathPointsAtAFixture(t *testing.T) {
	writeLocaleFixture(t, map[string]string{
		"en.yaml": "hello: |-\n    Hello from the fixture, %s!\n",
	})

	texts, err := GetTextTranslations("en")
	if err != nil {
		t.Fatalf("GetTextTranslations() unexpected error: %v", err)
	}

	if got := texts.GetString("hello"); got != "Hello from the fixture, %s!" {
		t.Errorf("hello = %q, want the fixture text", got)
	}
}

func TestSetPathDropsTheCache(t *testing.T) {
	writeLocaleFixture(t, map[string]string{"en.yaml": "hello: first\n"})

	if _, err := GetTextTranslations("en"); err != nil {
		t.Fatalf("GetTextTranslations() unexpected error: %v", err)
	}

	// Pointing somewhere else must not keep serving the locale parsed from the old
	// path, or a fork switching locale directories at runtime would never see it.
	writeLocaleFixture(t, map[string]string{"en.yaml": "hello: second\n"})

	texts, err := GetTextTranslations("en")
	if err != nil {
		t.Fatalf("GetTextTranslations() unexpected error: %v", err)
	}
	if got := texts.GetString("hello"); got != "second" {
		t.Errorf("hello = %q, want %q — the cache survived SetPath", got, "second")
	}
}

func TestUnknownLocaleFallsBackToEnglish(t *testing.T) {
	writeLocaleFixture(t, map[string]string{
		"en.yaml":          "hello: english\n",
		"en_commands.yaml": "general:\n    start: Start the bot\n",
	})

	t.Run("texts", func(t *testing.T) {
		texts, err := GetTextTranslations("kl")
		if err != nil {
			t.Fatalf("GetTextTranslations() unexpected error: %v", err)
		}
		if got := texts.GetString("hello"); got != "english" {
			t.Errorf("hello = %q, want the English fallback", got)
		}
	})

	t.Run("commands", func(t *testing.T) {
		cmds, err := GetCmdTranslations("kl")
		if err != nil {
			t.Fatalf("GetCmdTranslations() unexpected error: %v", err)
		}
		if got := cmds.GetStringMapString("general")["start"]; got != "Start the bot" {
			t.Errorf("general.start = %q, want the English fallback", got)
		}
	})

	t.Run("empty locale is treated as English", func(t *testing.T) {
		texts, err := GetTextTranslations("")
		if err != nil {
			t.Fatalf("GetTextTranslations() unexpected error: %v", err)
		}
		if got := texts.GetString("hello"); got != "english" {
			t.Errorf("hello = %q, want the English fallback", got)
		}
	})
}

func TestMissingEnglishLocaleIsAnError(t *testing.T) {
	writeLocaleFixture(t, map[string]string{})

	if _, err := GetTextTranslations("en"); err == nil {
		t.Error("GetTextTranslations() with no locale files returned no error")
	}
	if _, err := GetCmdTranslations("en"); err == nil {
		t.Error("GetCmdTranslations() with no locale files returned no error")
	}
}
