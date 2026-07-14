// Copyright (c) 2026 Richard Rodger and other contributors, MIT License

// grammar_test.go is the Go port of ts/test/grammar.test.js: grammar-driven
// extraction + rendering, validated against the Go @tabnas/json plugin.
package tabnasrailroad

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	jsonplugin "github.com/tabnas/json/go"
	tabnas "github.com/tabnas/parser/go"
)

// build constructs a Tabnas instance with the json grammar and the railroad
// plugin loaded, returning its railroad API.
func build(t *testing.T) *RailroadApi {
	t.Helper()
	tn := tabnas.Make()
	if err := jsonplugin.Json(tn, nil); err != nil {
		t.Fatalf("json plugin: %v", err)
	}
	if err := Plugin(tn, nil); err != nil {
		t.Fatalf("railroad plugin: %v", err)
	}
	return Of(tn)
}

func TestExtractsRuleSetAndStart(t *testing.T) {
	model := build(t).ToJson()
	if model.Start != "val" {
		t.Errorf("start = %q, want val", model.Start)
	}
	got := make([]string, 0, len(model.Rules))
	for k := range model.Rules {
		got = append(got, k)
	}
	sort.Strings(got)
	want := []string{"elem", "list", "map", "pair", "val"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("rules = %v, want %v", got, want)
	}
	if model.Meta["engine"] != "tabnas" {
		t.Errorf("meta.engine = %v, want tabnas", model.Meta["engine"])
	}
}

func TestValIsChoiceMapListVAL(t *testing.T) {
	val := build(t).ToJson().Rules["val"]
	if val.Kind != KindChoice {
		t.Fatalf("val.kind = %q, want choice", val.Kind)
	}
	hasMap, hasList, hasVAL := false, false, false
	for _, it := range val.Items {
		if it.Kind == KindNonTerminal && it.Text == "map" {
			hasMap = true
		}
		if it.Kind == KindNonTerminal && it.Text == "list" {
			hasList = true
		}
		if it.Kind == KindTerminal && it.Text == "VAL" {
			hasVAL = true
		}
	}
	if !hasMap || !hasList || !hasVAL {
		t.Errorf("val items missing map/list/VAL: map=%v list=%v VAL=%v", hasMap, hasList, hasVAL)
	}
}

func TestMapShape(t *testing.T) {
	m := build(t).ToJson().Rules["map"]
	if m.Kind != KindSeq {
		t.Fatalf("map.kind = %q, want seq", m.Kind)
	}
	if !NodeEqual(m.Items[0], Terminal("{")) {
		t.Errorf("map.items[0] = %+v, want terminal {", m.Items[0])
	}
	if m.Items[1].Kind != KindOptional {
		t.Errorf("map.items[1].kind = %q, want optional", m.Items[1].Kind)
	}
	if !NodeEqual(m.Items[1].Item, NonTerminal("pair")) {
		t.Errorf("map.items[1].item = %+v, want nonterminal pair", m.Items[1].Item)
	}
	if !NodeEqual(m.Items[2], Terminal("}")) {
		t.Errorf("map.items[2] = %+v, want terminal }", m.Items[2])
	}
}

func TestListShape(t *testing.T) {
	l := build(t).ToJson().Rules["list"]
	if l.Kind != KindSeq {
		t.Fatalf("list.kind = %q, want seq", l.Kind)
	}
	if !NodeEqual(l.Items[0], Terminal("[")) {
		t.Errorf("list.items[0] = %+v, want terminal [", l.Items[0])
	}
	if l.Items[1].Kind != KindOptional {
		t.Errorf("list.items[1].kind = %q, want optional", l.Items[1].Kind)
	}
	if !NodeEqual(l.Items[1].Item, NonTerminal("elem")) {
		t.Errorf("list.items[1].item = %+v, want nonterminal elem", l.Items[1].Item)
	}
	if !NodeEqual(l.Items[2], Terminal("]")) {
		t.Errorf("list.items[2] = %+v, want terminal ]", l.Items[2])
	}
}

func TestPairShape(t *testing.T) {
	p := build(t).ToJson().Rules["pair"]
	if p.Kind != KindOneOrMore {
		t.Fatalf("pair.kind = %q, want oneOrMore", p.Kind)
	}
	if !NodeEqual(p.Rep, Terminal(",")) {
		t.Errorf("pair.rep = %+v, want terminal ,", p.Rep)
	}
	if p.Item.Kind != KindSeq {
		t.Fatalf("pair.item.kind = %q, want seq", p.Item.Kind)
	}
	if !NodeEqual(p.Item.Items[0], Terminal("KEY")) {
		t.Errorf("pair.item.items[0] = %+v, want terminal KEY", p.Item.Items[0])
	}
	if !NodeEqual(p.Item.Items[1], Terminal(":")) {
		t.Errorf("pair.item.items[1] = %+v, want terminal :", p.Item.Items[1])
	}
	if !NodeEqual(p.Item.Items[2], NonTerminal("val")) {
		t.Errorf("pair.item.items[2] = %+v, want nonterminal val", p.Item.Items[2])
	}
}

func TestElemShape(t *testing.T) {
	e := build(t).ToJson().Rules["elem"]
	if e.Kind != KindOneOrMore {
		t.Fatalf("elem.kind = %q, want oneOrMore", e.Kind)
	}
	if !NodeEqual(e.Item, NonTerminal("val")) {
		t.Errorf("elem.item = %+v, want nonterminal val", e.Item)
	}
	if !NodeEqual(e.Rep, Terminal(",")) {
		t.Errorf("elem.rep = %+v, want terminal ,", e.Rep)
	}
}

func TestExtractGrammarBareMatchesApi(t *testing.T) {
	tn := tabnas.Make()
	if err := jsonplugin.Json(tn, nil); err != nil {
		t.Fatal(err)
	}
	if err := Plugin(tn, nil); err != nil {
		t.Fatal(err)
	}
	bare := ExtractGrammar(tn)
	api := Of(tn).ToJson()
	bj, _ := json.Marshal(bare)
	aj, _ := json.Marshal(api)
	if string(bj) != string(aj) {
		t.Errorf("ExtractGrammar != api.ToJson:\n%s\n%s", bj, aj)
	}
}

// ---- whole-grammar rendering --------------------------------------

func TestSvgWellFormedAnchoredLinked(t *testing.T) {
	svg, err := build(t).ToSvg()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(svg, "<svg ") {
		t.Errorf("svg should start with <svg ")
	}
	if !strings.HasSuffix(svg, "</svg>") {
		t.Errorf("svg should end with </svg>")
	}
	w := attrInt(t, svg, "width")
	h := attrInt(t, svg, "height")
	if w <= 0 || h <= 0 {
		t.Errorf("width/height must be positive, got %d/%d", w, h)
	}
	if h <= w {
		t.Errorf("expected a vertically-biased (taller-than-wide) diagram, got %dx%d", w, h)
	}
	for _, r := range []string{"val", "map", "list", "pair", "elem"} {
		if !strings.Contains(svg, `id="`+r+`"`) {
			t.Errorf("missing track anchor id=%q", r)
		}
	}
	if !strings.Contains(svg, `<a href="#map"`) {
		t.Errorf("nonterminal map should link")
	}
	if !strings.Contains(svg, `<a href="#val"`) {
		t.Errorf("nonterminal val should link")
	}
}

func TestAsciiContainsEveryRuleName(t *testing.T) {
	ascii, err := build(t).ToAscii(AsciiOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range []string{"val", "map", "list", "pair", "elem"} {
		if !strings.Contains(ascii, r+":") {
			t.Errorf("missing rule heading %s:", r)
		}
	}
}

func TestAsciiPlainIsPureAscii(t *testing.T) {
	ascii, err := build(t).ToAscii(AsciiOptions{Plain: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range ascii {
		if r > 127 {
			t.Fatalf("expected pure ASCII output, found rune %q", r)
		}
	}
}

func TestModelRoundTripsToSameOutput(t *testing.T) {
	model := build(t).ToJson()
	data, err := json.Marshal(model)
	if err != nil {
		t.Fatal(err)
	}
	var clone GrammarModel
	if err := json.Unmarshal(data, &clone); err != nil {
		t.Fatal(err)
	}
	svg1, _ := ModelToSvg(model)
	svg2, _ := ModelToSvg(&clone)
	if svg1 != svg2 {
		t.Errorf("SVG differs after JSON round-trip")
	}
	a1, _ := ModelToAscii(model)
	a2, _ := ModelToAscii(&clone)
	if a1 != a2 {
		t.Errorf("ASCII differs after JSON round-trip")
	}
}

func TestEmitsTokenLegend(t *testing.T) {
	model := build(t).ToJson()
	if len(model.Legend) == 0 {
		t.Fatal("model should carry a token legend")
	}
	meaning := map[string]string{}
	for _, e := range model.Legend {
		meaning[e.Token] = e.Meaning
	}
	if _, ok := meaning["KEY"]; !ok {
		t.Errorf("legend should explain KEY")
	}
	if _, ok := meaning["VAL"]; !ok {
		t.Errorf("legend should explain VAL")
	}
	if !strings.Contains(meaning["VAL"], "value") {
		t.Errorf("VAL meaning should mention value, got %q", meaning["VAL"])
	}
	ascii, _ := ModelToAscii(model)
	if !strings.Contains(ascii, "Tokens:") {
		t.Errorf("ASCII should include a Tokens key")
	}
	svg, _ := ModelToSvg(model)
	if !strings.Contains(svg, ">Tokens<") {
		t.Errorf("SVG should include a Tokens key")
	}
}

func TestReportsIgnoredTokens(t *testing.T) {
	model := build(t).ToJson()
	if len(model.Ignored) == 0 {
		t.Fatal("model should carry the ignored-token set")
	}
	tokens := map[string]bool{}
	for _, e := range model.Ignored {
		tokens[e.Token] = true
		if e.Meaning == "" {
			t.Errorf("every ignored token should carry a meaning, %q is empty", e.Token)
		}
	}
	if !tokens["SP"] {
		t.Errorf("SP should be reported as ignored")
	}
	if !tokens["LN"] {
		t.Errorf("LN should be reported as ignored")
	}
	ascii, _ := ModelToAscii(model)
	if !strings.Contains(ascii, "Ignored tokens:") {
		t.Errorf("ASCII should include an Ignored tokens key")
	}
	svg, _ := ModelToSvg(model)
	if !strings.Contains(svg, ">Ignored tokens<") {
		t.Errorf("SVG should include an Ignored tokens key")
	}
}

// attrInt extracts an integer attribute value from an SVG string.
func attrInt(t *testing.T, svg, name string) int {
	t.Helper()
	key := name + `="`
	i := strings.Index(svg, key)
	if i < 0 {
		t.Fatalf("attribute %q not found", name)
	}
	i += len(key)
	j := i
	for j < len(svg) && svg[j] >= '0' && svg[j] <= '9' {
		j++
	}
	n := 0
	for _, c := range svg[i:j] {
		n = n*10 + int(c-'0')
	}
	return n
}
