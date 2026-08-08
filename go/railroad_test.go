// Copyright (c) 2026 Richard Rodger and other contributors, MIT License

// railroad_test.go is the Go port of ts/test/railroad.test.js: node-level
// model construction, the text emitter, the single-node SVG/ASCII
// renderers, the plugin decoration, and error cases.
package tabnasrailroad

import (
	"errors"
	"reflect"
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

// ---- whole-model rule ordering --------------------------------------

// The renderers must emit rules in the model's declared RuleOrder, hoisting
// Start to the front — NOT in alphabetical or Go map order. This pins the
// renderer half with a hand-built model whose order is deliberately neither
// alphabetical nor start-first; TestExtractionHonoursDeclarationOrder below
// pins the extraction half.
func TestRenderersHonourDeclaredRuleOrder(t *testing.T) {
	mk := func(s string) *RailroadNode {
		n, err := Norm(s)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	model := &GrammarModel{
		Start: "mid",
		Rules: map[string]*RailroadNode{
			"zebra": mk("z"),
			"alpha": mk("a"),
			"mid":   mk("m"),
		},
		// Neither alphabetical nor start-first.
		RuleOrder: []string{"zebra", "alpha", "mid"},
	}
	want := []string{"mid", "zebra", "alpha"} // Start hoisted, rest in order.

	ascii, err := ModelToAscii(model)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, line := range strings.Split(ascii, "\n") {
		if strings.HasSuffix(line, ":") && !strings.ContainsAny(line, " \t") {
			got = append(got, strings.TrimSuffix(line, ":"))
		}
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("ascii rule order = %v, want %v", got, want)
	}

	svg, err := ModelToSvg(model)
	if err != nil {
		t.Fatal(err)
	}
	var svgGot []string
	for _, m := range regexp.MustCompile(`<g id="([^"]+)">`).FindAllStringSubmatch(svg, -1) {
		svgGot = append(svgGot, m[1])
	}
	if strings.Join(svgGot, ",") != strings.Join(want, ",") {
		t.Errorf("svg rule order = %v, want %v", svgGot, want)
	}
}

// ExtractGrammar must put rules in the model in the order the GRAMMAR
// declared them, matching the TS side's `Object.keys(rsm)` walk — not the
// alphabetical order a Go map walk has to be sorted into. The engine
// reports it via (*Tabnas).RuleNames; extract.go's declaredUserRules is the
// consumer.
//
// A struct grammar states its order in GrammarSpec.RuleOrder (a Go map has
// none to recover); GrammarText fills the same field in from the source key
// order. The rule names here are chosen so declaration order is neither
// alphabetical nor its reverse, so a sort in either direction fails this.
func TestExtractionHonoursDeclarationOrder(t *testing.T) {
	declared := []string{"zebra", "alpha", "mid"}

	tn := tabnas.Make()
	err := tn.Grammar(&tabnas.GrammarSpec{
		V: 2,
		Rule: map[string]*tabnas.GrammarRuleSpec{
			"zebra": {Open: []*tabnas.GrammarAltSpec{{S: "#TX", P: "alpha"}}},
			"alpha": {Open: []*tabnas.GrammarAltSpec{{S: "#TX", P: "mid"}}},
			"mid":   {Open: []*tabnas.GrammarAltSpec{{S: "#TX"}}},
		},
		RuleOrder: declared,
	})
	if err != nil {
		t.Fatal(err)
	}

	model := ExtractGrammar(tn, &ExtractOptions{Start: "mid"})
	if got := model.RuleOrder; !reflect.DeepEqual(got, declared) {
		t.Errorf("RuleOrder = %v, want %v", got, declared)
	}

	// The last-resort entry rule (neither the caller nor cfg.rule.start names
	// one) is the FIRST rule declared, not the alphabetically first — the
	// same rule ts/extract.ts's `Object.keys(rsm).find(isUserRule)` picks.
	if got := firstUserRule(tn); got != "zebra" {
		t.Errorf("firstUserRule = %q, want %q", got, "zebra")
	}

	// The renderers then hoist Start and keep the rest in that order.
	ascii, err := ModelToAscii(model)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, line := range strings.Split(ascii, "\n") {
		name := strings.TrimSuffix(line, ":")
		if strings.HasSuffix(line, ":") && !strings.ContainsAny(line, " \t") &&
			model.Rules[name] != nil {
			got = append(got, name)
		}
	}
	want := []string{"mid", "zebra", "alpha"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ascii rule order = %v, want %v", got, want)
	}
}

// ---- exported Norm / NodeEqual --------------------------------------

func TestNorm(t *testing.T) {
	// A bare string coerces to a Terminal, mirroring the TS norm export.
	n, err := Norm("(")
	if err != nil {
		t.Fatalf("Norm(string) errored: %v", err)
	}
	if !NodeEqual(n, Terminal("(")) {
		t.Errorf("Norm(\"(\") = %+v, want Terminal(\"(\")", n)
	}

	// A valid node passes through unchanged.
	nt := NonTerminal("expr")
	n, err = Norm(nt)
	if err != nil {
		t.Fatalf("Norm(node) errored: %v", err)
	}
	if n != nt {
		t.Errorf("Norm(node) should return the same node")
	}

	// nil, a typed-nil node, an empty-Kind node, and a non-Item value all error.
	for _, bad := range []any{nil, (*RailroadNode)(nil), &RailroadNode{}, 42} {
		if _, err := Norm(bad); err == nil {
			t.Errorf("Norm(%#v) should error", bad)
		} else {
			var re *RailroadError
			if !errors.As(err, &re) {
				t.Errorf("Norm(%#v): expected *RailroadError, got %v", bad, err)
			}
		}
	}
}

func TestNodeEqual(t *testing.T) {
	a := MustChoice(Terminal("a"), Optional(NonTerminal("b")),
		OneOrMore(Terminal("c"), Terminal(",")))
	b := MustChoice(Terminal("a"), Optional(NonTerminal("b")),
		OneOrMore(Terminal("c"), Terminal(",")))
	if !NodeEqual(a, b) {
		t.Errorf("structurally identical trees should be NodeEqual")
	}
	if !NodeEqual(SkipNode(), SkipNode()) {
		t.Errorf("NodeEqual(Skip, Skip) should be true")
	}
	if NodeEqual(Terminal("x"), NonTerminal("x")) {
		t.Errorf("differing kinds should not be NodeEqual")
	}
	if NodeEqual(Terminal("x"), Terminal("y")) {
		t.Errorf("differing text should not be NodeEqual")
	}
	if NodeEqual(a, MustChoice(Terminal("a"))) {
		t.Errorf("differing item counts should not be NodeEqual")
	}
	if NodeEqual(OneOrMore(Terminal("c"), Terminal(",")), OneOrMore(Terminal("c"), nil)) {
		t.Errorf("rep presence mismatch should not be NodeEqual")
	}
	if !NodeEqual(nil, nil) {
		t.Errorf("NodeEqual(nil, nil) should be true")
	}
	if NodeEqual(a, nil) || NodeEqual(nil, a) {
		t.Errorf("nil vs node should not be NodeEqual")
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
