package auth

import (
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

const (
	iterations = 600_000
	// maxIterations: database er bhul value e CPU atke na jay.
	maxIterations = 10_000_000
	saltLen       = 16
	keyLen        = 32
)

// HashPassword (8.3): "pbkdf2-sha256$600000$<base64 salt>$<base64 hash>", 16 byte random salt.
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	rand.Read(salt)
	key, err := pbkdf2.Key(sha256.New, password, salt, iterations, keyLen)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", iterations,
		base64.StdEncoding.EncodeToString(salt), base64.StdEncoding.EncodeToString(key)), nil
}

// CheckPassword: stored hash er iteration ar salt diye tulona (subtle, constant time). Bhul format e false.
func CheckPassword(stored, password string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter < 1 || iter > maxIterations {
		return false
	}
	salt, err1 := base64.StdEncoding.DecodeString(parts[2])
	want, err2 := base64.StdEncoding.DecodeString(parts[3])
	if err1 != nil || err2 != nil || len(want) == 0 {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iter, len(want))
	return err == nil && subtle.ConstantTimeCompare(got, want) == 1
}

// dummyHash (8.3 step 1): ochena email e-o ekta 600000 iteration er tulona, jate response er shomoy dekhe
// email ache kina bojha na jay. Password random, kono password e mile na.
const dummyHash = "pbkdf2-sha256$600000$sbcxGh7BiclZnlK3brM2rw==$ojvMPx6ZlYTD6Oyu+YkcBLCVZUx/QR5DqHcQAnGYYqw="

// pbkdfSem (8.3 step 1): ek shathe 4 tar beshi PBKDF2 na; na hole login er dhel e CPU bhore baki API atke.
var pbkdfSem = make(chan struct{}, 4)

// pbkdfSlot: semaphore er slot (release ferot); opekkhay request bondho hole ctx er error.
func pbkdfSlot(ctx context.Context) (func(), error) {
	select {
	case pbkdfSem <- struct{}{}:
		return func() { <-pbkdfSem }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// checkPasswordSem: semaphore er slot niye CheckPassword.
func checkPasswordSem(ctx context.Context, stored, password string) (bool, error) {
	release, err := pbkdfSlot(ctx)
	if err != nil {
		return false, err
	}
	defer release()
	return CheckPassword(stored, password), nil
}

// hashPasswordSem: semaphore er slot niye HashPassword (notun password o 600000 iteration).
func hashPasswordSem(ctx context.Context, password string) (string, error) {
	release, err := pbkdfSlot(ctx)
	if err != nil {
		return "", err
	}
	defer release()
	return HashPassword(password)
}
