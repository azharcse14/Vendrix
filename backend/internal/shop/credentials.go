// Package shop: shop er data: X-Shop-Host theke shop chena (3.2, Resolver), shop_credentials (SMS, payment,
// courier er secret) AES-GCM e encrypt kore save (2.2, 12.2; API theke kokhono ferot na).
//
// Feature doc: docs/features/shop.md, docs/features/credentials.md
package shop

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// ponytail: ekta CREDENTIALS_KEY, tai version shob shomoy 1; rotation (13.5) e purono ar notun key duitai lagbe.
const keyVersion = 1

// aad (2.2): ek shop ba provider er ciphertext onno row e copy korle decrypt hoy na.
func aad(shopID int64, provider string, version int) string {
	return fmt.Sprintf("%d:%s:%d", shopID, provider, version)
}

func seal(key []byte, ad string, plain []byte) (ct, nonce []byte, err error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, gcm.NonceSize())
	rand.Read(nonce)
	return gcm.Seal(nil, nonce, plain, []byte(ad)), nonce, nil
}

// open: M4 e credential porar shomoy (ekhon test e).
func open(key []byte, ad string, ct, nonce []byte) ([]byte, error) {
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

// SetCredential (3.8, `shop-api credentials set`): secret JSON object, slug er shop e provider er row upsert.
// Error e secret thake na.
func SetCredential(ctx context.Context, d *sql.DB, key []byte, slug, provider string, secret []byte) error {
	var obj map[string]any
	if err := json.Unmarshal(secret, &obj); err != nil || obj == nil {
		return errors.New("secret JSON object dao (jemon {\"api_key\":\"...\"})")
	}
	var shopID int64
	err := d.QueryRowContext(ctx, `SELECT id FROM shops WHERE slug = $1`, slug).Scan(&shopID)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("shop %q nai", slug)
	}
	if err != nil {
		return err
	}
	ct, nonce, err := seal(key, aad(shopID, provider, keyVersion), secret)
	if err != nil {
		return err
	}
	_, err = d.ExecContext(ctx, `INSERT INTO shop_credentials (shop_id, provider, secret_enc, nonce, key_version)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (shop_id, provider) DO UPDATE
		SET secret_enc = EXCLUDED.secret_enc, nonce = EXCLUDED.nonce, key_version = EXCLUDED.key_version, updated_at = now()`,
		shopID, provider, ct, nonce, keyVersion)
	if err != nil {
		return fmt.Errorf("provider %q: %w", provider, err)
	}
	return nil
}
