package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
)

// Seal (2.2): AES-256-GCM, random nonce; ad (AAD) bhul hole Open fail, tai ek row er ciphertext onno row e
// copy korle decrypt hoy na. Shop credential ar staff TOTP secret dui jaygay (shop auth ke import kore).
func Seal(key []byte, ad string, plain []byte) (ct, nonce []byte, err error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, gcm.NonceSize())
	rand.Read(nonce)
	return gcm.Seal(nil, nonce, plain, []byte(ad)), nonce, nil
}

// Open: Seal er ulta; key, ad ba nonce bhul hole error.
func Open(key []byte, ad string, ct, nonce []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonce, ct, []byte(ad))
}

func newGCM(key []byte) (cipher.AEAD, error) {
	b, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(b)
}
