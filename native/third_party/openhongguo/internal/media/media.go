// Package media plays a resolved stream with an external player or saves it.
// A caller-supplied content key can be handed to ffmpeg to rewrite a CENC
// stream into a clear file; the package never derives or requests keys and
// never receives account credentials.
package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Launcher runs an installed player or downloads a stream URL.
// Out and Err default to the process streams; Client defaults to a client
// with a 30 second response-header timeout and no cookie jar.
type Launcher struct {
	Out    io.Writer
	Err    io.Writer
	Client *http.Client
	// LookPath is exec.LookPath; tests replace it with a stub.
	LookPath func(string) (string, error)
}

// New returns the default launcher.
func New() *Launcher {
	return &Launcher{
		Out:      os.Stdout,
		Err:      os.Stderr,
		Client:   &http.Client{Timeout: 0, Transport: &http.Transport{ResponseHeaderTimeout: 30 * time.Second}},
		LookPath: exec.LookPath,
	}
}

// Open starts an external player on rawURL, or on an absolute path to an
// existing local file such as a decrypted episode.
// player is auto, mpv, ffplay, vlc, open, or xdg-open.
func (l *Launcher) Open(ctx context.Context, rawURL, player string) error {
	if !isLocalFile(rawURL) {
		if err := validateMediaURL(rawURL); err != nil {
			return err
		}
	}
	name, path, err := l.resolve(player)
	if err != nil {
		return err
	}
	args := playerArgs(name, rawURL)
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdout = orDiscard(l.Out)
	cmd.Stderr = orDiscard(l.Err)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("player %s failed: %w", name, err)
	}
	return nil
}

// Save downloads rawURL into path. An existing path is never overwritten.
// It returns the number of bytes written.
func (l *Launcher) Save(ctx context.Context, rawURL, path string) (int64, error) {
	if err := validateMediaURL(rawURL); err != nil {
		return 0, err
	}
	if strings.TrimSpace(path) == "" {
		return 0, errors.New("download path is empty")
	}
	client := l.Client
	if client == nil {
		client = New().Client
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return 0, err
	}
	written, copyErr := io.Copy(file, resp.Body)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(path)
		return written, fmt.Errorf("download failed: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return written, closeErr
	}
	return written, nil
}

// Decrypt rewrites a CENC (cenc-aes-ctr) MP4 into a clear MP4 with ffmpeg.
//
// The key is the content key the shipped player would receive; this package
// never derives, requests, or guesses it. ffmpeg reads the encrypted source
// (URL or local path) through its mov demuxer, decrypts each sample, and
// copies the streams into path.
func (l *Launcher) Decrypt(ctx context.Context, src, audio, keyHex, path string, limitSeconds int) (int64, error) {
	key, err := normalizeKey(keyHex)
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(path) == "" {
		return 0, errors.New("decrypt path is empty")
	}
	lookup := l.LookPath
	if lookup == nil {
		lookup = exec.LookPath
	}
	ffmpeg, err := lookup("ffmpeg")
	if err != nil {
		return 0, errors.New("ffmpeg is not installed; it is required to decrypt cenc-aes-ctr media")
	}
	if _, err := os.Stat(path); err == nil {
		return 0, errors.New("output file already exists")
	}
	args := append(ffmpegInputArgs(src, audio, key, 0, limitSeconds),
		"-c", "copy", "-movflags", "+faststart", "-f", "mp4", path)
	cmd := exec.CommandContext(ctx, ffmpeg, args...)
	var stderr bytes.Buffer
	cmd.Stdout = orDiscard(l.Out)
	cmd.Stderr = &stderr
	if runErr := cmd.Run(); runErr != nil {
		_ = os.Remove(path)
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = runErr.Error()
		}
		return 0, fmt.Errorf("decrypt failed: %s", firstLine(msg))
	}
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// ffmpegInputArgs reads src (and a separate audio track, if any) from
// startSeconds on, decrypting both with key when it is set, and maps the
// streams to keep.
func ffmpegInputArgs(src, audio, key string, startSeconds, limitSeconds int) []string {
	args := []string{"-hide_banner", "-loglevel", "error", "-nostdin", "-y"}
	input := func(url string) {
		if startSeconds > 0 {
			args = append(args, "-ss", strconv.Itoa(startSeconds))
		}
		if key != "" {
			args = append(args, "-decryption_key", key)
		}
		args = append(args, "-i", url)
	}
	input(src)
	if track := strings.TrimSpace(audio); track != "" {
		// A segmented title carries its audio as a separate protected track.
		input(track)
		args = append(args, "-map", "0:v:0", "-map", "1:a:0")
	} else {
		args = append(args, "-map", "0")
	}
	if limitSeconds > 0 {
		// A preview keeps only the leading seconds, which matters for a
		// feature-length title that would otherwise be decrypted whole.
		args = append(args, "-t", strconv.Itoa(limitSeconds))
	}
	return args
}

// normalizeKey accepts a 32 or 64 character hex AES key and returns it lower
// case. Anything else is rejected before a process starts.
func normalizeKey(keyHex string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(keyHex))
	if len(key) != 32 && len(key) != 64 {
		return "", errors.New("key must be 32 or 64 hex characters")
	}
	for _, r := range key {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return "", errors.New("key must be hex")
		}
	}
	return key, nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

// resolve picks the player command. An explicit player must be installed.
func (l *Launcher) resolve(player string) (name, path string, err error) {
	lookup := l.LookPath
	if lookup == nil {
		lookup = exec.LookPath
	}
	requested := strings.ToLower(strings.TrimSpace(player))
	if requested != "" && requested != "auto" {
		if !supportedPlayer(requested) {
			return "", "", fmt.Errorf("unsupported player %q; use auto, mpv, ffplay, vlc, open, or xdg-open", player)
		}
		if path, err := lookup(requested); err == nil {
			return requested, path, nil
		}
		return "", "", fmt.Errorf("player %q is not installed", requested)
	}
	for _, candidate := range autoPlayers() {
		if path, err := lookup(candidate); err == nil {
			return candidate, path, nil
		}
	}
	return "", "", errors.New("no supported player found; install mpv, ffplay, or vlc, or pass --player")
}

func supportedPlayer(name string) bool {
	switch name {
	case "mpv", "ffplay", "vlc", "open", "xdg-open":
		return true
	}
	return false
}

// autoPlayers prefers players that accept a plain URL, then an OS opener.
// open is macOS only and xdg-open is Linux only.
func autoPlayers() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{"mpv", "ffplay", "vlc", "open"}
	case "linux":
		return []string{"mpv", "ffplay", "vlc", "xdg-open"}
	default:
		return []string{"mpv", "ffplay", "vlc"}
	}
}

func playerArgs(name, rawURL string) []string {
	switch name {
	case "mpv":
		return []string{"--no-resume-playback", rawURL}
	case "ffplay":
		return []string{"-autoexit", "-loglevel", "warning", rawURL}
	case "vlc":
		return []string{"--play-and-exit", rawURL}
	default:
		return []string{rawURL}
	}
}

// isLocalFile reports whether p is an absolute path to an existing regular file.
func isLocalFile(p string) bool {
	if !filepath.IsAbs(p) {
		return false
	}
	info, err := os.Stat(p)
	return err == nil && info.Mode().IsRegular()
}

// validateMediaURL accepts only absolute http and https URLs.
func validateMediaURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return errors.New("media URL is invalid")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("media URL scheme %q is not http or https", u.Scheme)
	}
	return nil
}

func orDiscard(w io.Writer) io.Writer {
	if w == nil {
		return io.Discard
	}
	return w
}
