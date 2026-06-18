// Copyright (c) 2026 Richard Rodger and other contributors, MIT License

// main_test.go is the Go port of the `cli` describe block in
// ts/test/grammar.test.js: grammar mode writes the three artifacts, render
// mode renders a saved model from -f, and -h prints help.
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGrammarModeWritesThreeArtifacts(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := run([]string{"tabnas-railroad", "--grammar", "json", "-o", dir},
		strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("unexpected stderr: %s", stderr.String())
	}

	jsonBytes, err := os.ReadFile(filepath.Join(dir, "grammar.railroad.json"))
	if err != nil {
		t.Fatal(err)
	}
	var model struct {
		Start string `json:"start"`
	}
	if err := json.Unmarshal(jsonBytes, &model); err != nil {
		t.Fatal(err)
	}
	if model.Start != "val" {
		t.Errorf("start = %q, want val", model.Start)
	}

	svg, err := os.ReadFile(filepath.Join(dir, "grammar.svg"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(svg)
	if !strings.HasPrefix(s, "<svg ") || !strings.HasSuffix(s, "</svg>") {
		t.Errorf("svg not well-formed")
	}

	txt, err := os.ReadFile(filepath.Join(dir, "grammar.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(txt), "val:") {
		t.Errorf("grammar.txt should contain 'val:'")
	}
}

func TestRenderModeFromFile(t *testing.T) {
	dir := t.TempDir()
	// Produce a saved model first (grammar mode).
	var dummy bytes.Buffer
	if code := run([]string{"tabnas-railroad", "--grammar", "json", "-o", dir},
		strings.NewReader(""), &dummy, &dummy); code != 0 {
		t.Fatalf("setup failed: %s", dummy.String())
	}
	file := filepath.Join(dir, "grammar.railroad.json")

	var stdout, stderr bytes.Buffer
	code := run([]string{"tabnas-railroad", "-f", file, "--text"},
		strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "val = ") {
		t.Errorf("text output should start a rule with 'val = ', got:\n%s", out)
	}
}

func TestHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"tabnas-railroad", "-h"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Errorf("help should contain 'Usage:'")
	}
}
