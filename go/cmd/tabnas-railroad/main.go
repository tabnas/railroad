// Copyright (c) 2026 Richard Rodger and other contributors, MIT License

// Command tabnas-railroad renders railroad (syntax) diagrams from a tabnas
// grammar. Two modes:
//
//	grammar mode:  --grammar <module>
//	  build a fresh Tabnas instance with the named grammar plugin,
//	  introspect it, and write grammar.railroad.json + grammar.svg +
//	  grammar.txt into the output dir (default ./out).
//
//	render mode:   -f <model.json> | -
//	  read a saved GrammarModel and render it to SVG / ASCII / text.
//
// It is the Go port of ts/src/bin/tabnas-railroad-cli.ts. Because Go has no
// dynamic module require(), grammar mode resolves a fixed set of built-in
// grammar names (currently "json" / "@tabnas/json"); render mode is fully
// general.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	jsonplugin "github.com/tabnas/json/go"
	tabnas "github.com/tabnas/parser/go"
	railroad "github.com/tabnas/railroad/go"
)

type cliArgs struct {
	help    bool
	stdin   bool
	grammar string
	file    string
	out     string
	svg     bool
	ascii   bool
	jsonOut bool
	text    bool
	plain   bool
}

func main() {
	os.Exit(run(os.Args, os.Stdin, os.Stdout, os.Stderr))
}

func run(argv []string, stdin io.Reader, stdout, stderr io.Writer) int {
	args := cliArgs{}
	for i := 1; i < len(argv); i++ {
		arg := argv[i]
		switch {
		case arg == "-":
			args.stdin = true
		case arg == "--help" || arg == "-h":
			args.help = true
		case arg == "--grammar" || arg == "-g":
			i++
			if i < len(argv) {
				args.grammar = argv[i]
			}
		case arg == "--file" || arg == "-f":
			i++
			if i < len(argv) {
				args.file = argv[i]
			}
		case arg == "--out" || arg == "-o":
			i++
			if i < len(argv) {
				args.out = argv[i]
			}
		case arg == "--svg":
			args.svg = true
		case arg == "--ascii":
			args.ascii = true
		case arg == "--json":
			args.jsonOut = true
		case arg == "--text":
			args.text = true
		case arg == "--ascii-plain":
			args.ascii = true
			args.plain = true
		}
	}

	if args.help {
		printHelp(stdout)
		return 0
	}

	// Obtain a GrammarModel.
	var model *railroad.GrammarModel
	switch {
	case args.grammar != "":
		m, err := grammarFromModule(args.grammar)
		if err != nil {
			fmt.Fprintf(stderr, "tabnas-railroad: %s\n", firstLine(err.Error()))
			return 1
		}
		model = m
	case args.file != "" || args.stdin:
		var src string
		if args.file != "" {
			b, err := os.ReadFile(args.file)
			if err != nil {
				fmt.Fprintf(stderr, "tabnas-railroad: %s\n", firstLine(err.Error()))
				return 1
			}
			src = string(b)
		}
		if strings.TrimSpace(src) == "" || args.stdin {
			b, _ := io.ReadAll(stdin)
			src += string(b)
		}
		var m railroad.GrammarModel
		if err := json.Unmarshal([]byte(src), &m); err != nil {
			fmt.Fprintf(stderr, "tabnas-railroad: invalid JSON grammar model: %s\n", err.Error())
			return 1
		}
		model = &m
	default:
		printHelp(stdout)
		return 0
	}

	// grammar mode (or any mode with -o) writes the three artifacts.
	if args.grammar != "" || args.out != "" {
		dir := args.out
		if dir == "" {
			dir = "out"
		}
		want := args
		if !anyFormat(args) {
			want.svg = true
			want.ascii = true
			want.jsonOut = true
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintf(stderr, "tabnas-railroad: %s\n", err.Error())
			return 1
		}
		var written []string
		if want.jsonOut {
			b, err := marshalIndent(model)
			if err != nil {
				fmt.Fprintf(stderr, "tabnas-railroad: %s\n", err.Error())
				return 1
			}
			if err := os.WriteFile(filepath.Join(dir, "grammar.railroad.json"), b, 0o644); err != nil {
				fmt.Fprintf(stderr, "tabnas-railroad: %s\n", err.Error())
				return 1
			}
			written = append(written, "grammar.railroad.json")
		}
		if want.svg {
			svg, err := railroad.ModelToSvg(model)
			if err != nil {
				fmt.Fprintf(stderr, "tabnas-railroad: %s\n", err.Error())
				return 1
			}
			if err := os.WriteFile(filepath.Join(dir, "grammar.svg"), []byte(svg), 0o644); err != nil {
				fmt.Fprintf(stderr, "tabnas-railroad: %s\n", err.Error())
				return 1
			}
			written = append(written, "grammar.svg")
		}
		if want.ascii || want.text {
			var out string
			var err error
			if want.text {
				out = textModel(model)
			} else {
				out, err = railroad.ModelToAscii(model, railroad.AsciiOptions{Plain: args.plain})
			}
			if err != nil {
				fmt.Fprintf(stderr, "tabnas-railroad: %s\n", err.Error())
				return 1
			}
			if err := os.WriteFile(filepath.Join(dir, "grammar.txt"), []byte(out), 0o644); err != nil {
				fmt.Fprintf(stderr, "tabnas-railroad: %s\n", err.Error())
				return 1
			}
			written = append(written, "grammar.txt")
		}
		fmt.Fprintf(stdout, "wrote %s to %s/\n", strings.Join(written, ", "), dir)
		return 0
	}

	// render mode -> stdout, single format (default SVG).
	switch {
	case args.jsonOut:
		b, err := marshalIndent(model)
		if err != nil {
			fmt.Fprintf(stderr, "tabnas-railroad: %s\n", err.Error())
			return 1
		}
		fmt.Fprintln(stdout, string(b))
	case args.text:
		fmt.Fprintln(stdout, textModel(model))
	case args.ascii:
		out, err := railroad.ModelToAscii(model, railroad.AsciiOptions{Plain: args.plain})
		if err != nil {
			fmt.Fprintf(stderr, "tabnas-railroad: %s\n", err.Error())
			return 1
		}
		fmt.Fprintln(stdout, out)
	default:
		svg, err := railroad.ModelToSvg(model)
		if err != nil {
			fmt.Fprintf(stderr, "tabnas-railroad: %s\n", err.Error())
			return 1
		}
		fmt.Fprintln(stdout, svg)
	}
	return 0
}

// grammarFromModule builds a fresh Tabnas instance with a built-in grammar
// plugin and introspects it. Go has no dynamic require(), so the supported
// grammar names are resolved statically.
func grammarFromModule(spec string) (*railroad.GrammarModel, error) {
	name := spec
	if i := strings.Index(name, "#"); i >= 0 {
		name = name[:i]
	}
	plugin, err := pickPlugin(name)
	if err != nil {
		return nil, err
	}
	tn := tabnas.Make()
	if err := plugin(tn, nil); err != nil {
		return nil, err
	}
	return railroad.ExtractGrammar(tn), nil
}

func pickPlugin(name string) (tabnas.Plugin, error) {
	switch name {
	case "json", "@tabnas/json", "tabnas/json", "github.com/tabnas/json/go":
		return jsonplugin.Json, nil
	default:
		return nil, fmt.Errorf("could not find a grammar plugin for %q (built-in grammars: json)", name)
	}
}

func anyFormat(a cliArgs) bool {
	return a.svg || a.ascii || a.jsonOut || a.text
}

// textModel renders compact per-rule EBNF-ish text: `name = <node text>`.
func textModel(model *railroad.GrammarModel) string {
	names := orderRules(model)
	lines := make([]string, 0, len(names))
	for _, n := range names {
		t, _ := railroad.ToText(model.Rules[n])
		lines = append(lines, n+" = "+t)
	}
	return strings.Join(lines, "\n")
}

func orderRules(model *railroad.GrammarModel) []string {
	names := model.RuleOrder
	if len(names) == 0 {
		for k := range model.Rules {
			names = append(names, k)
		}
	}
	if model.Start != "" {
		if _, ok := model.Rules[model.Start]; ok {
			out := []string{model.Start}
			for _, n := range names {
				if n != model.Start {
					out = append(out, n)
				}
			}
			return out
		}
	}
	return names
}

func marshalIndent(model *railroad.GrammarModel) ([]byte, error) {
	// The model's custom MarshalJSON emits compact, rule-ordered JSON; run it
	// through Indent so grammar.railroad.json is pretty-printed like the TS
	// JSON.stringify(model, null, 2).
	compact, err := json.Marshal(model)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, compact, "", "  "); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func printHelp(w io.Writer) {
	fmt.Fprint(w, `
tabnas-railroad: render railroad (syntax) diagrams from a tabnas grammar.

Usage:
  tabnas-railroad --grammar <module> [-o <dir>] [formats]
  tabnas-railroad -f <model.json> [format]
  echo '<model.json>' | tabnas-railroad - [format]

Modes:
  --grammar <m>            Build a fresh Tabnas instance with grammar plugin
  -g <m>                     <m>, introspect it, and write artifacts.
                           e.g. --grammar json (built-in grammars: json)
  --file <path>            Render a saved GrammarModel JSON file.
  -f <path>
  -                        Read a GrammarModel JSON from stdin.

Output:
  -o <dir>                 Write grammar.railroad.json + grammar.svg +
                            grammar.txt into <dir> (default ./out in
                            grammar mode). Without -o, render mode prints
                            one format to stdout.

Formats (default: all three when writing, SVG to stdout):
  --json                   Declarative JSON model.
  --svg                    Vertical-flow SVG.
  --ascii                  Vertical ASCII diagram.
  --ascii-plain            ASCII with plain | - + glyphs (implies --ascii).
  --text                   Compact per-rule EBNF text.

  --help, -h               Print this help.

Examples:
  > tabnas-railroad --grammar json -o diagrams
  > tabnas-railroad -f diagrams/grammar.railroad.json --ascii
  > tabnas-railroad --grammar json --text -o /tmp/rr
`)
}
