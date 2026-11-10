package docsync

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// v1.8 (4.7): feature doc, backend er code map, package doc ar docs/STATUS.md er test.
var (
	featuresDir   = filepath.Join(repoRoot, "docs", "features")
	milestonesDir = filepath.Join(repoRoot, "docs", "milestones")
	statusFile    = filepath.Join(repoRoot, "docs", "STATUS.md")
	backendDir    = filepath.Join(repoRoot, "backend")
)

// featureHeadings: protiti feature doc e ei kram e (docs/features/_template.md).
var featureHeadings = []string{"## Ki kore", "## Kivabe kaj kore", "## Code er map", "## Niyom ar tar test",
	"## Data", "## Bhul hole ki hoy", "## Bodlanor age mone rakho", "## Proshno-uttor"}

// statusHeadings: scripts/hooks/session-start.sh prothom tinta pore.
var statusHeadings = []string{"## Ekhon", "## Porer kaj", "## User er kache proshno", "## Milestone", "## Session log"}

const maxFeatureLines = 250

var (
	testNameRE    = regexp.MustCompile(`\bTest[A-Z][A-Za-z0-9_]*`)
	goTestFuncRE  = regexp.MustCompile(`(?m)^func (Test[A-Z][A-Za-z0-9_]*)\(`)
	featureLinkRE = regexp.MustCompile(`docs/features/([a-z0-9-]+\.md)`)
)

func mdFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		out[filepath.Base(p)] = string(b)
	}
	return out
}

func checkLinks(t *testing.T, dir, name string, links []string) {
	t.Helper()
	for _, l := range links {
		if strings.Contains(l, "://") || strings.HasPrefix(l, "mailto:") || strings.HasPrefix(l, "#") {
			continue
		}
		target, _, _ := strings.Cut(l, "#")
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(target))); err != nil {
			t.Errorf("%s: link %q er file nai", name, l)
		}
	}
}

func h2(t *testing.T, name, text string) []string {
	t.Helper()
	var out []string
	for _, h := range parse(t, name, text).headings {
		if strings.HasPrefix(h, "## ") {
			out = append(out, strings.TrimSpace(h))
		}
	}
	return out
}

// inOrder: want er shob heading got e ei kram e ache kina (majhe onno heading thakte pare).
func inOrder(got, want []string) (string, bool) {
	i := 0
	for _, h := range got {
		if i < len(want) && h == want[i] {
			i++
		}
	}
	if i < len(want) {
		return want[i], false
	}
	return "", true
}

func goTestNames(t *testing.T) map[string]bool {
	t.Helper()
	names := map[string]bool{}
	err := filepath.WalkDir(backendDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, "_test.go") {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, m := range goTestFuncRE.FindAllStringSubmatch(string(b), -1) {
			names[m[1]] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return names
}

// goPackageDirs: backend er je folder e test chhara .go file ache (Go package), repo root theke slash path e.
func goPackageDirs(t *testing.T) []string {
	t.Helper()
	set := map[string]bool{}
	err := filepath.WalkDir(backendDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && p != backendDir && (d.Name() == "testdata" || strings.HasPrefix(d.Name(), ".")) {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
			rel, err := filepath.Rel(repoRoot, filepath.Dir(p))
			if err != nil {
				return err
			}
			set[filepath.ToSlash(rel)] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	dirs := make([]string, 0, len(set))
	for d := range set {
		dirs = append(dirs, d)
	}
	slices.Sort(dirs)
	return dirs
}

// TestFeatureDocs (4.7): template ar protiti feature doc er heading kram e, 250 line er kom, link er file ache,
// doc e lekha Go test er naam code e ache (rename hole doc o), ar index e doc er link.
func TestFeatureDocs(t *testing.T) {
	docs := mdFiles(t, featuresDir)
	index, ok := docs["README.md"]
	if !ok {
		t.Fatal("docs/features/README.md (index ar code map) nai")
	}
	tmpl, ok := docs["_template.md"]
	if !ok {
		t.Fatal("docs/features/_template.md nai")
	}
	if missing, ok := inOrder(h2(t, "_template.md", tmpl), featureHeadings); !ok {
		t.Errorf("_template.md e %q nai", missing)
	}
	tests := goTestNames(t)
	for name, text := range docs {
		p := parse(t, name, text)
		checkLinks(t, featuresDir, name, p.links)
		if len(text) > maxBytes {
			t.Errorf("%s %d byte, shima %d", name, len(text), maxBytes)
		}
		for _, b := range p.blocks {
			if b.lines > maxCodeLines {
				t.Errorf("%s er %d number line e %d line er code block: code repo te, doc e link", name, b.start, b.lines)
			}
		}
		if name == "README.md" || name == "_template.md" {
			continue
		}
		if missing, ok := inOrder(h2(t, name, text), featureHeadings); !ok {
			t.Errorf("%s: %q heading nai ba kram e nai (_template.md er moto koro)", name, missing)
		}
		if n := strings.Count(text, "\n"); n > maxFeatureLines {
			t.Errorf("%s %d line, shima %d: choto koro, code copy na", name, n, maxFeatureLines)
		}
		if !strings.Contains(index, "("+name+")") {
			t.Errorf("docs/features/README.md er index e %s er link nai", name)
		}
		for _, tn := range testNameRE.FindAllString(text, -1) {
			if !tests[tn] {
				t.Errorf("%s: %s naam er Go test nai (test rename hole doc o bodlao)", name, tn)
			}
		}
	}
}

// TestCodeMap (4.7): backend er protiti Go package docs/features/README.md er code map e.
func TestCodeMap(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(featuresDir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range goPackageDirs(t) {
		if !strings.Contains(string(b), "`"+dir+"`") {
			t.Errorf("docs/features/README.md er code map e `%s` nai", dir)
		}
	}
}

// TestPackageDocs (4.7): protiti package er kono (test na) file er package comment e
// "docs/features/<naam>.md", ar sei doc ache.
func TestPackageDocs(t *testing.T) {
	for _, dir := range goPackageDirs(t) {
		abs := filepath.Join(repoRoot, filepath.FromSlash(dir))
		entries, err := os.ReadDir(abs)
		if err != nil {
			t.Fatal(err)
		}
		var links []string
		for _, e := range entries {
			n := e.Name()
			if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
				continue
			}
			f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(abs, n), nil, parser.PackageClauseOnly|parser.ParseComments)
			if err != nil {
				t.Fatal(err)
			}
			if f.Doc != nil {
				for _, m := range featureLinkRE.FindAllStringSubmatch(f.Doc.Text(), -1) {
					links = append(links, m[1])
				}
			}
		}
		if len(links) == 0 {
			t.Errorf("%s: package comment e feature doc er link nai (\"Feature doc: docs/features/<naam>.md\")", dir)
		}
		for _, l := range links {
			if _, err := os.Stat(filepath.Join(featuresDir, l)); err != nil {
				t.Errorf("%s: package comment er docs/features/%s nai", dir, l)
			}
		}
	}
}

// TestStatusFile (4.7): docs/STATUS.md er heading (SessionStart hook pore), ar STATUS o milestone file er link.
func TestStatusFile(t *testing.T) {
	b, err := os.ReadFile(statusFile)
	if err != nil {
		t.Fatal("docs/STATUS.md nai: Claude Code er tracking file (AGENTS.md)")
	}
	text := string(b)
	got := h2(t, "STATUS.md", text)
	for _, h := range statusHeadings {
		if !slices.Contains(got, h) {
			t.Errorf("docs/STATUS.md e %q heading nai (scripts/hooks/session-start.sh eta pore)", h)
		}
	}
	checkLinks(t, filepath.Dir(statusFile), "STATUS.md", parse(t, "STATUS.md", text).links)
	for name, mtext := range mdFiles(t, milestonesDir) {
		checkLinks(t, milestonesDir, "milestones/"+name, parse(t, name, mtext).links)
	}
}
