package media

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func stubPlayer(t *testing.T, name, output string) *Launcher {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, name)
	body := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + shellQuote(output) + "\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return &Launcher{
		Out: io.Discard, Err: io.Discard,
		LookPath: func(string) (string, error) { return script, nil },
	}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func TestOpenRunsSelectedPlayerWithURL(t *testing.T) {
	out := filepath.Join(t.TempDir(), "args.txt")
	l := stubPlayer(t, "mpv", out)
	if err := l.Open(context.Background(), "https://media.invalid/episode.mp4", "auto"); err != nil {
		t.Fatal(err)
	}
	args, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Fields(string(args))
	if len(got) != 2 || got[0] != "--no-resume-playback" || got[1] != "https://media.invalid/episode.mp4" {
		t.Fatalf("player args = %v", got)
	}
}

func TestOpenAcceptsDecryptedLocalFile(t *testing.T) {
	out := filepath.Join(t.TempDir(), "args.txt")
	l := stubPlayer(t, "ffplay", out)
	clear := filepath.Join(t.TempDir(), "episode.mp4")
	if err := os.WriteFile(clear, []byte("mp4"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := l.Open(context.Background(), clear, "auto"); err != nil {
		t.Fatal(err)
	}
	args, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(args), clear) {
		t.Fatalf("player args = %q", args)
	}
	if err := l.Open(context.Background(), "relative.mp4", "auto"); err == nil {
		t.Fatal("relative path was accepted")
	}
}

func TestOpenRejectsUnsupportedInput(t *testing.T) {
	l := stubPlayer(t, "mpv", filepath.Join(t.TempDir(), "args.txt"))
	if err := l.Open(context.Background(), "file:///etc/passwd", "mpv"); err == nil {
		t.Fatal("file URL was accepted")
	}
	if err := l.Open(context.Background(), "https://media.invalid/a.mp4", "nano"); err == nil || !strings.Contains(err.Error(), "unsupported player") {
		t.Fatalf("unsupported player err = %v", err)
	}
	missing := &Launcher{LookPath: func(string) (string, error) { return "", errors.New("missing") }}
	if err := missing.Open(context.Background(), "https://media.invalid/a.mp4", "auto"); err == nil || !strings.Contains(err.Error(), "no supported player") {
		t.Fatalf("missing player err = %v", err)
	}
}

func TestSaveDownloadsAndRefusesOverwrite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/episode.mp4" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("fixture media bytes"))
	}))
	t.Cleanup(server.Close)

	l := New()
	path := filepath.Join(t.TempDir(), "episode.mp4")
	written, err := l.Save(context.Background(), server.URL+"/episode.mp4", path)
	if err != nil {
		t.Fatal(err)
	}
	if written != int64(len("fixture media bytes")) {
		t.Fatalf("written = %d", written)
	}
	body, err := os.ReadFile(path)
	if err != nil || string(body) != "fixture media bytes" {
		t.Fatalf("saved = %q err = %v", body, err)
	}
	if _, err := l.Save(context.Background(), server.URL+"/episode.mp4", path); err == nil {
		t.Fatal("existing file was overwritten")
	}
	if _, err := l.Save(context.Background(), server.URL+"/missing.mp4", filepath.Join(t.TempDir(), "x.mp4")); err == nil || !strings.Contains(err.Error(), "HTTP 404") {
		t.Fatalf("missing media err = %v", err)
	}
}

func TestDecryptInvokesFFmpegWithKey(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "ffmpeg-args.txt")
	script := filepath.Join(dir, "ffmpeg")
	body := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + shellQuote(argsFile) +
		"\nfor last; do :; done\nprintf 'decrypted' > \"$last\"\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	l := &Launcher{
		Out: io.Discard, Err: io.Discard,
		LookPath: func(string) (string, error) { return script, nil },
	}
	out := filepath.Join(dir, "out.mp4")
	key := strings.Repeat("0123456789abcdef", 2)
	n, err := l.Decrypt(context.Background(), "https://media.invalid/drm.mp4", "", key, out, 0)
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatalf("wrote %d bytes", n)
	}
	raw, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	args := string(raw)
	for _, want := range []string{"-decryption_key", key, "-i", "https://media.invalid/drm.mp4", out} {
		if !strings.Contains(args, want) {
			t.Fatalf("missing %q in args:\n%s", want, args)
		}
	}
}

func TestDecryptRejectsBadKeyAndExistingOutput(t *testing.T) {
	l := &Launcher{Out: io.Discard, Err: io.Discard, LookPath: func(string) (string, error) { return "/bin/true", nil }}
	if _, err := l.Decrypt(context.Background(), "https://media.invalid/x.mp4", "", "zz", filepath.Join(t.TempDir(), "a.mp4"), 0); err == nil {
		t.Fatal("short key accepted")
	}
	existing := filepath.Join(t.TempDir(), "exists.mp4")
	if err := os.WriteFile(existing, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Decrypt(context.Background(), "https://media.invalid/x.mp4", "", strings.Repeat("aa", 16), existing, 0); err == nil {
		t.Fatal("existing output overwritten")
	}
}

func TestDecryptWithoutFFmpeg(t *testing.T) {
	l := &Launcher{
		Out: io.Discard, Err: io.Discard,
		LookPath: func(string) (string, error) { return "", errors.New("not found") },
	}
	_, err := l.Decrypt(context.Background(), "https://media.invalid/x.mp4", "", strings.Repeat("aa", 16), filepath.Join(t.TempDir(), "a.mp4"), 0)
	if err == nil || !strings.Contains(err.Error(), "ffmpeg") {
		t.Fatalf("err = %v", err)
	}
}
