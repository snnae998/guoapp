// Package drm recovers the CENC content key from the wrapped material the
// anonymous video model carries.
//
// The service ships one opaque blob per representation in
// encrypt_info.spade_a. The shipped player base64-decodes it, scrambles the
// bytes with a fixed permutation and splits the result into a hex content key
// and a short scheme version. That routine lives in the player's native
// library; this package is a behaviour-compatible reimplementation validated
// against a live episode.
package drm

import (
	"encoding/base64"
	"errors"
	"strings"
)

// ErrInvalidMaterial reports wrapped key material this package cannot read.
var ErrInvalidMaterial = errors.New("drm: unrecognized key material")

// Key is one recovered content key.
type Key struct {
	// Hex is the AES content key as lowercase hex.
	Hex string
	// Version is the scheme marker the player reads next to the key.
	Version string
}

// Derive recovers the content key from a base64 spade_a value.
func Derive(spadeA string) (Key, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(spadeA))
	if err != nil || len(raw) < 4 {
		return Key{}, ErrInvalidMaterial
	}
	marker := int(raw[0] ^ raw[1] ^ raw[2])
	prefixLen := marker - 0x30
	if prefixLen < 1 || prefixLen > len(raw) {
		return Key{}, ErrInvalidMaterial
	}
	outLen := len(raw) - marker + 0x2f
	if outLen < 2 || 1+outLen > len(raw) {
		return Key{}, ErrInvalidMaterial
	}
	buf := append([]byte(nil), raw[1:1+outLen]...)

	// The marker bytes are the two bytes before the tail window xored with
	// every byte of that window.
	tail := len(raw) - prefixLen
	if tail-2 < 0 || tail > len(raw) {
		return Key{}, ErrInvalidMaterial
	}
	b16, b14 := raw[tail-2], raw[tail-1]
	tag := make([]byte, prefixLen)
	for i := range tag {
		if i+tail >= len(raw) {
			return Key{}, ErrInvalidMaterial
		}
		tag[i] = b14 ^ b16 ^ raw[i+tail]
	}
	tagText := string(tag)
	scramble(buf, strings.HasPrefix(tagText, "app_v2") || strings.HasPrefix(tagText, "web_v2"))

	versionLen := hexDigit(buf[0])
	if versionLen < 1 || versionLen >= outLen {
		return Key{}, ErrInvalidMaterial
	}
	keyLen := outLen - versionLen
	hexKey := string(buf[1:keyLen])
	if hexKey == "" || len(hexKey)%2 != 0 {
		return Key{}, ErrInvalidMaterial
	}
	for i := 0; i < len(hexKey); i++ {
		if hexDigit(hexKey[i]) < 0 {
			return Key{}, ErrInvalidMaterial
		}
	}
	return Key{Hex: strings.ToLower(hexKey), Version: string(buf[keyLen:])}, nil
}

// scramble applies the player's byte permutation in place.
//
// v2 marks the "app_v2"/"web_v2" family, which swaps adjacent bytes first and
// subtracts the previous rolling constant.
func scramble(buf []byte, v2 bool) {
	if v2 {
		last := len(buf) - 1
		for i := 0; i+1 < len(buf); i += 2 {
			buf[i], buf[i+1] = buf[i+1], buf[i]
			if i == last-2 {
				break
			}
		}
	}
	var prev, cur byte = 0x55, 0xfa
	for i := range buf {
		b := buf[i]
		term := -byte(0x15 + popcount(uint32(i)))
		if i%2 == 0 {
			if v2 {
				buf[i] = (cur ^ b) - prev + term
			} else {
				buf[i] = term + (cur ^ b)
			}
			cur = b
		} else {
			if v2 {
				buf[i] = (prev ^ b) - prev + term
			} else {
				buf[i] = term + (prev ^ b)
			}
			prev = b
		}
	}
}

// popcount counts the set bits of all four bytes of v, matching the player's
// NEON pairwise-add of the byte lanes.
func popcount(v uint32) int {
	return int(bits8(byte(v)) + bits8(byte(v>>8)) + bits8(byte(v>>16)) + bits8(byte(v>>24)))
}

func bits8(v byte) byte {
	var n byte
	for v != 0 {
		v &= v - 1
		n++
	}
	return n
}

func hexDigit(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}
