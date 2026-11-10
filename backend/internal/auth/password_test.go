package auth

import (
	"strings"
	"testing"
)

// TestPasswordHash (8.3): format pbkdf2-sha256$600000$<salt>$<hash>, protibar notun salt, thik password e
// true, bhul ba bhul format e false.
func TestPasswordHash(t *testing.T) {
	h, err := HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(h, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" || parts[1] != "600000" {
		t.Fatalf("format: %q", h)
	}
	if h2, _ := HashPassword("correct horse"); h2 == h {
		t.Error("ekoi salt duibar")
	}
	if !CheckPassword(h, "correct horse") {
		t.Error("thik password e false")
	}
	for _, bad := range []string{"correct hors", "", "Correct horse"} {
		if CheckPassword(h, bad) {
			t.Errorf("%q e true", bad)
		}
	}
	for _, stored := range []string{"", "x", "bcrypt$600000$a$b", "pbkdf2-sha256$abc$" + parts[2] + "$" + parts[3],
		"pbkdf2-sha256$0$" + parts[2] + "$" + parts[3], "pbkdf2-sha256$600000$!!$" + parts[3],
		"pbkdf2-sha256$99999999999$" + parts[2] + "$" + parts[3]} {
		if CheckPassword(stored, "correct horse") {
			t.Errorf("bhul format %q e true", stored)
		}
	}
}
