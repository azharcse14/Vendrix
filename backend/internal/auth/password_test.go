package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
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

// TestPBKDFSemaphore (8.3 step 1): ek shathe 4 tar beshi PBKDF2 na (CPU DoS); 5 tomo opekkha kore, slot
// khalile chole; opekkhay request bondho hole ctx er error.
func TestPBKDFSemaphore(t *testing.T) {
	if cap(pbkdfSem) != 4 {
		t.Fatalf("semaphore %d, chai 4", cap(pbkdfSem))
	}
	h, err := HashPassword("pw")
	if err != nil {
		t.Fatal(err)
	}
	for range 4 {
		pbkdfSem <- struct{}{}
	}
	done := make(chan bool)
	go func() {
		ok, _ := checkPasswordSem(context.Background(), h, "pw")
		done <- ok
	}()
	select {
	case <-done:
		t.Fatal("4 ta cholar shomoy 5 tomo PBKDF2 cholche")
	case <-time.After(100 * time.Millisecond):
	}
	<-pbkdfSem
	if !<-done {
		t.Error("slot khalir por thik password false")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	pbkdfSem <- struct{}{} // abar 4 ta bhora (3 ta age theke)
	if _, err := checkPasswordSem(ctx, h, "pw"); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("opekkhay ctx shesh: %v", err)
	}
	for range 4 {
		<-pbkdfSem
	}
}
