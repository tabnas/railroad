// Copyright (c) 2026 Richard Rodger and other contributors, MIT License

// railroad.go
// Railroad-diagram plugin for the tabnas parser, the Go port of
// ts/src/railroad.ts. Loading the plugin decorates the instance with a
// "railroad" decoration: a *RailroadApi that introspects THIS instance's
// installed grammar and returns a declarative GrammarModel, plus helpers to
// render that grammar to a vertical-flow SVG or ASCII diagram.
//
//	tn := tabnas.Make()
//	json.Json(tn, nil)
//	railroad.Plugin(tn, nil)
//	api := railroad.Of(tn)
//	model, _ := api.ToJson()   // GrammarModel
//	svg, _ := api.ToSvg()      // whole-grammar SVG
//	ascii, _ := api.ToAscii()  // whole-grammar ASCII
//
// The extraction + rendering logic lives in extract.go / svg.go / ascii.go;
// this file is the plugin wiring plus the package-level entry points for
// instance-free use.
package railroad

import (
	tabnas "github.com/tabnas/parser/go"
)

// DecorationName is the key under which the plugin stores its API on the
// instance (retrieve via tn.Decoration(DecorationName) or railroad.Of(tn)).
const DecorationName = "railroad"

// RailroadApi is the shape the "railroad" decoration takes: a grammar
// extractor plus render helpers and the diagram-model constructors. It is
// the Go analog of the TS callable tn.railroad.
type RailroadApi struct {
	tn *tabnas.Tabnas
}

// Extract introspects this instance's grammar and returns its GrammarModel.
// (Go analog of calling tn.railroad() / tn.railroad.toJson().)
func (a *RailroadApi) Extract(opts ...*ExtractOptions) *GrammarModel {
	return ExtractGrammar(a.tn, opts...)
}

// ToJson is an alias of Extract.
func (a *RailroadApi) ToJson(opts ...*ExtractOptions) *GrammarModel {
	return ExtractGrammar(a.tn, opts...)
}

// ToSvg extracts this instance's grammar and renders it to SVG.
func (a *RailroadApi) ToSvg(opts ...*ExtractOptions) (string, error) {
	return ModelToSvg(a.Extract(opts...))
}

// ToAscii extracts this instance's grammar and renders it to ASCII. A
// trailing AsciiOptions controls plain mode.
func (a *RailroadApi) ToAscii(asciiOpts AsciiOptions, opts ...*ExtractOptions) (string, error) {
	return ModelToAscii(a.Extract(opts...), asciiOpts)
}

// RenderNode renders a single node to a standalone SVG.
func (a *RailroadApi) RenderNode(node *RailroadNode, opts ...SvgOptions) (string, error) {
	return RenderNodeSvg(node, opts...)
}

// RenderNodeAscii renders a single node to an ASCII block.
func (a *RailroadApi) RenderNodeAscii(node *RailroadNode, opts ...AsciiOptions) (string, error) {
	return RenderNodeAscii(node, opts...)
}

// RenderNodeText renders a single node to compact EBNF text.
func (a *RailroadApi) RenderNodeText(node *RailroadNode) (string, error) {
	return ToText(node)
}

// Plugin is the tabnas plugin entry point. Decoration is lazy: every helper
// re-reads the instance's current grammar when called, so install order
// does not matter. Mirrors the TS railroad plugin.
func Plugin(tn *tabnas.Tabnas, _ map[string]any) error {
	tn.Decorate(DecorationName, &RailroadApi{tn: tn})
	return nil
}

// Of returns the RailroadApi previously installed on tn by Plugin, or a
// freshly-bound API when the plugin has not been loaded (so callers can use
// the helpers without explicitly installing the plugin first).
func Of(tn *tabnas.Tabnas) *RailroadApi {
	if v := tn.Decoration(DecorationName); v != nil {
		if api, ok := v.(*RailroadApi); ok {
			return api
		}
	}
	return &RailroadApi{tn: tn}
}
