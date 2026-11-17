package protocol

import (
	"net/url"
	"strings"
)

// RedactedURL keeps scheme, host, and path. Query strings are dropped so
// device ids, tickets, and playback parameters never reach a log line.
func RedactedURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "[invalid URL]"
	}
	return u.Scheme + "://" + u.Host + u.Path
}

// RedactText replaces credential-shaped substrings. It is a backstop for errors.
func RedactText(s string) string {
	replacer := strings.NewReplacer(
		"Cookie", "[redacted]",
		"cookie", "[redacted]",
		"Authorization", "[redacted]",
		"authorization", "[redacted]",
		"device_token", "[redacted]",
		"X-SS-STUB", "[redacted]",
		"x-ss-stub", "[redacted]",
		"X-Gorgon", "[redacted]",
		"x-gorgon", "[redacted]",
		"X-Argus", "[redacted]",
		"x-argus", "[redacted]",
		"X-Medusa", "[redacted]",
		"x-medusa", "[redacted]",
		"X-Helios", "[redacted]",
		"x-helios", "[redacted]",
		"X-Ladon", "[redacted]",
		"x-ladon", "[redacted]",
		"X-Khronos", "[redacted]",
		"x-khronos", "[redacted]",
		"ttreq", "[redacted]",
	)
	return replacer.Replace(s)
}
