package users

import (
	"strings"
	"testing"
)

func TestSanitizeDisplayName(t *testing.T) {
	cases := map[string]string{
		"Ada Lovelace":                "Ada Lovelace",
		"  spaced   out\tname  ":      "spaced out name",
		"bidi\u202eevil":              "bidievil",
		"zero\u200bwidth\x00ctrl\x07": "zerowidthctrl",
		"":                            "fallback",
		" \n\t ":                      "fallback",
	}
	for in, want := range cases {
		if got := sanitizeDisplayName(in, "fallback"); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
	long := strings.Repeat("é", 100)
	if got := sanitizeDisplayName(long, "x"); len([]rune(got)) != 80 {
		t.Errorf("not truncated to 80 runes: %d", len([]rune(got)))
	}
}

func TestSanitizeAvatarURL(t *testing.T) {
	ok := "https://avatars.githubusercontent.com/u/583231?v=4"
	if got := sanitizeAvatarURL(ok); got == nil || *got != ok {
		t.Fatalf("valid avatar rejected: %v", got)
	}
	for _, bad := range []string{
		"http://avatars.githubusercontent.com/u/1",
		"https://evil.example/u/1",
		"https://avatars.githubusercontent.com.evil.example/u/1",
		"https://user:pw@avatars.githubusercontent.com/u/1",
		"javascript:alert(1)",
		"",
		"https://avatars.githubusercontent.com/" + strings.Repeat("a", 600),
	} {
		if got := sanitizeAvatarURL(bad); got != nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestWithSuffix(t *testing.T) {
	if got := withSuffix("octocat", "2"); got != "octocat-2" {
		t.Fatal(got)
	}
	long := strings.Repeat("a", 39)
	if got := withSuffix(long, "a1b2c3"); len(got) != 39 || !strings.HasSuffix(got, "-a1b2c3") {
		t.Fatalf("%q (%d)", got, len(got))
	}
	// Truncation must not leave a dangling hyphen before the separator.
	if got := withSuffix(strings.Repeat("a", 35)+"-bbb", "2"); strings.Contains(got, "--") || !handlePattern.MatchString(got) {
		t.Fatalf("invalid handle %q", got)
	}
}
