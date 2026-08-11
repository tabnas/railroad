// Copyright (c) 2026 Richard Rodger and other contributors, MIT License

package tabnasrailroad

// parity_test.go — cross-runtime conformance, driven by the shared
// `test/spec/*.tsv` fixtures at the repo root (see ../test/AGENTS.md).
//
// The fixture loader, the escape codec, the ERROR: contract and the row
// loop all come from github.com/tabnas/support/go, whose TypeScript half
// ts/test/parity.test.js uses to run the SAME files — so the two renderers
// cannot drift without one going red, and neither can the two loaders.
//
// What is left here is only what is specific to railroad: which renderer a
// fixture is for.

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	support "github.com/tabnas/support/go"
)

// TestSpec runs every fixture in the spec directory. The second column's
// HEADER names the renderer, which is why there is a runner per file
// rather than one over the directory.
func TestSpec(t *testing.T) {
	dir, err := support.FindSpecDir("")
	if err != nil {
		t.Fatal(err)
	}

	specs, err := support.LoadSpecDir(dir, nil)
	if err != nil {
		t.Fatal(err)
	}

	for _, spec := range specs {
		if 2 > len(spec.Header) {
			t.Fatalf("%s: expected at least two columns", spec.Name)
		}
		kind := spec.Header[1]
		if "text" != kind && "ascii" != kind {
			t.Fatalf("%s: unknown second column %q", spec.Name, kind)
		}

		support.Runner{
			// The first column is the node's own JSON shape, which is what
			// both runtimes marshal to and unmarshal from. The third is
			// renderer options, when the renderer takes any.
			ParseRow: func(input string, row *support.Row) (any, error) {
				node := &RailroadNode{}
				if err := node.UnmarshalJSON([]byte(input)); err != nil {
					return nil, err
				}

				opts := map[string]any{}
				if raw := row.Named("opts"); "" != strings.TrimSpace(raw) {
					if err := json.Unmarshal([]byte(raw), &opts); err != nil {
						return nil, err
					}
				}

				return renderSpec(kind, node, opts)
			},

			// The rendered text is compared against the expected column,
			// which holds it as a JSON string — so the comparison is the
			// runner's ordinary one, over two strings.
			ExpectedName: kind,
		}.Spec(t, spec)
	}
}

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
