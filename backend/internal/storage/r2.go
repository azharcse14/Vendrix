package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"
)

// R2: Cloudflare R2 (S3 API, path-style /{bucket}/{key}, region "auto"). Endpoint ar Client test e bodlay.
type R2 struct {
	Endpoint, Bucket, AccessKeyID, SecretAccessKey string
	Client                                         *http.Client
}

func NewR2(accountID, bucket, keyID, secret string) *R2 {
	return &R2{Endpoint: "https://" + accountID + ".r2.cloudflarestorage.com", Bucket: bucket, AccessKeyID: keyID,
		SecretAccessKey: secret, Client: &http.Client{Timeout: 30 * time.Second}}
}

func (s *R2) Put(ctx context.Context, key string, body []byte, contentType string) error {
	return s.do(ctx, http.MethodPut, key, body, contentType)
}

func (s *R2) Delete(ctx context.Context, key string) error {
	return s.do(ctx, http.MethodDelete, key, nil, "")
}

func (s *R2) do(ctx context.Context, method, key string, body []byte, contentType string) error {
	if !validKey(key) {
		return errKey
	}
	req, err := http.NewRequestWithContext(ctx, method, s.Endpoint+"/"+uriEncode(s.Bucket+"/"+key), bytes.NewReader(body))
	if err != nil {
		return err
	}
	sum := sha256.Sum256(body)
	hash := hex.EncodeToString(sum[:])
	req.Header.Set("X-Amz-Content-Sha256", hash)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	signV4(req, hash, s.AccessKeyID, s.SecretAccessKey, "auto", "s3", time.Now())
	res, err := s.Client.Do(req)
	if err != nil {
		return fmt.Errorf("r2 %s: %w", method, err)
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1<<16))
	if res.StatusCode/100 == 2 || (method == http.MethodDelete && res.StatusCode == http.StatusNotFound) {
		return nil
	}
	return fmt.Errorf("r2 %s: status %d", method, res.StatusCode)
}

// uriEncode: S3 er canonical URI (unreserved ar "/" chhara shob %XX).
func uriEncode(s string) string {
	var b strings.Builder
	for _, c := range []byte(s) {
		if 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || '0' <= c && c <= '9' || strings.IndexByte("-._~/", c) >= 0 {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func hmacSHA(key []byte, s string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(s))
	return m.Sum(nil)
}

// signV4: AWS SigV4 Authorization header (header-based, query string chhara). Host ar r.Header er shob
// header sign hoy; X-Amz-Date t theke boshe.
func signV4(r *http.Request, payloadHash, keyID, secret, region, service string, t time.Time) {
	t = t.UTC()
	amzDate, day := t.Format("20060102T150405Z"), t.Format("20060102")
	r.Header.Set("X-Amz-Date", amzDate)
	hdr := map[string]string{"host": r.URL.Host}
	for k, v := range r.Header {
		hdr[strings.ToLower(k)] = strings.Join(strings.Fields(strings.Join(v, ",")), " ")
	}
	names := make([]string, 0, len(hdr))
	for k := range hdr {
		names = append(names, k)
	}
	slices.Sort(names)
	var canon strings.Builder
	for _, k := range names {
		canon.WriteString(k + ":" + hdr[k] + "\n")
	}
	signed := strings.Join(names, ";")
	p := r.URL.EscapedPath()
	if p == "" {
		p = "/"
	}
	creq := strings.Join([]string{r.Method, p, r.URL.RawQuery, canon.String(), signed, payloadHash}, "\n")
	sum := sha256.Sum256([]byte(creq))
	scope := day + "/" + region + "/" + service + "/aws4_request"
	sts := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + hex.EncodeToString(sum[:])
	k := hmacSHA([]byte("AWS4"+secret), day)
	k = hmacSHA(k, region)
	k = hmacSHA(k, service)
	k = hmacSHA(k, "aws4_request")
	r.Header.Set("Authorization", fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		keyID, scope, signed, hex.EncodeToString(hmacSHA(k, sts))))
}
