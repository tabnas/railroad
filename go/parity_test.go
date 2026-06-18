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
	"os"
	"reflect"
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
