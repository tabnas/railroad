// Copyright (c) 2026 Richard Rodger and other contributors, MIT License

package tabnasrailroad

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"testing"
)

// TestParityWithTypeScriptModel compares this port's grammar model against
// the committed snapshot of the TypeScript one, which is the contract
// AGENTS.md states: "same start, same per-rule node trees, same legend,
// same ignored set, same meta.engine".
//
// Three files said this test did that. It did not exist, and
// go/testdata/ts-json-model.json — the snapshot they name — was read by
// nothing: `grep -rn testdata go/*_test.go` returned empty. So the
// cross-language contract was pinned by nothing at all while
// AGENTS.md:45, AGENTS.md:254 and test/AGENTS.md:52 each said it was.
//
// Compared by VALUE after a JSON round-trip, not by bytes: JSON object
// key order carries no meaning, and asserting it would fail on a
// difference that is not one. Rules are compared by NAME, so this does
// not assert declaration order — that is
// TestExtractionHonoursDeclarationOrder's job, as AGENTS.md:254 says.
func TestParityWithTypeScriptModel(t *testing.T) {
	raw, err := json.Marshal(build(t).ToJson())
	if err != nil {
		t.Fatalf("marshal model: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("round-trip model: %v", err)
	}

	gold, err := os.ReadFile("testdata/ts-json-model.json")
	if err != nil {
		t.Fatalf("read the TypeScript snapshot: %v", err)
	}
	var want map[string]any
	if err := json.Unmarshal(gold, &want); err != nil {
		t.Fatalf("parse the TypeScript snapshot: %v", err)
	}

	// Every field the contract names, including the ones that agree
	// today: a test that only checks what once differed stops covering
	// the rest the moment it agrees.
	for _, field := range []string{"start", "legend", "ignored"} {
		if !reflect.DeepEqual(got[field], want[field]) {
			t.Errorf("%s differs from the TypeScript model:\n go: %#v\n ts: %#v",
				field, got[field], want[field])
		}
	}

	// meta.ENGINE, not the whole meta map. go/doc/concepts.md:125 states
	// the contract as "same start, same per-rule node trees, same legend,
	// same ignored set, same meta.engine", and GrammarModel.Meta is
	// map[string]any — deliberately open-ended. Comparing all of it would
	// fail on a runtime adding its own metadata while the documented
	// contract still held, which is a gate breaking over something it was
	// never asked to police.
	if engineOf(got["meta"]) != engineOf(want["meta"]) {
		t.Errorf("meta.engine differs from the TypeScript model:\n go: %#v\n ts: %#v",
			engineOf(got["meta"]), engineOf(want["meta"]))
	}
	if nil == engineOf(want["meta"]) {
		t.Error("sanity: the snapshot carries no meta.engine, so comparing " +
			"it against this port's proves nothing")
	}

	gotRules, ok := got["rules"].(map[string]any)
	if !ok {
		t.Fatal("this port emitted no rules map, so nothing below compares")
	}
	wantRules, ok := want["rules"].(map[string]any)
	if !ok {
		t.Fatal("the snapshot holds no rules map, so nothing below compares")
	}
	if 0 == len(wantRules) {
		t.Fatal("sanity: the snapshot has no rules, so comparing against " +
			"it would pass whatever this port produced")
	}

	if !reflect.DeepEqual(names(gotRules), names(wantRules)) {
		t.Fatalf("rule names differ:\n go: %v\n ts: %v",
			names(gotRules), names(wantRules))
	}
	for _, name := range names(wantRules) {
		if !reflect.DeepEqual(gotRules[name], wantRules[name]) {
			g, _ := json.Marshal(gotRules[name])
			w, _ := json.Marshal(wantRules[name])
			t.Errorf("rule %q node tree differs:\n go: %s\n ts: %s", name, g, w)
		}
	}
}

func engineOf(meta any) any {
	m, ok := meta.(map[string]any)
	if !ok {
		return nil
	}
	return m["engine"]
}

func names(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
