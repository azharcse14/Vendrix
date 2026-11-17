package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestLocalStore (11.7 item 9): Put dir er bhetore file likhe (folder nijei banay), Delete muche (nai holeo ok);
// key dir er baire jete pare na.
func TestLocalStore(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "uploads")
	s := Local{Dir: dir}
	ctx := context.Background()
	key := "shops/1/products/2/abc.jpg"
	if err := s.Put(ctx, key, []byte("jpeg"), "image/jpeg"); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, key)); err != nil || string(b) != "jpeg" {
		t.Fatalf("file: %q %v", b, err)
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, key)); !os.IsNotExist(err) {
		t.Errorf("delete er por: %v", err)
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Errorf("abar delete: %v", err)
	}
	for _, bad := range []string{"", "../x.jpg", "shops/../../x.jpg", "/etc/x.jpg", "shops//x.jpg", "shops/./x.jpg", `shops\x.jpg`, "shops/"} {
		if err := s.Put(ctx, bad, []byte("x"), "image/jpeg"); err == nil {
			t.Errorf("%q: Put cholche", bad)
		}
		if err := s.Delete(ctx, bad); err == nil {
			t.Errorf("%q: Delete cholche", bad)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "x.jpg")); !os.IsNotExist(err) {
		t.Error("dir er baire file")
	}
}
