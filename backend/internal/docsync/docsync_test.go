package docsync

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// v1.6: design docs/design/ e section onujayi alada file; schema, phone, pricing, order er code
// file-i asol, doc e shudhu link. Test gulo dekhe:
//  1. protiti file Claude Code er Read tool er ek call e puro ashe (maxBytes);
//  2. boro code block abar doc e dhoke na (maxCodeLines): code repo te file, doc e link;
//  3. doc er relative link er file ache (6.7, 8.1, 8.5, 8.8 er code o);
//  4. protiti "## N." ar "### N.M" ek bar-i, ar N er file e (NN-*.md);
//  5. README.md ar AGENTS.md protiti section file er naam bole, ar AGENTS.md er map thik file dekhay.
const (
	repoRoot     = "../../.."
	maxBytes     = 60_000 // Read: ek call e 25,000 token; Banglish e 60 KB er niche nirapod
	maxCodeLines = 80
)

var designDir = filepath.Join(repoRoot, "docs", "design")

type block struct {
	lang  string
	start int // 1 theke line number
	lines int
}

type parsed struct {
	headings []string // fence er baire "## " ba "### "
	blocks   []block
	links    []string // fence er baire markdown link er target
}

var linkRE = regexp.MustCompile(`\]\(([^)\s]+)\)`)

// parse: ``` fence er bhetore heading ba link gona hoy na; CRLF chole; bondho na hoya fence e fail.
func parse(t *testing.T, name, text string) parsed {
	t.Helper()
	var p parsed
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		if strings.HasPrefix(l, "```") {
			b := block{lang: strings.TrimSpace(strings.TrimPrefix(l, "```")), start: i + 1}
			for i++; i < len(lines) && strings.TrimRight(lines[i], " \t") != "```"; i++ {
				b.lines++
			}
			if i >= len(lines) {
				t.Fatalf("%s er %d number line er ``` block bondho hoyni", name, b.start)
			}
			p.blocks = append(p.blocks, b)
			continue
		}
		if strings.HasPrefix(l, "## ") || strings.HasPrefix(l, "### ") {
			p.headings = append(p.headings, l)
		}
		for _, m := range linkRE.FindAllStringSubmatch(l, -1) {
			p.links = append(p.links, m[1])
		}
	}
	return p
}

func designFiles(t *testing.T) map[string]string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(designDir, "*.md"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("docs/design e kono .md file nai (%v)", err)
	}
	out := map[string]string{}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		out[filepath.Base(p)] = string(b)
	}
	if _, ok := out["README.md"]; !ok {
		t.Fatal("docs/design/README.md (suchipotro ar version) nai")
	}
	return out
}

func TestFileSize(t *testing.T) {
	for name, text := range designFiles(t) {
		if len(text) > maxBytes {
			t.Errorf("%s %d byte, shima %d: section bhag koro ba code file e shoriye link dao", name, len(text), maxBytes)
		}
	}
}

func TestNoLargeCodeBlocks(t *testing.T) {
	for name, text := range designFiles(t) {
		for _, b := range parse(t, name, text).blocks {
			if b.lines > maxCodeLines {
				t.Errorf("%s er %d number line e %d line er code block (%s): code repo te file hishebe rakho, doc e link", name, b.start, b.lines, b.lang)
			}
		}
	}
}

func TestRelativeLinksExist(t *testing.T) {
	for name, text := range designFiles(t) {
		for _, l := range parse(t, name, text).links {
			if strings.Contains(l, "://") || strings.HasPrefix(l, "mailto:") || strings.HasPrefix(l, "#") {
				continue
			}
			target, _, _ := strings.Cut(l, "#")
			if _, err := os.Stat(filepath.Join(designDir, filepath.FromSlash(target))); err != nil {
				t.Errorf("%s: link %q er file nai", name, l)
			}
		}
	}
}

var secRE = regexp.MustCompile(`^#{2,3} (\d+)\.(\d*)`)

func TestSectionsInTheirFile(t *testing.T) {
	seen := map[string]string{}
	for name, text := range designFiles(t) {
		for _, h := range parse(t, name, text).headings {
			m := secRE.FindStringSubmatch(h)
			if m == nil {
				continue
			}
			key := m[1] + "." + m[2]
			if prev, ok := seen[key]; ok {
				t.Errorf("section %s duibar: %s ar %s", key, prev, name)
			}
			seen[key] = name
			n, _ := strconv.Atoi(m[1])
			if want := fmt.Sprintf("%02d-", n); !strings.HasPrefix(name, want) {
				t.Errorf("%q ache %s e, thakbe %s*.md e", h, name, want)
			}
		}
	}
	for n := 1; n <= 16; n++ {
		if _, ok := seen[fmt.Sprintf("%d.", n)]; !ok {
			t.Errorf("\"## %d.\" section kono file e nai", n)
		}
	}
}

var agentsRowRE = regexp.MustCompile(`^\|\s*(\d+)\b.*docs/design/(\d{2})-`)

func TestIndexes(t *testing.T) {
	files := designFiles(t)
	raw, err := os.ReadFile(filepath.Join(repoRoot, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	agents := string(raw)
	for name := range files {
		if name == "README.md" {
			continue
		}
		if !strings.Contains(files["README.md"], "("+name+")") {
			t.Errorf("README.md er suchipotro te %s er link nai", name)
		}
		if !strings.Contains(agents, "docs/design/"+name) {
			t.Errorf("AGENTS.md er section map e docs/design/%s nai", name)
		}
	}
	for _, l := range strings.Split(agents, "\n") {
		if m := agentsRowRE.FindStringSubmatch(l); m != nil {
			sec, _ := strconv.Atoi(m[1])
			file, _ := strconv.Atoi(m[2])
			if sec != file {
				t.Errorf("AGENTS.md er map: section %d ke %02d-*.md dekhay: %s", sec, file, l)
			}
		}
	}
}

func TestParser(t *testing.T) {
	doc := "## 1. X\r\n```go\r\n## heading na\r\n[a](b.md)\r\n```  \r\n### 1.2 Y\n[c](d.md#x) [e](https://x.y)\n```mermaid\nm\n```\n"
	p := parse(t, "test", doc)
	if len(p.headings) != 2 || p.headings[1] != "### 1.2 Y" {
		t.Errorf("headings: %q", p.headings)
	}
	if len(p.blocks) != 2 || p.blocks[0].lines != 2 || p.blocks[1].lang != "mermaid" || p.blocks[1].start != 8 {
		t.Errorf("blocks: %+v", p.blocks)
	}
	if len(p.links) != 2 || p.links[0] != "d.md#x" || p.links[1] != "https://x.y" {
		t.Errorf("links: %q", p.links)
	}
}
