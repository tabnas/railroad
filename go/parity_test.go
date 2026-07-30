// Copyright (c) 2026 Richard Rodger and other contributors, MIT License

// parity_test.go checks cross-language parity: the Go GrammarModel for the
// @tabnas/json grammar must match the reference TypeScript model in
// testdata/ts-json-model.json — same start, same rule node trees, same
// legend, same ignored set. Rule-map key order and JSON key order are not
// part of the contract (the SVG/ASCII tests assert well-formedness, not
// pixel-identity), so the comparison is structural.
package tabnasrailroad

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	jsonplugin "github.com/tabnas/json/go"
	tabnas "github.com/tabnas/parser/go"
)

func TestParityWithTypeScriptModel(t *testing.T) {
	refBytes, err := os.ReadFile("testdata/ts-json-model.json")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	var ref GrammarModel
	if err := json.Unmarshal(refBytes, &ref); err != nil {
		t.Fatalf("parse reference: %v", err)
	}

	tn := tabnas.Make()
	if err := jsonplugin.Json(tn, nil); err != nil {
		t.Fatal(err)
	}
	got := ExtractGrammar(tn)

	if got.Start != ref.Start {
		t.Errorf("start: go=%q ts=%q", got.Start, ref.Start)
	}

	// Same rule names.
	if len(got.Rules) != len(ref.Rules) {
		t.Errorf("rule count: go=%d ts=%d", len(got.Rules), len(ref.Rules))
	}
	for name, refNode := range ref.Rules {
		goNode, ok := got.Rules[name]
		if !ok {
			t.Errorf("go model missing rule %q", name)
			continue
		}
		// Structural equality on the node tree (kind/text/items/item/rep),
		// compared via canonical JSON so the bespoke marshalling shape is
		// the unit of comparison.
		gj, _ := json.Marshal(goNode)
		rj, _ := json.Marshal(refNode)
		if string(gj) != string(rj) {
			t.Errorf("rule %q differs:\n go: %s\n ts: %s", name, gj, rj)
		}
	}

	// Legend (order-independent set of token->meaning).
	if !sameEntries(got.Legend, ref.Legend) {
		t.Errorf("legend differs:\n go: %v\n ts: %v", got.Legend, ref.Legend)
	}
	// Ignored set.
	if !sameEntries(got.Ignored, ref.Ignored) {
		t.Errorf("ignored differs:\n go: %v\n ts: %v", got.Ignored, ref.Ignored)
	}

	// Meta engine.
	if got.Meta["engine"] != ref.Meta["engine"] {
		t.Errorf("meta.engine: go=%v ts=%v", got.Meta["engine"], ref.Meta["engine"])
	}
}

func sameEntries(a, b []LegendEntry) bool {
	am := map[string]string{}
	bm := map[string]string{}
	for _, e := range a {
		am[e.Token] = e.Meaning
	}
	for _, e := range b {
		bm[e.Token] = e.Meaning
	}
	return reflect.DeepEqual(am, bm)
}

// --- shared TSV fixtures ---------------------------------------------------
//
// The rest of this file runs the shared `test/spec/*.tsv` fixtures at the repo
// root (see ../test/AGENTS.md). ts/test/parity.test.js discovers and runs the
// SAME files, so the two renderers cannot drift without one going red.

type specRow struct {
	file     string
	lineNo   int
	node     string
	expected string
	opts     string
}

func specDir() string { return filepath.Join("..", "test", "spec") }

// specUnescape decodes the escape set used in non-JSON columns. Kept
// byte-identical to the TS loader so both runtimes see the same node source.
func specUnescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				b.WriteByte('\n')
				i++
				continue
			case 'r':
				b.WriteByte('\r')
				i++
				continue
			case 't':
				b.WriteByte('\t')
				i++
				continue
			case '\\':
				b.WriteByte('\\')
				i++
				continue
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}

// loadSpec reads one fixture. The header row's SECOND column names the
// renderer to run over the node in the first column ("text" or "ascii").
func loadSpec(t *testing.T, path string) (string, []specRow) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("spec file not found: %s: %v", path, err)
	}
	kind := ""
	var rows []specRow
	for i, line := range strings.Split(string(data), "\n") {
		if i == 0 {
			cols := strings.Split(line, "\t")
			if len(cols) < 2 {
				t.Fatalf("%s: header must name at least 2 columns", path)
			}
			kind = cols[1]
			continue
		}
		// A comment line starts with '#' and has no tab; a data row always
		// has at least one (input + expected), so '#'-leading sources still
		// work.
		if line == "" || (strings.HasPrefix(line, "#") && !strings.Contains(line, "\t")) {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 2 {
			t.Fatalf("%s:%d: expected at least 2 tab-separated columns", path, i+1)
		}
		row := specRow{
			file:     filepath.Base(path),
			lineNo:   i + 1,
			node:     specUnescape(cols[0]),
			expected: cols[1],
		}
		if 3 <= len(cols) {
			row.opts = cols[2]
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		t.Fatalf("%s: no cases", path)
	}
	return kind, rows
}

// renderSpec runs the named renderer. The `opts` column uses the TS spelling
// (`{"ascii":true}`); Go's equivalent field is AsciiOptions.Plain.
func renderSpec(kind string, node *RailroadNode, opts map[string]any) (string, error) {
	switch kind {
	case "text":
		return ToText(node)
	case "ascii":
		plain, _ := opts["ascii"].(bool)
		return RenderNodeAscii(node, AsciiOptions{Plain: plain})
	}
	return "", fmt.Errorf("unknown renderer %q", kind)
}

func specLabel(s string) string {
	if 60 < len(s) {
		return s[:57] + "..."
	}
	return s
}

func runSpecFile(t *testing.T, path string) {
	kind, rows := loadSpec(t, path)
	if kind != "text" && kind != "ascii" {
		t.Fatalf("%s: unknown second column %q", path, kind)
	}
	for _, row := range rows {
		t.Run(specLabel(row.node), func(t *testing.T) {
			// The node column is the node's own JSON shape, which is what both
			// runtimes marshal to and unmarshal from.
			node := &RailroadNode{}
			if err := node.UnmarshalJSON([]byte(row.node)); err != nil {
				t.Fatalf("%s:%d: bad node JSON: %v", row.file, row.lineNo, err)
			}
			opts := map[string]any{}
			if strings.TrimSpace(row.opts) != "" {
				if err := json.Unmarshal([]byte(row.opts), &opts); err != nil {
					t.Fatalf("%s:%d: bad opts JSON %q: %v", row.file, row.lineNo, row.opts, err)
				}
			}

			got, err := renderSpec(kind, node, opts)

			if strings.HasPrefix(row.expected, "ERROR") {
				want := strings.TrimPrefix(strings.TrimPrefix(row.expected, "ERROR"), ":")
				if err == nil {
					t.Fatalf("%s:%d: expected error, got %q", row.file, row.lineNo, got)
				}
				if want != "" && !strings.Contains(err.Error(), want) {
					t.Fatalf("%s:%d: expected error %q, got %q", row.file, row.lineNo, want, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("%s:%d: unexpected error: %v", row.file, row.lineNo, err)
			}

			var want string
			if err := json.Unmarshal([]byte(row.expected), &want); err != nil {
				t.Fatalf("%s:%d: bad expected JSON %q: %v", row.file, row.lineNo, row.expected, err)
			}
			if got != want {
				t.Errorf("%s:%d:\n  got  %q\n  want %q", row.file, row.lineNo, got, want)
			}
		})
	}
}

// TestSpec auto-discovers every fixture: adding a .tsv runs it in both
// runtimes without touching either runner.
func TestSpec(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(specDir(), "*.tsv"))
	if err != nil {
		t.Fatalf("glob spec dir: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no spec files under %s", specDir())
	}
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) { runSpecFile(t, path) })
	}
}
