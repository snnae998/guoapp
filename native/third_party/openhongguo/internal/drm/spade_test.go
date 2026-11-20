package drm

import "testing"

// Live vector: episode 7690261304497458201 of series 7690184933037329432.
const liveSpade = "nLwSxG6JI85zhT//c7AK4HauO+lzrg7TdZwO0XWrP+J0pDypqQ=="

func TestDeriveMatchesLiveEpisode(t *testing.T) {
	key, err := Derive(liveSpade)
	if err != nil {
		t.Fatal(err)
	}
	if key.Hex != "1be760855c48a8c86000eeb8c5bb1010" {
		t.Fatalf("key = %q", key.Hex)
	}
	if key.Version != "1" {
		t.Fatalf("version = %q", key.Version)
	}
}

func TestDeriveRejectsGarbage(t *testing.T) {
	cases := []string{"", "not base64 !!", "AAAA", "MTIzNDU2Nzg5MDEyMzQ1Ng=="}
	for _, c := range cases {
		if _, err := Derive(c); err == nil {
			t.Fatalf("%q accepted", c)
		}
	}
}
