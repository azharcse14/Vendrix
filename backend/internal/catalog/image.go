package catalog

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"vendrix/backend/internal/httpx"
)

const (
	maxUpload      = 10 << 20 // 2.2
	maxSide        = 4000     // decompression bomb (2.2)
	maxImages      = 10       // product e (11.7 item 9)
	uploadDeadline = 2 * time.Minute
)

// UploadImage: POST /api/admin/products/{id}/images multipart (file, alt_bn, alt_en) → 201 row. Kram (11.7
// item 9): product/shima check → MaxBytesReader e body → JPEG/PNG decode + re-encode (EXIF jay) → Put →
// DB (product lock, abar check) → DB fail e object muche.
func (h *Handlers) UploadImage(w http.ResponseWriter, r *http.Request) {
	// server er WriteTimeout header porar shomoy-i boshe, tai read ar write duitai barai (2.2)
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Now().Add(uploadDeadline))
	_ = rc.SetWriteDeadline(time.Now().Add(uploadDeadline))
	id, ok := pathID(r)
	if !ok {
		h.writeErr(w, r, errNotFound)
		return
	}
	st, _ := httpx.StaffFrom(r.Context())
	// age-i 404/max, jate 10 MB pora ba decode na hoy
	if err := imageRoom(r.Context(), h.db, st.ShopID, id, vendorScope(st), nil); err != nil {
		h.writeErr(w, r, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
	file, alt, err := readImageForm(r)
	var tooBig *http.MaxBytesError
	switch {
	case errors.As(err, &tooBig):
		httpx.WriteError(w, r, http.StatusRequestEntityTooLarge, httpx.ErrorBody{Code: "validation_failed"})
		return
	case err != nil:
		h.writeErr(w, r, err)
		return
	}
	select {
	case h.sem <- struct{}{}:
	case <-r.Context().Done():
		return
	}
	out, ext, width, height, err := reencode(file)
	<-h.sem
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	key := fmt.Sprintf("shops/%d/products/%d/%s.%s", st.ShopID, id, rand.Text(), ext)
	ctype := map[string]string{"jpg": "image/jpeg", "png": "image/png"}[ext]
	if err := h.store.Put(r.Context(), key, out, ctype); err != nil {
		h.fail(w, r, err)
		return
	}
	altJSON, _ := json.Marshal(alt)
	after, err := h.audited(r, "image.create", func(tx *sql.Tx, st httpx.Staff) (int64, json.RawMessage, json.RawMessage, error) {
		var next int
		if err := imageRoom(r.Context(), tx, st.ShopID, id, vendorScope(st), &next); err != nil {
			return 0, nil, nil, err
		}
		return insertImage(r.Context(), tx, st.ShopID, id, key, string(altJSON), width, height, next)
	})
	if err != nil {
		// client chole geleo object muchte hobe
		if derr := h.store.Delete(context.WithoutCancel(r.Context()), key); derr != nil {
			h.log.ErrorContext(r.Context(), "image cleanup", "key", key, "err", derr)
		}
		h.writeErr(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write(after)
}

// readImageForm: shudhu file (ekta), alt_bn, alt_en part; multipart stream e, temp file na.
func readImageForm(r *http.Request) ([]byte, Text, error) {
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, nil, badField("body", "invalid")
	}
	var file []byte
	alt := Text{}
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, bodyErr(err)
		}
		switch name := p.FormName(); name {
		case "file":
			if file != nil {
				return nil, nil, badField("body", "invalid")
			}
			if file, err = io.ReadAll(p); err != nil {
				return nil, nil, bodyErr(err)
			}
		case "alt_bn", "alt_en":
			b, err := io.ReadAll(io.LimitReader(p, 4096))
			if err != nil {
				return nil, nil, bodyErr(err)
			}
			s := strings.TrimSpace(string(b))
			if !utf8.ValidString(s) || utf8.RuneCountInString(s) > 200 {
				return nil, nil, badField(name, "invalid")
			}
			if s != "" {
				alt[strings.TrimPrefix(name, "alt_")] = s
			}
		default:
			return nil, nil, badField("body", "invalid")
		}
	}
	if file == nil {
		return nil, nil, badField("file", "required")
	}
	return file, alt, nil
}

// bodyErr: MaxBytesError (413) hubohu, baki multipart bhul 400.
func bodyErr(err error) error {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		return err
	}
	return badField("body", "invalid")
}

// reencode (2.2): DecodeConfig e format ar maap (pixel decode er age), tarpor decode + encode; EXIF/GPS
// metadata notun file e jay na. SVG/WebP/PDF/onno → unsupported.
func reencode(b []byte) (out []byte, ext string, w, h int, err error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil || (format != "jpeg" && format != "png") || cfg.Width < 1 || cfg.Height < 1 {
		return nil, "", 0, 0, badField("file", "unsupported")
	}
	if cfg.Width > maxSide || cfg.Height > maxSide {
		return nil, "", 0, 0, badField("file", "too_large")
	}
	img, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, "", 0, 0, badField("file", "unsupported")
	}
	var buf bytes.Buffer
	if format == "jpeg" {
		ext, err = "jpg", jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85})
	} else {
		ext, err = "png", png.Encode(&buf, img)
	}
	return buf.Bytes(), ext, img.Bounds().Dx(), img.Bounds().Dy(), err
}

// OrderImages: PATCH /api/admin/products/{id}/images/order {ids}: product er shob chobi er id, notun kram e.
func (h *Handlers) OrderImages(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		h.writeErr(w, r, errNotFound)
		return
	}
	var in struct {
		IDs []int64 `json:"ids"`
	}
	if !decode(w, r, &in) {
		return
	}
	h.write(w, r, http.StatusOK, "product.images_order", func(tx *sql.Tx, st httpx.Staff) (int64, json.RawMessage, json.RawMessage, error) {
		if _, _, err := lockProduct(r.Context(), tx, st.ShopID, id, vendorScope(st)); err != nil {
			return 0, nil, nil, err
		}
		if err := orderImages(r.Context(), tx, st.ShopID, id, in.IDs); err != nil {
			return 0, nil, nil, err
		}
		after, _ := json.Marshal(map[string][]int64{"ids": in.IDs})
		return id, nil, after, nil
	})
}

// DeleteImage: DELETE /api/admin/images/{id} → 204. Row age (commit), tarpor object; object fail hole shudhu log.
func (h *Handlers) DeleteImage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		h.writeErr(w, r, errNotFound)
		return
	}
	var key string
	_, err := h.audited(r, "image.delete", func(tx *sql.Tx, st httpx.Staff) (int64, json.RawMessage, json.RawMessage, error) {
		k, before, err := deleteImage(r.Context(), tx, st.ShopID, id, vendorScope(st))
		key = k
		return id, before, nil, err
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	if err := h.store.Delete(context.WithoutCancel(r.Context()), key); err != nil {
		h.log.ErrorContext(r.Context(), "image object delete", "key", key, "err", err)
	}
	w.WriteHeader(http.StatusNoContent)
}

// lockProduct: product shop/vendor scope e lock (FOR UPDATE; tx chhara autocommit e sathe sathe chhare), na
// hole sql.ErrNoRows (404); chobi koyta ar notun chobi er sort_order.
func lockProduct(ctx context.Context, q querier, shopID, productID int64, vendor any) (n, next int, err error) {
	err = q.QueryRowContext(ctx, `SELECT
		  (SELECT count(*) FROM product_images i WHERE i.shop_id = p.shop_id AND i.product_id = p.id),
		  (SELECT coalesce(max(sort_order) + 1, 0) FROM product_images i WHERE i.shop_id = p.shop_id AND i.product_id = p.id)
		FROM products p WHERE p.shop_id = $1 AND p.id = $2 AND ($3::bigint IS NULL OR p.vendor_id = $3) FOR UPDATE`,
		shopID, productID, vendor).Scan(&n, &next)
	return n, next, err
}

// imageRoom: lockProduct + 10 ta thakle 400 images=max; next != nil hole notun sort_order.
func imageRoom(ctx context.Context, q querier, shopID, productID int64, vendor any, next *int) error {
	n, nx, err := lockProduct(ctx, q, shopID, productID, vendor)
	if err == nil && n >= maxImages {
		err = badField("images", "max")
	}
	if next != nil {
		*next = nx
	}
	return err
}

func insertImage(ctx context.Context, tx *sql.Tx, shopID, productID int64, key, alt string, w, h, sort int) (int64, json.RawMessage, json.RawMessage, error) {
	var id int64
	var row json.RawMessage
	err := tx.QueryRowContext(ctx, `INSERT INTO product_images (shop_id, product_id, storage_key, alt, width, height, sort_order)
		VALUES ($1, $2, $3, $4::jsonb, $5, $6, $7) RETURNING id, to_jsonb(product_images.*) - 'shop_id'`,
		shopID, productID, key, alt, w, h, sort).Scan(&id, &row)
	return id, nil, row, err
}

// orderImages: ids = product er shob chobi (duplicate/baki/onno na), sort_order = index. Product age lock.
func orderImages(ctx context.Context, tx *sql.Tx, shopID, productID int64, ids []int64) error {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM product_images WHERE shop_id = $1 AND product_id = $2`, shopID, productID)
	if err != nil {
		return err
	}
	have := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		have[id] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	seen := map[int64]bool{}
	for _, id := range ids {
		if !have[id] || seen[id] {
			return badField("ids", "invalid")
		}
		seen[id] = true
	}
	if len(ids) == 0 || len(seen) != len(have) {
		return badField("ids", "invalid")
	}
	for i, id := range ids {
		if _, err := tx.ExecContext(ctx, `UPDATE product_images SET sort_order = $3 WHERE shop_id = $1 AND id = $2`, shopID, id, i); err != nil {
			return err
		}
	}
	return nil
}

// deleteImage: vendor scope product theke; nai → sql.ErrNoRows (404).
func deleteImage(ctx context.Context, tx *sql.Tx, shopID, id int64, vendor any) (string, json.RawMessage, error) {
	var key string
	var before json.RawMessage
	err := tx.QueryRowContext(ctx, `DELETE FROM product_images i USING products p
		WHERE i.shop_id = $1 AND i.id = $2 AND p.shop_id = i.shop_id AND p.id = i.product_id AND ($3::bigint IS NULL OR p.vendor_id = $3)
		RETURNING i.storage_key, to_jsonb(i.*) - 'shop_id'`, shopID, id, vendor).Scan(&key, &before)
	return key, before, err
}
