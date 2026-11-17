package catalog

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"vendrix/backend/internal/httpx"
)

func testImage(w, h int) image.Image {
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		m.Set(x, 0, color.RGBA{200, 40, 40, 255})
	}
	return m
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, testImage(w, h)); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// jpegWithExif: JPEG er SOI er porei APP1 Exif segment (GPS er moto lekha shoho).
func jpegWithExif(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, testImage(20, 10), nil); err != nil {
		t.Fatal(err)
	}
	app1 := []byte("Exif\x00\x00GPS 23.81N 90.41E")
	out := append([]byte{0xFF, 0xD8, 0xFF, 0xE1, byte((len(app1) + 2) >> 8), byte(len(app1) + 2)}, app1...)
	return append(out, b.Bytes()[2:]...)
}

// form: multipart body; file nil hole file part nai; fields "naam", "value" jora.
func form(file []byte, fields ...string) (*bytes.Buffer, string) {
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	for i := 0; i+1 < len(fields); i += 2 {
		_ = mw.WriteField(fields[i], fields[i+1])
	}
	if file != nil {
		fw, _ := mw.CreateFormFile("file", "x.bin")
		_, _ = fw.Write(file)
	}
	_ = mw.Close()
	return &b, mw.FormDataContentType()
}

func (f *fixture) upload(t *testing.T, st httpx.Staff, product string, file []byte, want int, fields ...string) map[string]any {
	t.Helper()
	body, ctype := form(file, fields...)
	r := httptest.NewRequest("POST", "/api/admin/products/"+product+"/images", body)
	r.Header.Set("Content-Type", ctype)
	r = r.WithContext(httpx.WithStaff(r.Context(), st))
	rec := httptest.NewRecorder()
	f.srv.ServeHTTP(rec, r)
	if rec.Code != want {
		t.Fatalf("upload %s: %d %s", product, rec.Code, rec.Body.String())
	}
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

// files: store er dir e koyta file.
func (f *fixture) files(t *testing.T) int {
	t.Helper()
	n := 0
	_ = filepath.WalkDir(f.store.Dir, func(_ string, e os.DirEntry, err error) error {
		if err == nil && !e.IsDir() {
			n++
		}
		return nil
	})
	return n
}

// TestImageUploadIntegration (2.2, 12.2 item 8, 11.7 item 9, 14.2 test 29): JPEG/PNG decode + re-encode (EXIF
// nai), SVG/WebP/PDF 400, 4000+ px 400, 10 MB+ 413 (MaxBytesReader, multipart er age), max 10, vendor/shop
// scope 404, DB fail e object muche, key er akar, order PATCH, DELETE row age tarpor object, audit.
func TestImageUploadIntegration(t *testing.T) {
	f := newFixture(t)
	a, b := f.owner(t, "a"), f.owner(t, "b")
	p := idOf(f.admin(t, a, "POST", "/api/admin/products", `{"slug":"chal","name":{"bn":"চাল"}}`, 201))

	img := f.upload(t, a, p, jpegWithExif(t), 201, "alt_bn", "লাল চাল", "alt_en", "Red rice")
	key, _ := img["storage_key"].(string)
	if !regexp.MustCompile(`^shops/`+s64(a.ShopID)+`/products/`+p+`/[A-Z2-7]{26}\.jpg$`).MatchString(key) ||
		img["width"] != float64(20) || img["height"] != float64(10) || img["sort_order"] != float64(0) ||
		img["alt"].(map[string]any)["bn"] != "লাল চাল" || img["shop_id"] != nil {
		t.Errorf("jpeg: %v", img)
	}
	stored, err := os.ReadFile(filepath.Join(f.store.Dir, key))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(stored, []byte("Exif")) || bytes.Contains(stored, []byte("GPS")) {
		t.Error("EXIF thheke geche")
	}
	if _, format, err := image.DecodeConfig(bytes.NewReader(stored)); err != nil || format != "jpeg" {
		t.Errorf("stored jpeg: %s %v", format, err)
	}
	img2 := f.upload(t, a, p, pngBytes(t, 4000, 3), 201)
	if k := img2["storage_key"].(string); !strings.HasSuffix(k, ".png") || img2["sort_order"] != float64(1) || img2["width"] != float64(4000) {
		t.Errorf("png: %v", img2)
	}
	if f.files(t) != 2 {
		t.Fatalf("files: %d", f.files(t))
	}

	for name, c := range map[string]struct {
		file   []byte
		fields []string
		want   string
	}{
		"svg":        {[]byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`), nil, "validation_failed file=unsupported"},
		"webp":       {[]byte("RIFF\x1a\x00\x00\x00WEBPVP8 \x0e\x00\x00\x00"), nil, "validation_failed file=unsupported"},
		"pdf":        {[]byte("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n"), nil, "validation_failed file=unsupported"},
		"truncated":  {jpegWithExif(t)[:40], nil, "validation_failed file=unsupported"},
		"wide":       {pngBytes(t, 4001, 1), nil, "validation_failed file=too_large"},
		"tall":       {pngBytes(t, 1, 4001), nil, "validation_failed file=too_large"},
		"no file":    {nil, []string{"alt_bn", "x"}, "validation_failed file=required"},
		"alt long":   {pngBytes(t, 2, 2), []string{"alt_bn", strings.Repeat("চ", 201)}, "validation_failed alt_bn=invalid"},
		"extra part": {pngBytes(t, 2, 2), []string{"nai", "x"}, "validation_failed body=invalid"},
	} {
		if got := errCode(f.upload(t, a, p, c.file, 400, c.fields...)); got != c.want {
			t.Errorf("%s: %s, chai %s", name, got, c.want)
		}
	}
	// duita file part
	var two bytes.Buffer
	mw := multipart.NewWriter(&two)
	for range 2 {
		fw, _ := mw.CreateFormFile("file", "x.png")
		_, _ = fw.Write(pngBytes(t, 2, 2))
	}
	_ = mw.Close()
	r := httptest.NewRequest("POST", "/api/admin/products/"+p+"/images", &two)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	f.srv.ServeHTTP(rec, r.WithContext(httpx.WithStaff(r.Context(), a)))
	if rec.Code != 400 {
		t.Errorf("duita file: %d %s", rec.Code, rec.Body.String())
	}
	// 10 MB er beshi: MaxBytesReader 413 (pura body pora na)
	big := append(pngBytes(t, 2, 2), make([]byte, 10<<20)...)
	if got := errCode(f.upload(t, a, p, big, 413)); got != "validation_failed" {
		t.Errorf("10 MB+: %s", got)
	}
	f.upload(t, a, p, []byte("x"), 400, "alt_bn", "x")
	r = httptest.NewRequest("POST", "/api/admin/products/"+p+"/images", strings.NewReader(`{}`))
	r.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	f.srv.ServeHTTP(rec, r.WithContext(httpx.WithStaff(r.Context(), a)))
	if rec.Code != 400 {
		t.Errorf("multipart na: %d", rec.Code)
	}
	if f.files(t) != 2 {
		t.Errorf("bhul upload e file roye geche: %d", f.files(t))
	}

	// vendor ar shop scope: onno vendor/shop er product 404, file thake na
	x := f.id(t, `INSERT INTO vendors (shop_id, slug, name) VALUES ($1, 'x', '{"bn":"x"}') RETURNING id`, f.ids["a"])
	vx := httpx.Staff{ID: f.id(t, `INSERT INTO staff (shop_id, vendor_id, email, name, password_hash, role)
		VALUES ($1, $2, 'x@a.com', 'X', 'h', 'vendor_owner') RETURNING id`, f.ids["a"], x), ShopID: f.ids["a"], VendorID: &x, Role: "vendor_owner"}
	f.upload(t, vx, p, pngBytes(t, 2, 2), 404)
	f.upload(t, b, p, pngBytes(t, 2, 2), 404)
	f.upload(t, a, "999999", pngBytes(t, 2, 2), 404)
	if f.files(t) != 2 {
		t.Errorf("scope: files %d", f.files(t))
	}

	// DB fail (trigger): object muche, 500
	f.exec(t, `CREATE FUNCTION fail_img() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'boom'; END $$`)
	f.exec(t, `CREATE TRIGGER fail_img BEFORE INSERT ON product_images FOR EACH ROW EXECUTE FUNCTION fail_img()`)
	f.upload(t, a, p, pngBytes(t, 2, 2), 500)
	f.exec(t, `DROP TRIGGER fail_img ON product_images`)
	if f.files(t) != 2 {
		t.Errorf("DB fail er por file: %d", f.files(t))
	}

	// order: puro id set, ulta kram
	i1, i2 := idOf(img), idOf(img2)
	if got := f.admin(t, a, "PATCH", "/api/admin/products/"+p+"/images/order", `{"ids":[`+i2+`,`+i1+`]}`, 200); len(got["ids"].([]any)) != 2 {
		t.Errorf("order: %v", got)
	}
	if n := f.id(t, `SELECT sort_order FROM product_images WHERE id = $1`, img2["id"]); n != 0 {
		t.Errorf("order sort_order: %d", n)
	}
	for _, body := range []string{`{"ids":[` + i1 + `]}`, `{"ids":[` + i1 + `,` + i1 + `]}`, `{"ids":[` + i1 + `,` + i2 + `,999999]}`, `{"ids":[]}`, `{}`} {
		if got := errCode(f.admin(t, a, "PATCH", "/api/admin/products/"+p+"/images/order", body, 400)); got != "validation_failed ids=invalid" {
			t.Errorf("order %s: %s", body, got)
		}
	}
	f.admin(t, vx, "PATCH", "/api/admin/products/"+p+"/images/order", `{"ids":[`+i2+`,`+i1+`]}`, 404)
	f.admin(t, b, "PATCH", "/api/admin/products/"+p+"/images/order", `{"ids":[`+i2+`,`+i1+`]}`, 404)

	// delete: onno vendor/shop 404; row age, store fail holeo 204 (object pore log)
	f.admin(t, vx, "DELETE", "/api/admin/images/"+i1, "", 404)
	f.admin(t, b, "DELETE", "/api/admin/images/"+i1, "", 404)
	f.admin(t, a, "DELETE", "/api/admin/images/"+i1, "", 204)
	if _, err := os.Stat(filepath.Join(f.store.Dir, key)); !os.IsNotExist(err) {
		t.Errorf("delete er por object: %v", err)
	}
	f.admin(t, a, "DELETE", "/api/admin/images/"+i1, "", 404)
	f.store.failDelete = true
	f.admin(t, a, "DELETE", "/api/admin/images/"+i2, "", 204)
	f.store.failDelete = false
	if n := f.id(t, `SELECT count(*) FROM product_images WHERE product_id = $1`, img["product_id"]); n != 0 {
		t.Errorf("rows: %d", n)
	}

	// max 10: 10 ta row thakle 400 images=max, file na
	f.exec(t, `INSERT INTO product_images (shop_id, product_id, storage_key, width, height)
		SELECT $1, $2, 'shops/x/' || g, 1, 1 FROM generate_series(1, 10) g`, a.ShopID, img["product_id"])
	before := f.files(t)
	if got := errCode(f.upload(t, a, p, pngBytes(t, 2, 2), 400)); got != "validation_failed images=max" {
		t.Errorf("max: %s", got)
	}
	if f.files(t) != before {
		t.Error("max e file roye geche")
	}

	for action, want := range map[string]int64{"image.create": 2, "image.delete": 2, "product.images_order": 1} {
		if n := f.id(t, `SELECT count(*) FROM audit_logs WHERE shop_id = $1 AND action = $2`, a.ShopID, action); n != want {
			t.Errorf("audit %s: %d", action, n)
		}
	}
}

func (f *fixture) exec(t *testing.T, q string, args ...any) {
	t.Helper()
	if _, err := f.d.Exec(q, args...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

// TestImageUploadSemaphore (11.7 item 9, Q9): decode/re-encode ekshathe max 2; slot khali hole chole.
func TestImageUploadSemaphore(t *testing.T) {
	f := newFixture(t)
	a := f.owner(t, "a")
	p := idOf(f.admin(t, a, "POST", "/api/admin/products", `{"slug":"chal","name":{"bn":"চাল"}}`, 201))
	if cap(f.h.sem) != 2 {
		t.Fatalf("semaphore cap %d", cap(f.h.sem))
	}
	f.h.sem <- struct{}{}
	f.h.sem <- struct{}{}
	done := make(chan int)
	go func() {
		body, ctype := form(pngBytes(t, 2, 2))
		r := httptest.NewRequest("POST", "/api/admin/products/"+p+"/images", body)
		r.Header.Set("Content-Type", ctype)
		rec := httptest.NewRecorder()
		f.srv.ServeHTTP(rec, r.WithContext(httpx.WithStaff(r.Context(), a)))
		done <- rec.Code
	}()
	select {
	case c := <-done:
		t.Fatalf("slot chhara shesh: %d", c)
	case <-time.After(300 * time.Millisecond):
	}
	<-f.h.sem
	select {
	case c := <-done:
		if c != 201 {
			t.Errorf("slot er por: %d", c)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("slot khali holeo atke ache")
	}
	<-f.h.sem
}

// TestImageUploadDeadline (14.2 test 29, 2.2): server er read/write timeout er beshi shomoy newa upload er-o
// response ashe (route nijer deadline baray). Server timeout choto (300ms), upload 900ms dhore.
func TestImageUploadDeadline(t *testing.T) {
	f := newFixture(t)
	a := f.owner(t, "a")
	p := idOf(f.admin(t, a, "POST", "/api/admin/products", `{"slug":"chal","name":{"bn":"চাল"}}`, 201))
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.srv.ServeHTTP(w, r.WithContext(httpx.WithStaff(r.Context(), a)))
	}))
	ts.Config.ReadTimeout, ts.Config.WriteTimeout = 300*time.Millisecond, 300*time.Millisecond
	ts.Start()
	defer ts.Close()

	body, ctype := form(pngBytes(t, 50, 50))
	raw := body.Bytes()
	pr, pw := io.Pipe()
	go func() {
		for i := 0; i < len(raw); i += len(raw)/3 + 1 {
			time.Sleep(300 * time.Millisecond)
			_, _ = pw.Write(raw[i:min(i+len(raw)/3+1, len(raw))])
		}
		_ = pw.Close()
	}()
	req, _ := http.NewRequest("POST", ts.URL+"/api/admin/products/"+p+"/images", pr)
	req.Header.Set("Content-Type", ctype)
	res, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("response ashe ni: %v", err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil || res.StatusCode != 201 {
		t.Errorf("deadline: %d %s %v", res.StatusCode, b, err)
	}
}
