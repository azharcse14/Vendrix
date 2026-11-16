package order

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base32"
	"strings"
	"testing"
)

// TestConfirmToken (8.10 item 1, 2; 14.2 test 31 er unit ongsho): base32(HMAC-SHA256(secret, "cod-confirm:" +
// public_token)) er prothom 26 okkhor, uppercase; ekoi input e ekoi token (SMS retry te ekoi link); hash er age
// uppercase (link e case bodlaleo chole).
func TestConfirmToken(t *testing.T) {
	const secret, pub = "confirm-secret-0123456789abcdef01", "PUBLICTOKEN234567ABCDEFGHIJ"
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte("cod-confirm:" + pub))
	want := base32.StdEncoding.EncodeToString(m.Sum(nil))[:26]

	tok := ConfirmToken(secret, pub)
	if tok != want {
		t.Fatalf("token %q, chai %q", tok, want)
	}
	if len(tok) != 26 || strings.ToUpper(tok) != tok || strings.Trim(tok, "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567") != "" {
		t.Errorf("token %q: 26 okkhor base32 uppercase chai", tok)
	}
	if ConfirmToken(secret, pub) != tok {
		t.Error("ekoi input e alada token")
	}
	if ConfirmToken(secret+"x", pub) == tok || ConfirmToken(secret, pub+"X") == tok {
		t.Error("secret ba public_token bodlaleo ekoi token")
	}
	sum := sha256.Sum256([]byte(tok))
	for _, s := range []string{tok, strings.ToLower(tok), strings.ToLower(tok[:10]) + tok[10:]} {
		if !bytes.Equal(ConfirmHash(s), sum[:]) {
			t.Errorf("ConfirmHash(%q) sha256(uppercase) na", s)
		}
	}
	if bytes.Equal(ConfirmHash(pub), sum[:]) {
		t.Error("public_token er hash confirm token er sathe mile")
	}
}
