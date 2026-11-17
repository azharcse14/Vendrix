package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestSigV4 (2.2): AWS er published vector, hubohu.
//   - "get-vanilla": AWS Signature Version 4 test suite (aws4_testsuite, docs.aws.amazon.com/general/latest/gr/signature-v4-test-suite.html).
//   - "s3 GET Object": Amazon S3 API Reference, "Signature Calculations for the Authorization Header: Transferring
//     Payload in a Single Chunk" er Example: GET Object (examplebucket, Range: bytes=0-9).
func TestSigV4(t *testing.T) {
	empty := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" // sha256("")
	cases := []struct {
		name, url, keyID, secret, region, service string
		at                                        time.Time
		headers                                   map[string]string
		want                                      string
	}{
		{"get-vanilla", "https://example.amazonaws.com/", "AKIDEXAMPLE", "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY", "us-east-1", "service",
			time.Date(2015, 8, 30, 12, 36, 0, 0, time.UTC), nil,
			"AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, " +
				"Signature=5fa00fa31553b73ebf1942676e86291e8372ff2a2260956d9b8aae1d763fbf31"},
		{"s3 GET Object", "https://examplebucket.s3.amazonaws.com/test.txt", "AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			"us-east-1", "s3", time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC),
			map[string]string{"Range": "bytes=0-9", "X-Amz-Content-Sha256": empty},
			"AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request, " +
				"SignedHeaders=host;range;x-amz-content-sha256;x-amz-date, " +
				"Signature=f0e8bdb87c964420e857bd35b5d6ed310bd44f0170aba48dd91039c6036bdb41"},
	}
	for _, c := range cases {
		r, _ := http.NewRequest("GET", c.url, nil)
		for k, v := range c.headers {
			r.Header.Set(k, v)
		}
		signV4(r, empty, c.keyID, c.secret, c.region, c.service, c.at)
		if got := r.Header.Get("Authorization"); got != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.name, got, c.want)
		}
	}
}

// TestR2Store (2.2): path-style PUT/DELETE, body ar content type, payload hash, sign kora; non-2xx error;
// DELETE e 404 ok (age-i nai).
func TestR2Store(t *testing.T) {
	type req struct{ method, path, ctype, hash, auth, body string }
	var got []req
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = append(got, req{r.Method, r.URL.Path, r.Header.Get("Content-Type"), r.Header.Get("X-Amz-Content-Sha256"),
			r.Header.Get("Authorization"), string(b)})
		w.WriteHeader(status)
	}))
	defer srv.Close()
	s := NewR2("acc", "shop-media", "KEYID", "SECRET")
	if !strings.HasPrefix(s.Endpoint, "https://acc.r2.cloudflarestorage.com") || s.Client.Timeout == 0 {
		t.Errorf("default: %s %v", s.Endpoint, s.Client.Timeout)
	}
	s.Endpoint = srv.URL
	ctx := context.Background()
	if err := s.Put(ctx, "shops/1/products/2/abc.jpg", []byte("jpeg"), "image/jpeg"); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("jpeg"))
	p := got[0]
	if p.method != "PUT" || p.path != "/shop-media/shops/1/products/2/abc.jpg" || p.ctype != "image/jpeg" || p.body != "jpeg" ||
		p.hash != hex.EncodeToString(sum[:]) || !strings.HasPrefix(p.auth, "AWS4-HMAC-SHA256 Credential=KEYID/") ||
		!strings.Contains(p.auth, "/auto/s3/aws4_request") || !strings.Contains(p.auth, "content-type") {
		t.Errorf("put: %+v", p)
	}
	if err := s.Delete(ctx, "shops/1/products/2/abc.jpg"); err != nil || got[1].method != "DELETE" || got[1].path != p.path {
		t.Errorf("delete: %v %+v", err, got[1])
	}
	status = http.StatusNotFound
	if err := s.Delete(ctx, "shops/1/x.jpg"); err != nil {
		t.Errorf("delete 404: %v", err)
	}
	status = http.StatusForbidden
	if err := s.Put(ctx, "shops/1/x.jpg", nil, "image/png"); err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Errorf("put 403: %v", err)
	}
	if err := s.Put(ctx, "../x.jpg", nil, "image/png"); err == nil {
		t.Error("bhul key cholche")
	}
}
