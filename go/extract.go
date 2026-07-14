// Copyright (c) 2026 Richard Rodger and other contributors, MIT License

// extract.go
// Build a railroad GrammarModel by introspecting a live tabnas instance.
// Reads the rule set (tn.RSM()) and resolved config (tn.Config()) and
// reverse-maps the alt-based rule machine into railroad constructs
// (sequence / choice / optional / repetition).
//
// Mapping (see doc + tests against @tabnas/json):
//   - open alt: the first sN-b token positions are consumed terminals; a
//     P push appends a nonterminal. b == sN (pure peek) consumes nothing —
//     render only the ref.
//   - several open alts -> choice.
//   - close alt R == <self> (+ guard token) -> repetition (OneOrMore with
//     the guard token on the return path). R == <other> -> continuation.
//     A close alt that consumes a token with no b/R is this rule's own
//     closing terminal (append it). A backup close (b>0) leaves the token
//     for the parent -> drop. End-of-source / pure-pop -> drop.
//   - synthetic helper rules (name has `$` or `_gen…`) are inlined.
//   - normalization factors common prefix/suffix across choice branches
//     and turns an empty branch into Optional.
//
// This file mirrors ts/src/extract.ts. The one structural difference from
// the TS port is the engine introspection shape: the Go engine resolves an
// alt's token spec into [][]Tin (the raw "#KEY"/"#VAL" set-name strings are
// not retained on the live RuleSpec), so the extractor recovers a readable
// set name by matching the resolved Tin set against the instance's named
// token sets, disambiguating identical sets (KEY vs VAL) by position role.
package tabnasrailroad

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	tabnas "github.com/tabnas/parser/go"
)

// ExtractOptions controls extraction.
type ExtractOptions struct {
	// NoFactor disables prefix/suffix factoring + empty-branch->optional
	// (the TS `factor` defaults to true; this is its inverse).
	NoFactor bool
	// Start overrides the entry rule (defaults to config RuleStart).
	Start string
	// TokenSetNames lists the named token-set candidates the extractor may
	// recover for a resolved Tin set, in preference order. When empty, the
	// standard set ["VAL", "KEY"] is used (the json grammar's sets). A
	// position immediately followed by a colon (CL) prefers "KEY".
	TokenSetNames []string
	// TokenDesc supplies human descriptions for named tokens/sets, keyed by
	// bare or "#"-prefixed name. It is the Go analog of the TS cfg.tokenDesc
	// hook a grammar attaches via config.modify.
	TokenDesc map[string]string
}

// ctx carries the resolved introspection state threaded through extraction.
type ctx struct {
	tn       *tabnas.Tabnas
	cfg      *tabnas.LexConfig
	rsm      map[string]*tabnas.RuleSpec
	zz, aa   tabnas.Tin
	factor   bool
	legend   map[string]string // named token label -> meaning
	setNames []string          // candidate token-set names, preference order
	tokenSet map[string][]tabnas.Tin
	tokenDsc map[string]string
}

// CANON gives canonical meanings for the standard tabnas/jsonic token names,
// used when a grammar references a token whose literal/regex isn't
// recoverable from its config. A grammar-supplied description takes
// precedence, then CANON, then engine-derived meanings.
var canon = map[string]string{
	"OB":  "open brace { (start of a map)",
	"CB":  "close brace } (end of a map)",
	"OS":  "open square bracket [ (start of a list)",
	"CS":  "close square bracket ] (end of a list)",
	"CL":  "colon : (separates a key from its value)",
	"CA":  "comma , (separates map pairs or list items)",
	"CO":  "comma , (separates map pairs or list items)",
	"SP":  "whitespace (spaces or tabs)",
	"LN":  "newline (line break)",
	"CM":  "comment",
	"NR":  "number literal (e.g. 42, -1.5, 1e3)",
	"ST":  "quoted string (e.g. \"text\")",
	"TX":  "bare unquoted text (an unquoted word)",
	"VL":  "value keyword (true, false, null, ...)",
	"ZZ":  "end of input (no more tokens)",
	"AA":  "any token (matches anything)",
	"BD":  "bad input (a character the lexer rejected)",
	"UK":  "unknown token (unrecognised input)",
	"KEY": "map key: bare text, number, string, or keyword",
	"VAL": "value: bare text, number, string, or keyword",
}

var hashPrefix = regexp.MustCompile(`^#`)

// descOf returns a grammar-supplied description for a token/set name, keyed
// by "#"-prefixed or bare name.
func descOf(name string, c *ctx) string {
	if c.tokenDsc == nil {
		return ""
	}
	if d, ok := c.tokenDsc["#"+name]; ok && d != "" {
		return d
	}
	if d, ok := c.tokenDsc[name]; ok && d != "" {
		return d
	}
	return ""
}

// canonShort returns the CANON meaning truncated at its parenthetical, for
// compact inline use in a token-set listing.
func canonShort(name string) string {
	c, ok := canon[name]
	if !ok {
		return name
	}
	if i := strings.Index(c, " ("); i > 0 {
		return c[:i]
	}
	return c
}

// ExtractGrammar builds a railroad GrammarModel from a live tabnas instance.
// It mirrors ts/extract.ts extractGrammar(tn).
func ExtractGrammar(tn *tabnas.Tabnas, opts ...*ExtractOptions) *GrammarModel {
	var o ExtractOptions
	if len(opts) > 0 && opts[0] != nil {
		o = *opts[0]
	}

	cfg := tn.Config()
	rsm := tn.RSM()

	setNames := o.TokenSetNames
	if len(setNames) == 0 {
		setNames = []string{"VAL", "KEY"}
	}

	c := &ctx{
		tn:       tn,
		cfg:      cfg,
		rsm:      rsm,
		zz:       tabnas.TinZZ,
		aa:       tabnas.TinAA,
		factor:   !o.NoFactor,
		legend:   map[string]string{},
		setNames: setNames,
		tokenSet: map[string][]tabnas.Tin{},
		tokenDsc: o.TokenDesc,
	}
	// Resolve the candidate token sets once.
	for _, name := range setNames {
		if tins := tn.TokenSet(name); tins != nil {
			c.tokenSet[name] = tins
		}
	}

	// Entry rule.
	start := o.Start
	if start == "" {
		if cfg != nil && cfg.RuleStart != "" {
			start = cfg.RuleStart
		} else {
			start = firstUserRule(rsm)
		}
	}
	if start == "__start__" {
		if spec, ok := rsm["__start__"]; ok {
			if u := unwrapStart(spec); u != "" {
				start = u
			}
		}
	}

	rules := map[string]*RailroadNode{}
	order := sortedUserRules(rsm)
	for _, name := range order {
		rules[name] = ruleNode(name, c, map[string]bool{})
	}

	// Legend: only the named tokens actually present in the final diagram.
	used := map[string]bool{}
	for _, r := range rules {
		collectTerminals(r, used)
	}
	legend := make([]LegendEntry, 0, len(c.legend))
	for label, meaning := range c.legend {
		if used[label] {
			legend = append(legend, LegendEntry{Token: label, Meaning: meaning})
		}
	}
	sort.Slice(legend, func(i, j int) bool { return legend[i].Token < legend[j].Token })

	ignored := buildIgnored(c)

	model := &GrammarModel{
		Start:     start,
		Rules:     rules,
		RuleOrder: order,
		Meta:      map[string]any{"engine": "tabnas"},
	}
	if len(legend) > 0 {
		model.Legend = legend
	}
	if len(ignored) > 0 {
		model.Ignored = ignored
	}
	return model
}

// buildIgnored reports the IGNORE token set (whitespace, newlines, comments)
// with the same descriptions, since those tokens never appear in a rule.
func buildIgnored(c *ctx) []LegendEntry {
	ig := c.tn.TokenSet("IGNORE")
	if ig == nil {
		return nil
	}
	out := []LegendEntry{}
	seen := map[string]bool{}
	for _, tin := range ig {
		if isControl(tin, c) {
			continue
		}
		name := stripHash(c.tn.TinName(tin))
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, LegendEntry{Token: name, Meaning: tokenMeaning(tin, c)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Token < out[j].Token })
	return out
}

// collectTerminals collects every distinct terminal label in a node tree.
func collectTerminals(node *RailroadNode, out map[string]bool) {
	if node == nil {
		return
	}
	switch node.Kind {
	case KindTerminal:
		out[node.Text] = true
	case KindSeq, KindChoice, KindDiagram:
		for _, n := range node.Items {
			collectTerminals(n, out)
		}
	case KindOptional:
		collectTerminals(node.Item, out)
	case KindOneOrMore, KindZeroOrMore:
		collectTerminals(node.Item, out)
		if node.Rep != nil {
			collectTerminals(node.Rep, out)
		}
	}
}

// ---- rule -> node --------------------------------------------------

func ruleNode(name string, c *ctx, visited map[string]bool) *RailroadNode {
	spec, ok := c.rsm[name]
	if !ok || spec == nil {
		return NonTerminal(name)
	}
	if visited[name] || len(visited) > 32 {
		return NonTerminal(name)
	}
	v2 := cloneVisited(visited)
	v2[name] = true

	open := spec.OpenAlts()
	closeAlts := spec.CloseAlts()

	branches := []*RailroadNode{}
	for _, a := range open {
		if n := openAltNode(a, c, v2); n != nil {
			branches = append(branches, n)
		}
	}

	var body *RailroadNode
	switch len(branches) {
	case 0:
		body = SkipNode()
	case 1:
		body = branches[0]
	default:
		body = MustChoice(branches...)
	}

	body = applyCloseAlts(body, closeAlts, name, c, v2)
	if c.factor {
		body = normalizeNode(body)
	}
	return body
}

func openAltNode(alt *tabnas.AltSpec, c *ctx, visited map[string]bool) *RailroadNode {
	positions := altPositions(alt, c)
	sN := len(positions)
	consumed := sN - numericBack(alt)
	if consumed < 0 {
		consumed = 0
	}

	parts := []*RailroadNode{}
	for i := 0; i < consumed && i < len(positions); i++ {
		if pn := positionNode(positions, i, c); pn != nil {
			parts = append(parts, pn)
		}
	}
	if ref := refNode(alt.P, c, visited); ref != nil {
		parts = append(parts, ref)
	}

	switch len(parts) {
	case 0:
		return SkipNode()
	case 1:
		return parts[0]
	default:
		return Sequence(parts...)
	}
}

func applyCloseAlts(
	body *RailroadNode, closeAlts []*tabnas.AltSpec, ruleName string, c *ctx, visited map[string]bool,
) *RailroadNode {
	repeat := false
	var repSep *RailroadNode
	var closingTerm *RailroadNode
	var continuation *RailroadNode

	for _, alt := range closeAlts {
		positions := altPositions(alt, c)
		back := numericBack(alt)
		sN := len(positions)
		consumed := sN - back
		if consumed < 0 {
			consumed = 0
		}

		if alt.R != "" {
			if alt.R == ruleName {
				repeat = true
				if consumed > 0 {
					if pn := positionNode(positions, 0, c); pn != nil {
						repSep = pn
					}
				}
			} else {
				if cn := refNode(alt.R, c, visited); cn != nil {
					continuation = cn
				}
			}
			continue
		}

		// No R: a backup close leaves the token for the parent -> drop.
		if back > 0 {
			continue
		}

		// Consumes token(s) with no backup -> this rule's own closing term.
		terms := []*RailroadNode{}
		for i := 0; i < consumed && i < len(positions); i++ {
			if pn := positionNode(positions, i, c); pn != nil {
				terms = append(terms, pn)
			}
		}
		if len(terms) == 1 {
			closingTerm = terms[0]
		} else if len(terms) > 1 {
			closingTerm = Sequence(terms...)
		}
		// else: pure pop / end-of-source -> drop
	}

	result := body
	if repeat {
		result = OneOrMore(body, repSep)
	}
	if continuation != nil {
		result = Sequence(append(flattenSeq(result), continuation)...)
	}
	if closingTerm != nil {
		result = Sequence(append(flattenSeq(result), closingTerm)...)
	}
	return result
}

// refNode resolves a push/replace target -> inline if synthetic, else a
// nonterminal link. (The Go engine has no dynamic-ref placeholder in the
// resolved alt; PF/RF dynamic refs are surfaced as a "dynamic" comment by
// the caller only when the static name is empty but a func ref exists; we
// keep parity with the static path here.)
func refNode(target string, c *ctx, visited map[string]bool) *RailroadNode {
	if target == "" {
		return nil
	}
	if isSynthetic(target) {
		if _, ok := c.rsm[target]; ok {
			return ruleNode(target, c, visited)
		}
	}
	return NonTerminal(target)
}

// ---- token / position resolution -----------------------------------

// position holds the resolved Tin set for one lookahead slot.
type position struct {
	tins []tabnas.Tin
}

func altPositions(alt *tabnas.AltSpec, c *ctx) []position {
	out := make([]position, 0, len(alt.S))
	for _, slot := range alt.S {
		tins := make([]tabnas.Tin, 0, len(slot))
		tins = append(tins, slot...)
		out = append(out, position{tins: tins})
	}
	return out
}

// positionNode maps one token position -> a node (or nil if it's pure
// control tokens). It uses the surrounding positions to disambiguate a
// key position (followed by a colon) as KEY.
//
// The TS extractor reads the raw "#KEY"/"#VAL" set name straight off the
// alt spec to render a readable set label; the Go engine resolves that to a
// Tin set, so this recovers the set name by matching the resolved tins to a
// named token set (preferring it over a bare per-tin label, regardless of
// the set's size — which is how the json grammar's single-member KEY set
// becomes the "KEY" terminal). Punctuation literals (e.g. "{", which is no
// token set's member) fall through to the per-tin fixed label.
func positionNode(positions []position, i int, c *ctx) *RailroadNode {
	pos := positions[i]
	useful := []tabnas.Tin{}
	for _, t := range pos.tins {
		if !isControl(t, c) {
			useful = append(useful, t)
		}
	}
	if len(useful) == 0 {
		return nil
	}
	if setName := matchTokenSet(useful, positions, i, c); setName != "" {
		c.legend[setName] = setMeaning(setName, c)
		return Terminal(setName)
	}
	if len(useful) == 1 {
		return Terminal(namedLabel(useful[0], c))
	}
	branches := make([]*RailroadNode, len(useful))
	for k, t := range useful {
		branches[k] = Terminal(namedLabel(t, c))
	}
	return MustChoice(branches...)
}

// namedLabel returns the token label, recording a legend entry when the
// label is a named token (not a self-explanatory punctuation literal).
func namedLabel(tin tabnas.Tin, c *ctx) string {
	label := tokenLabel(tin, c)
	fixed := c.tn.FixedTin(tin)
	if label != fixed {
		c.legend[label] = tokenMeaning(tin, c)
	}
	return label
}

// tokenMeaning returns the human meaning for a named token.
func tokenMeaning(tin tabnas.Tin, c *ctx) string {
	name := stripHash(c.tn.TinName(tin))
	if d := descOf(name, c); d != "" {
		return d
	}
	if m, ok := canon[name]; ok {
		return m
	}
	if c.cfg != nil && c.cfg.MatchTokens != nil {
		if re, ok := c.cfg.MatchTokens[tin]; ok && re != nil {
			return "text matching /" + prettySource(re) + "/"
		}
	}
	if c.cfg != nil && c.cfg.FixedTokens != nil {
		for src, t := range c.cfg.FixedTokens {
			if t == tin {
				return "literal " + jsonString(src)
			}
		}
	}
	if owner := soleSetOf(tin, c); owner != "" {
		return "part of " + owner
	}
	if name != "" {
		return name + " token"
	}
	return "token"
}

// setMeaning returns the human meaning for a named token set.
func setMeaning(setName string, c *ctx) string {
	if d := descOf(setName, c); d != "" {
		return d
	}
	if m, ok := canon[setName]; ok {
		return m
	}
	members := []string{}
	seen := map[string]bool{}
	for _, t := range c.tokenSet[setName] {
		n := stripHash(c.tn.TinName(t))
		var label string
		if d := descOf(n, c); d != "" {
			label = d
		} else if cs := canonShort(n); cs != "" {
			label = cs
		} else {
			label = n
		}
		if label != "" && !seen[label] {
			seen[label] = true
			members = append(members, label)
		}
	}
	if len(members) > 0 {
		return "one of: " + strings.Join(members, ", ")
	}
	return "token set"
}

// soleSetOf returns the single named token set a tin belongs to (ignoring
// IGNORE), or "" if it is in none or several.
func soleSetOf(tin tabnas.Tin, c *ctx) string {
	found := ""
	for _, name := range c.setNames {
		if name == "IGNORE" {
			continue
		}
		for _, m := range c.tokenSet[name] {
			if m == tin {
				if found != "" {
					return ""
				}
				found = name
				break
			}
		}
	}
	return found
}

func tokenLabel(tin tabnas.Tin, c *ctx) string {
	if fixed := c.tn.FixedTin(tin); fixed != "" {
		return fixed
	}
	if nm := c.tn.TinName(tin); nm != "" {
		return stripHash(nm)
	}
	if c.cfg != nil && c.cfg.MatchTokens != nil {
		if re, ok := c.cfg.MatchTokens[tin]; ok && re != nil {
			return prettySource(re)
		}
	}
	return "#" + itoa(tin)
}

// matchTokenSet finds a named token set whose members equal the given tins.
// When several named sets match (e.g. KEY and VAL share the same members),
// it prefers "KEY" if the position is immediately followed by a colon (CL)
// — a map-key position — else the first candidate in preference order.
func matchTokenSet(tins []tabnas.Tin, positions []position, i int, c *ctx) string {
	want := map[tabnas.Tin]bool{}
	for _, t := range tins {
		want[t] = true
	}
	matched := []string{}
	for _, name := range c.setNames {
		members := c.tokenSet[name]
		if len(members) != len(want) {
			continue
		}
		all := true
		for _, m := range members {
			if !want[m] {
				all = false
				break
			}
		}
		if all {
			matched = append(matched, name)
		}
	}
	if len(matched) == 0 {
		return ""
	}
	if len(matched) == 1 {
		return matched[0]
	}
	// Ambiguous: prefer KEY when this is a key position (next slot is CL).
	if isKeyPosition(positions, i, c) {
		for _, name := range matched {
			if name == "KEY" {
				return name
			}
		}
	}
	return matched[0]
}

// isKeyPosition reports whether the slot at i is immediately followed by a
// colon (CL) slot — the shape of a map key (KEY ":" ...).
func isKeyPosition(positions []position, i int, c *ctx) bool {
	if i+1 >= len(positions) {
		return false
	}
	next := positions[i+1].tins
	for _, t := range next {
		if t == tabnas.TinCL {
			return true
		}
	}
	return false
}

// ---- normalization passes ------------------------------------------

func normalizeNode(node *RailroadNode) *RailroadNode {
	if node == nil {
		return node
	}
	switch node.Kind {
	case KindSeq:
		return seqOf(mapNodes(node.Items, normalizeNode))
	case KindChoice:
		return factorChoice(mapNodes(node.Items, normalizeNode))
	case KindOptional:
		return Optional(normalizeNode(node.Item))
	case KindOneOrMore:
		return OneOrMore(normalizeNode(node.Item), normalizeRep(node.Rep))
	case KindZeroOrMore:
		return ZeroOrMore(normalizeNode(node.Item), normalizeRep(node.Rep))
	case KindDiagram:
		return &RailroadNode{Kind: KindDiagram, Items: mapNodes(node.Items, normalizeNode)}
	default:
		return node
	}
}

func normalizeRep(rep *RailroadNode) *RailroadNode {
	if rep == nil {
		return nil
	}
	return normalizeNode(rep)
}

func factorChoice(rawBranches []*RailroadNode) *RailroadNode {
	// Dedup identical branches.
	branches := []*RailroadNode{}
	for _, b := range rawBranches {
		dup := false
		for _, u := range branches {
			if NodeEqual(u, b) {
				dup = true
				break
			}
		}
		if !dup {
			branches = append(branches, b)
		}
	}
	if len(branches) == 1 {
		return branches[0]
	}

	// Separate empty (skip) branches.
	hasEmpty := false
	nonEmpty := []*RailroadNode{}
	for _, b := range branches {
		if b.Kind == KindSkip {
			hasEmpty = true
			continue
		}
		nonEmpty = append(nonEmpty, b)
	}
	if len(nonEmpty) == 0 {
		return SkipNode()
	}

	seqs := make([][]*RailroadNode, len(nonEmpty))
	for i, n := range nonEmpty {
		seqs[i] = asSeqItems(n)
	}

	// Common prefix.
	prefix := []*RailroadNode{}
prefixLoop:
	for i := 0; ; i++ {
		if i >= len(seqs[0]) {
			break
		}
		first := seqs[0][i]
		for _, s := range seqs {
			if i >= len(s) || !NodeEqual(s[i], first) {
				break prefixLoop
			}
		}
		prefix = append(prefix, first)
	}

	// Common suffix (not overlapping the prefix).
	suffix := []*RailroadNode{}
	minTail := 1 << 30
	for _, s := range seqs {
		if tail := len(s) - len(prefix); tail < minTail {
			minTail = tail
		}
	}
suffixLoop:
	for k := 1; k <= minTail; k++ {
		first := seqs[0][len(seqs[0])-k]
		for _, s := range seqs {
			el := s[len(s)-k]
			if !NodeEqual(el, first) {
				break suffixLoop
			}
		}
		suffix = append([]*RailroadNode{first}, suffix...)
	}

	// Remainders between prefix and suffix.
	remEmpty := false
	remNodes := []*RailroadNode{}
	for _, s := range seqs {
		rem := seqOf(s[len(prefix) : len(s)-len(suffix)])
		if rem.Kind == KindSkip {
			remEmpty = true
			continue
		}
		dup := false
		for _, u := range remNodes {
			if NodeEqual(u, rem) {
				dup = true
				break
			}
		}
		if !dup {
			remNodes = append(remNodes, rem)
		}
	}

	var core *RailroadNode
	switch len(remNodes) {
	case 0:
		core = SkipNode()
	case 1:
		core = remNodes[0]
	default:
		core = &RailroadNode{Kind: KindChoice, Items: remNodes}
	}

	if (hasEmpty || remEmpty) && core.Kind != KindSkip {
		core = Optional(core)
	}

	out := append([]*RailroadNode{}, prefix...)
	if core.Kind != KindSkip {
		out = append(out, core)
	}
	out = append(out, suffix...)
	return seqOf(out)
}

func asSeqItems(node *RailroadNode) []*RailroadNode {
	if node.Kind == KindSeq {
		return append([]*RailroadNode{}, node.Items...)
	}
	if node.Kind == KindSkip {
		return []*RailroadNode{}
	}
	return []*RailroadNode{node}
}

func flattenSeq(node *RailroadNode) []*RailroadNode {
	if node.Kind == KindSeq {
		return append([]*RailroadNode{}, node.Items...)
	}
	if node.Kind == KindSkip {
		return []*RailroadNode{}
	}
	return []*RailroadNode{node}
}

func seqOf(items []*RailroadNode) *RailroadNode {
	flat := []*RailroadNode{}
	for _, it := range items {
		if it.Kind == KindSeq {
			flat = append(flat, it.Items...)
		} else if it.Kind == KindSkip {
			continue
		} else {
			flat = append(flat, it)
		}
	}
	switch len(flat) {
	case 0:
		return SkipNode()
	case 1:
		return flat[0]
	default:
		return &RailroadNode{Kind: KindSeq, Items: flat}
	}
}

// ---- small helpers -------------------------------------------------

func isControl(tin tabnas.Tin, c *ctx) bool {
	return tin == c.zz || tin == c.aa
}

// numericBack returns the alt's backtrack count (B). The Go engine resolves
// a function-form backtrack into BF; a present BF is treated as a single
// peek (1), mirroring the TS numericBack(function -> 1).
func numericBack(alt *tabnas.AltSpec) int {
	if alt.BF != nil {
		return 1
	}
	return alt.B
}

func isSynthetic(name string) bool {
	if strings.Contains(name, "$") {
		return true
	}
	return genRe.MatchString(name)
}

var genRe = regexp.MustCompile(`^_gen\d`)

func isUserRule(name string) bool {
	return name != "__start__" && !isSynthetic(name)
}

func firstUserRule(rsm map[string]*tabnas.RuleSpec) string {
	for _, name := range sortedKeys(rsm) {
		if isUserRule(name) {
			return name
		}
	}
	return ""
}

// sortedUserRules returns the user rules in a deterministic order: the
// engine's RSM is a Go map (unordered), so we sort the keys to make the
// model's rule order stable across runs.
func sortedUserRules(rsm map[string]*tabnas.RuleSpec) []string {
	out := []string{}
	for _, name := range sortedKeys(rsm) {
		if isUserRule(name) {
			out = append(out, name)
		}
	}
	return out
}

func sortedKeys(rsm map[string]*tabnas.RuleSpec) []string {
	keys := make([]string, 0, len(rsm))
	for k := range rsm {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func unwrapStart(spec *tabnas.RuleSpec) string {
	if spec == nil {
		return ""
	}
	open := spec.OpenAlts()
	if len(open) > 0 && open[0] != nil && open[0].P != "" {
		return open[0].P
	}
	return ""
}

func stripHash(s string) string {
	return hashPrefix.ReplaceAllString(s, "")
}

func prettySource(re *regexp.Regexp) string {
	s := re.String()
	s = strings.TrimPrefix(s, "^")
	s = strings.TrimSuffix(s, "$")
	return s
}

func cloneVisited(v map[string]bool) map[string]bool {
	out := make(map[string]bool, len(v)+1)
	for k := range v {
		out[k] = true
	}
	return out
}

func mapNodes(items []*RailroadNode, f func(*RailroadNode) *RailroadNode) []*RailroadNode {
	out := make([]*RailroadNode, len(items))
	for i, n := range items {
		out[i] = f(n)
	}
	return out
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
