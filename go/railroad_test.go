// Copyright (c) 2026 Richard Rodger and other contributors, MIT License

// railroad_test.go is the Go port of ts/test/railroad.test.js: node-level
// model construction, the text emitter, the single-node SVG/ASCII
// renderers, the plugin decoration, and error cases.
package tabnasrailroad

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	tabnas "github.com/tabnas/parser/go"
)

// ---- plugin load --------------------------------------------------

func TestPluginDecoratesInstance(t *testing.T) {
	tn := tabnas.Make()
	if Of(tn) == nil {
		t.Fatal("Of should never return nil")
	}
	// Before loading, the decoration is absent.
	if tn.Decoration(DecorationName) != nil {
		t.Fatal("railroad decoration should be absent before Plugin")
	}
	if err := Plugin(tn, nil); err != nil {
		t.Fatal(err)
	}
	api, ok := tn.Decoration(DecorationName).(*RailroadApi)
	if !ok || api == nil {
		t.Fatal("railroad decoration should be a *RailroadApi after Plugin")
	}
	// The API exposes the render helpers.
	if _, err := api.RenderNodeText(Terminal("x")); err != nil {
		t.Errorf("RenderNodeText: %v", err)
	}
}

func TestChildInstancesInheritDecoration(t *testing.T) {
	tn := tabnas.Make()
	if err := Plugin(tn, nil); err != nil {
		t.Fatal(err)
	}
	child, err := tn.Derive()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := child.Decoration(DecorationName).(*RailroadApi); !ok {
		t.Error("derived instance should inherit the railroad decoration")
	}
}

// ---- text emitter -------------------------------------------------

func TestTextEmitter(t *testing.T) {
	cases := []struct {
		node *RailroadNode
		want string
	}{
		{Terminal("hi"), `"hi"`},
		{NonTerminal("expr"), "expr"},
		{Sequence(Terminal("a"), Terminal("b")), `"a" "b"`},
		{MustChoice(Terminal("a"), Terminal("b")), `("a" | "b")`},
		{Optional(Terminal("a")), `["a"]`},
		{OneOrMore(Terminal("a"), nil), `"a"+`},
		{ZeroOrMore(Terminal("a"), nil), `{"a"}`},
	}
	for _, tc := range cases {
		got, err := ToText(tc.node)
		if err != nil {
			t.Errorf("ToText(%+v): %v", tc.node, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ToText = %q, want %q", got, tc.want)
		}
	}
}

// ---- svg node renderer --------------------------------------------

func TestSvgNodeWellFormed(t *testing.T) {
	svg, err := RenderNodeSvg(Diagram(Sequence(Terminal("GET"), NonTerminal("path"))))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(svg, "<svg ") || !strings.HasSuffix(svg, "</svg>") {
		t.Errorf("svg not well-formed: %.40q...", svg)
	}
	w := svgAttr(t, svg, "width")
	h := svgAttr(t, svg, "height")
	if w <= 0 || h <= 0 {
		t.Errorf("width/height must be positive: %d/%d", w, h)
	}
	if !strings.Contains(svg, "GET") || !strings.Contains(svg, "path") || !strings.Contains(svg, "<rect") {
		t.Errorf("svg missing GET/path/<rect")
	}
}

func TestSvgNestedRendersWithoutError(t *testing.T) {
	node := Diagram(Sequence(
		Terminal("["),
		Optional(Sequence(NonTerminal("item"), ZeroOrMore(Sequence(Terminal(","), NonTerminal("item")), nil))),
		Terminal("]")))
	svg, err := RenderNodeSvg(node)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(svg, "<svg ") {
		t.Errorf("svg not well-formed")
	}
}

func TestSvgStacksSequenceWithoutOverlap(t *testing.T) {
	svg, err := RenderNodeSvg(Sequence(Terminal("a"), Terminal("b"), Terminal("c")))
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`<rect[^>]*\sy="([\d.]+)"[^>]*\sheight="([\d.]+)"`)
	matches := re.FindAllStringSubmatch(svg, -1)
	if len(matches) != 3 {
		t.Fatalf("expected 3 rects, got %d", len(matches))
	}
	type band struct{ y, bottom float64 }
	bands := make([]band, len(matches))
	for i, m := range matches {
		y := atof(m[1])
		h := atof(m[2])
		bands[i] = band{y, y + h}
	}
	// Sort by y.
	for i := 1; i < len(bands); i++ {
		for j := i; j > 0 && bands[j].y < bands[j-1].y; j-- {
			bands[j], bands[j-1] = bands[j-1], bands[j]
		}
	}
	for i := 1; i < len(bands); i++ {
		if bands[i].y < bands[i-1].bottom {
			t.Errorf("sequence boxes overlap vertically: %v", bands)
		}
	}
}

// ---- ascii node renderer ------------------------------------------

func TestAsciiNodeSequence(t *testing.T) {
	out, err := RenderNodeAscii(Diagram(Sequence(Terminal("a"), NonTerminal("b"))))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"a"`) || !strings.Contains(out, "b") {
		t.Errorf("ascii missing \"a\"/b:\n%s", out)
	}
}

func TestAsciiNodePlainPureAscii(t *testing.T) {
	out, err := RenderNodeAscii(MustChoice(Terminal("a"), Terminal("b")), AsciiOptions{Plain: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range out {
		if r > 127 {
			t.Fatalf("expected pure ASCII, found rune %q", r)
		}
	}
}

// ---- errors --------------------------------------------------------

func TestChoiceNoBranchesErrors(t *testing.T) {
	_, err := Choice()
	var re *RailroadError
	if !errors.As(err, &re) {
		t.Errorf("Choice() should return a *RailroadError, got %v", err)
	}
}

func TestUnknownNodeKindErrors(t *testing.T) {
	bogus := &RailroadNode{Kind: "bogus"}
	if _, err := RenderNodeSvg(bogus); err == nil {
		t.Errorf("RenderNodeSvg(bogus) should error")
	} else {
		var re *RailroadError
		if !errors.As(err, &re) {
			t.Errorf("expected *RailroadError, got %v", err)
		}
	}
	if _, err := ToText(bogus); err == nil {
		t.Errorf("ToText(bogus) should error")
	}
}

func TestInvalidNodeErrors(t *testing.T) {
	// A nil node is invalid (the Go analog of Sequence(null)).
	if _, err := norm(nil); err == nil {
		t.Errorf("norm(nil) should error")
	}
	if _, err := ToText(nil); err == nil {
		t.Errorf("ToText(nil) should error")
	}
}

// ---- helpers -------------------------------------------------------

func svgAttr(t *testing.T, svg, name string) int {
	t.Helper()
	key := name + `="`
	i := strings.Index(svg, key)
	if i < 0 {
		t.Fatalf("attr %q not found", name)
	}
	i += len(key)
	n := 0
	for i < len(svg) && svg[i] >= '0' && svg[i] <= '9' {
		n = n*10 + int(svg[i]-'0')
		i++
	}
	return n
}

func atof(s string) float64 {
	var whole, frac float64
	var div float64 = 1
	dot := false
	for _, c := range s {
		if c == '.' {
			dot = true
			continue
		}
		d := float64(c - '0')
		if dot {
			div *= 10
			frac += d / div
		} else {
			whole = whole*10 + d
		}
	}
	return whole + frac
}
