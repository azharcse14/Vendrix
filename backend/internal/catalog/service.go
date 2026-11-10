package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Text: {"bn": "...", "en": "..."} (7.1). Public API ek bhasha e resolve kore, admin puro object pay.
type Text map[string]string

// In: locale er lekha; khali hole bn (fallback, 7.1).
func (t Text) In(locale string) string {
	if s := t[locale]; s != "" {
		return s
	}
	return t["bn"]
}

// valid: bn lagbe; shudhu bn/en key.
func (t Text) valid() bool { return t.keysOK() && strings.TrimSpace(t["bn"]) != "" }

// keysOK: shudhu bn/en key (description er moto lekha, bn khali hote pare).
func (t Text) keysOK() bool {
	for k := range t {
		if k != "bn" && k != "en" {
			return false
		}
	}
	return true
}

var slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// validSlug (7.7, v1.10): Latin, chhoto haat, hyphen; max 80.
func validSlug(s string) bool { return len(s) <= 80 && slugRe.MatchString(s) }

// searchTerm (6.4): q 2 okkhorer kom hole search na (ok false). like = ILIKE er jonno `\ % _` escape
// (pattern er SQL e '%' || like || '%'), raw = word_similarity er asol lekha. NFC SQL e.
func searchTerm(q string) (like, raw string, ok bool) {
	raw = strings.TrimSpace(q)
	if utf8.RuneCountInString(raw) < 2 {
		return "", "", false
	}
	like = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(raw)
	return like, raw, true
}

// makesCycle (7.7): id er parent newParent hole cycle hoy kina. parents = shop er shob category er
// id → parent_id (0 = root). DB shudhu nijer parent atkay (categories_not_own_parent_check).
func makesCycle(parents map[int64]int64, id, newParent int64) bool {
	for p, n := newParent, 0; p != 0 && n <= len(parents); p, n = parents[p], n+1 {
		if p == id {
			return true
		}
	}
	return false
}

// changeCategory (7.7): parent bodlale shop er category lock kore cycle check, tarpor update.
func changeCategory(ctx context.Context, tx *sql.Tx, shopID, id int64, in categoryInput) (int64, json.RawMessage, json.RawMessage, error) {
	if in.ParentID.Set && in.ParentID.V != nil {
		parents, err := lockCategoryParents(ctx, tx, shopID)
		if err != nil {
			return 0, nil, nil, err
		}
		if _, ok := parents[id]; !ok {
			return 0, nil, nil, errNotFound
		}
		if makesCycle(parents, id, *in.ParentID.V) {
			return 0, nil, nil, badField("parent_id", "cycle")
		}
	}
	return updateCategory(ctx, tx, shopID, id, in)
}

// productVendor (Rule 13, v1.10): vendor staff e shob shomoy nijer vendor (body te onno vendor → 404);
// shop staff e body er vendor ba nil (store house vendor ney).
func productVendor(staffVendor, body *int64) (*int64, error) {
	if staffVendor == nil {
		return body, nil
	}
	if body != nil && *body != *staffVendor {
		return nil, errNotFound
	}
	return staffVendor, nil
}
