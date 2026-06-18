// Copyright (c) 2026 Richard Rodger and other contributors, MIT License

// Package railroad is the Go port of @tabnas/railroad: it introspects a
// live tabnas grammar instance and renders railroad / syntax diagrams as
// a declarative JSON GrammarModel, vertical-flow SVG, and vertical ASCII.
//
// The model (this file) is a small, engine-agnostic tree of nodes
// (terminal, nonterminal, sequence, choice, optional, repetition) plus
// the GrammarModel envelope (one node per grammar rule). It is pure,
// JSON-serializable data: the interchange format that the SVG and ASCII
// renderers consume, and that the extractor produces from a live tabnas
// instance.
//
// This file mirrors ts/src/model.ts.
package tabnasrailroad

import (
	"encoding/json"
	"strings"
)

// Version is the current version of the module.
const Version = "0.2.0"

// NodeKind enumerates the railroad node variants (the tagged-union tag).
type NodeKind = string

const (
	KindTerminal    NodeKind = "terminal"
	KindNonTerminal NodeKind = "nonterminal"
	KindComment     NodeKind = "comment"
	KindSkip        NodeKind = "skip"
	KindSeq         NodeKind = "seq"
	KindChoice      NodeKind = "choice"
	KindOptional    NodeKind = "optional"
	KindOneOrMore   NodeKind = "oneOrMore"
	KindZeroOrMore  NodeKind = "zeroOrMore"
	KindDiagram     NodeKind = "diagram"
)

// RailroadNode is one node of a railroad diagram tree. It is a faithful
// port of the TypeScript discriminated union: which fields are populated
// depends on Kind. Terminal/NonTerminal/Comment use Text; Seq/Choice/
// Diagram use Items; Optional uses Item; OneOrMore/ZeroOrMore use Item and
// the optional Rep; Skip uses nothing.
//
// Custom JSON marshalling reproduces the TS object shapes exactly: each
// kind serializes only its relevant fields (so a Terminal is
// {"kind":"terminal","text":"x"} and a Skip is {"kind":"skip"}), and Rep
// is omitted when absent.
type RailroadNode struct {
	Kind  NodeKind
	Text  string
	Items []*RailroadNode
	Item  *RailroadNode
	Rep   *RailroadNode
}

// MarshalJSON emits the kind-specific object shape, matching ts/model.ts.
func (n *RailroadNode) MarshalJSON() ([]byte, error) {
	if n == nil {
		return []byte("null"), nil
	}
	switch n.Kind {
	case KindTerminal, KindNonTerminal, KindComment:
		return json.Marshal(struct {
			Kind string `json:"kind"`
			Text string `json:"text"`
		}{n.Kind, n.Text})
	case KindSkip:
		return json.Marshal(struct {
			Kind string `json:"kind"`
		}{n.Kind})
	case KindSeq, KindChoice, KindDiagram:
		items := n.Items
		if items == nil {
			items = []*RailroadNode{}
		}
		return json.Marshal(struct {
			Kind  string          `json:"kind"`
			Items []*RailroadNode `json:"items"`
		}{n.Kind, items})
	case KindOptional:
		return json.Marshal(struct {
			Kind string        `json:"kind"`
			Item *RailroadNode `json:"item"`
		}{n.Kind, n.Item})
	case KindOneOrMore, KindZeroOrMore:
		if n.Rep == nil {
			return json.Marshal(struct {
				Kind string        `json:"kind"`
				Item *RailroadNode `json:"item"`
			}{n.Kind, n.Item})
		}
		return json.Marshal(struct {
			Kind string        `json:"kind"`
			Item *RailroadNode `json:"item"`
			Rep  *RailroadNode `json:"rep"`
		}{n.Kind, n.Item, n.Rep})
	default:
		return nil, &RailroadError{Message: "railroad: unknown node kind " + jsonString(n.Kind), Node: n}
	}
}

// UnmarshalJSON parses a kind-specific object shape into a RailroadNode,
// so a model round-trips through JSON (used by the render-mode CLI and the
// round-trip tests).
func (n *RailroadNode) UnmarshalJSON(data []byte) error {
	var raw struct {
		Kind  NodeKind          `json:"kind"`
		Text  string            `json:"text"`
		Items []json.RawMessage `json:"items"`
		Item  json.RawMessage   `json:"item"`
		Rep   json.RawMessage   `json:"rep"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	n.Kind = raw.Kind
	n.Text = raw.Text
	if raw.Items != nil {
		n.Items = make([]*RailroadNode, len(raw.Items))
		for i, im := range raw.Items {
			child := &RailroadNode{}
			if err := child.UnmarshalJSON(im); err != nil {
				return err
			}
			n.Items[i] = child
		}
	}
	if len(raw.Item) > 0 && string(raw.Item) != "null" {
		child := &RailroadNode{}
		if err := child.UnmarshalJSON(raw.Item); err != nil {
			return err
		}
		n.Item = child
	}
	if len(raw.Rep) > 0 && string(raw.Rep) != "null" {
		child := &RailroadNode{}
		if err := child.UnmarshalJSON(raw.Rep); err != nil {
			return err
		}
		n.Rep = child
	}
	return nil
}

// LegendEntry names a token (or token set) that appears in the diagram and
// gives its human meaning. Used for the diagram legend and the ignored set.
type LegendEntry struct {
	Token   string `json:"token"`
	Meaning string `json:"meaning"`
}

// GrammarModel is one whole grammar: an ordered rule map plus the entry
// rule. This is the declarative artifact emitted as grammar.railroad.json.
//
// RuleOrder preserves the insertion order of rule names (Go maps have no
// stable order); the renderers and JSON emitter use it so output is
// deterministic and matches the TS object-key order.
type GrammarModel struct {
	Start     string                   `json:"start"`
	Rules     map[string]*RailroadNode `json:"rules"`
	RuleOrder []string                 `json:"-"`
	Legend    []LegendEntry            `json:"legend,omitempty"`
	Ignored   []LegendEntry            `json:"ignored,omitempty"`
	Meta      map[string]any           `json:"meta,omitempty"`
}

// MarshalJSON emits the model with rules in RuleOrder (falling back to map
// order when RuleOrder is empty) so the serialized form is deterministic.
func (m *GrammarModel) MarshalJSON() ([]byte, error) {
	var b strings.Builder
	b.WriteString(`{"start":`)
	writeJSON(&b, m.Start)

	b.WriteString(`,"rules":{`)
	order := m.RuleOrder
	if len(order) == 0 {
		order = make([]string, 0, len(m.Rules))
		for k := range m.Rules {
			order = append(order, k)
		}
	}
	first := true
	for _, name := range order {
		node, ok := m.Rules[name]
		if !ok {
			continue
		}
		if !first {
			b.WriteByte(',')
		}
		first = false
		writeJSON(&b, name)
		b.WriteByte(':')
		nb, err := json.Marshal(node)
		if err != nil {
			return nil, err
		}
		b.Write(nb)
	}
	b.WriteByte('}')

	if len(m.Legend) > 0 {
		b.WriteString(`,"legend":`)
		lb, err := json.Marshal(m.Legend)
		if err != nil {
			return nil, err
		}
		b.Write(lb)
	}
	if len(m.Ignored) > 0 {
		b.WriteString(`,"ignored":`)
		ib, err := json.Marshal(m.Ignored)
		if err != nil {
			return nil, err
		}
		b.Write(ib)
	}
	if m.Meta != nil {
		b.WriteString(`,"meta":`)
		mb, err := json.Marshal(m.Meta)
		if err != nil {
			return nil, err
		}
		b.Write(mb)
	}
	b.WriteByte('}')
	return []byte(b.String()), nil
}

// UnmarshalJSON parses a model, recovering RuleOrder from the raw JSON key
// order so a round-tripped model renders identically.
func (m *GrammarModel) UnmarshalJSON(data []byte) error {
	var raw struct {
		Start   string                     `json:"start"`
		Rules   map[string]json.RawMessage `json:"rules"`
		Legend  []LegendEntry              `json:"legend"`
		Ignored []LegendEntry              `json:"ignored"`
		Meta    map[string]any             `json:"meta"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	m.Start = raw.Start
	m.Legend = raw.Legend
	m.Ignored = raw.Ignored
	m.Meta = raw.Meta
	m.Rules = make(map[string]*RailroadNode, len(raw.Rules))
	for name, rm := range raw.Rules {
		node := &RailroadNode{}
		if err := node.UnmarshalJSON(rm); err != nil {
			return err
		}
		m.Rules[name] = node
	}
	m.RuleOrder = ruleKeyOrder(data)
	return nil
}

// ruleKeyOrder extracts the rule-name keys in the order they appear in the
// raw JSON, so an unmarshalled model preserves the author's rule ordering.
func ruleKeyOrder(data []byte) []string {
	dec := json.NewDecoder(strings.NewReader(string(data)))
	// Walk to the "rules" object value and read its keys in order.
	depth := 0
	inRules := false
	var order []string
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case json.Delim:
			switch t {
			case '{':
				depth++
			case '}':
				depth--
				if inRules && depth == 1 {
					inRules = false
				}
			}
		case string:
			if depth == 1 && t == "rules" {
				// Next token is the rules object's opening delim.
				if d, err := dec.Token(); err == nil {
					if dd, ok := d.(json.Delim); ok && dd == '{' {
						depth++
						inRules = true
						readObjectKeys(dec, &order)
						depth--
						inRules = false
					}
				}
			}
		}
	}
	return order
}

// readObjectKeys reads the keys of the current JSON object (already past
// its opening brace), appending each top-level key to order and skipping
// its value.
func readObjectKeys(dec *json.Decoder, order *[]string) {
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return
		}
		key, ok := keyTok.(string)
		if !ok {
			return
		}
		*order = append(*order, key)
		skipValue(dec)
	}
	// Consume the closing brace.
	dec.Token()
}

// skipValue consumes one complete JSON value from the decoder.
func skipValue(dec *json.Decoder) {
	tok, err := dec.Token()
	if err != nil {
		return
	}
	if d, ok := tok.(json.Delim); ok && (d == '{' || d == '[') {
		depth := 1
		for depth > 0 {
			t, err := dec.Token()
			if err != nil {
				return
			}
			if dd, ok := t.(json.Delim); ok {
				if dd == '{' || dd == '[' {
					depth++
				} else {
					depth--
				}
			}
		}
	}
}

// RailroadError is raised on a malformed diagram model (empty choice,
// unknown kind, invalid node). It mirrors the TS RailroadError.
type RailroadError struct {
	Message string
	Node    any
}

func (e *RailroadError) Error() string { return e.Message }

// ---- node constructors ---------------------------------------------

// Terminal builds a terminal (literal / named token) node.
func Terminal(text string) *RailroadNode { return &RailroadNode{Kind: KindTerminal, Text: text} }

// NonTerminal builds a nonterminal (rule reference) node.
func NonTerminal(text string) *RailroadNode {
	return &RailroadNode{Kind: KindNonTerminal, Text: text}
}

// Comment builds an inline comment node.
func Comment(text string) *RailroadNode { return &RailroadNode{Kind: KindComment, Text: text} }

// SkipNode builds a bypass (empty) node.
func SkipNode() *RailroadNode { return &RailroadNode{Kind: KindSkip} }

// Sequence builds a sequence of nodes.
func Sequence(items ...*RailroadNode) *RailroadNode {
	return &RailroadNode{Kind: KindSeq, Items: items}
}

// Choice builds a choice of branches. It returns a RailroadError (via
// panic-free path: callers that may pass zero branches must check) when
// given no branches, mirroring the TS Choice throwing.
func Choice(items ...*RailroadNode) (*RailroadNode, error) {
	if len(items) < 1 {
		return nil, &RailroadError{Message: "railroad: choice needs at least one branch"}
	}
	return &RailroadNode{Kind: KindChoice, Items: items}, nil
}

// MustChoice is Choice that panics with a RailroadError on no branches —
// the ergonomic form used internally where branch count is guaranteed.
func MustChoice(items ...*RailroadNode) *RailroadNode {
	n, err := Choice(items...)
	if err != nil {
		panic(err)
	}
	return n
}

// Optional builds an optional (bypassable) node.
func Optional(item *RailroadNode) *RailroadNode {
	return &RailroadNode{Kind: KindOptional, Item: item}
}

// OneOrMore builds a one-or-more repetition, with an optional rep node on
// the return path (e.g. a separator).
func OneOrMore(item *RailroadNode, rep *RailroadNode) *RailroadNode {
	return &RailroadNode{Kind: KindOneOrMore, Item: item, Rep: rep}
}

// ZeroOrMore builds a zero-or-more repetition (a bypassable OneOrMore).
func ZeroOrMore(item *RailroadNode, rep *RailroadNode) *RailroadNode {
	return &RailroadNode{Kind: KindZeroOrMore, Item: item, Rep: rep}
}

// Diagram builds a top-level diagram wrapper of nodes.
func Diagram(items ...*RailroadNode) *RailroadNode {
	return &RailroadNode{Kind: KindDiagram, Items: items}
}

// norm validates a node, returning a RailroadError for a nil/invalid node.
// (The TS norm also coerces a bare string to a Terminal; in Go terminals
// are always explicit, so this only validates.)
func norm(item *RailroadNode) (*RailroadNode, error) {
	if item == nil || item.Kind == "" {
		return nil, &RailroadError{Message: "railroad: invalid diagram node", Node: item}
	}
	return item, nil
}

// ---- text emitter --------------------------------------------------

// ToText renders a node as compact EBNF-ish text: terminals quoted, choice
// in (a | b), optional in [x], zero-or-more in {x}, one-or-more as x+.
// Mirrors ts/model.ts toText.
func ToText(node *RailroadNode) (string, error) {
	n, err := norm(node)
	if err != nil {
		return "", err
	}
	switch n.Kind {
	case KindTerminal:
		return jsonString(n.Text), nil
	case KindNonTerminal:
		return n.Text, nil
	case KindComment:
		return "/* " + n.Text + " */", nil
	case KindSkip:
		return "", nil
	case KindSeq:
		return joinText(n.Items, " ", true)
	case KindChoice:
		s, err := joinText(n.Items, " | ", false)
		if err != nil {
			return "", err
		}
		return "(" + s + ")", nil
	case KindOptional:
		s, err := ToText(n.Item)
		if err != nil {
			return "", err
		}
		return "[" + s + "]", nil
	case KindOneOrMore:
		s, err := ToText(n.Item)
		if err != nil {
			return "", err
		}
		out := s + "+"
		if n.Rep != nil {
			rs, err := ToText(n.Rep)
			if err != nil {
				return "", err
			}
			out += " /* " + rs + " */"
		}
		return out, nil
	case KindZeroOrMore:
		s, err := ToText(n.Item)
		if err != nil {
			return "", err
		}
		out := "{" + s + "}"
		if n.Rep != nil {
			rs, err := ToText(n.Rep)
			if err != nil {
				return "", err
			}
			out += " /* " + rs + " */"
		}
		return out, nil
	case KindDiagram:
		return joinText(n.Items, " ", true)
	default:
		return "", &RailroadError{Message: "railroad: unknown node kind " + jsonString(n.Kind), Node: n}
	}
}

func joinText(items []*RailroadNode, sep string, dropEmpty bool) (string, error) {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		s, err := ToText(it)
		if err != nil {
			return "", err
		}
		if dropEmpty && s == "" {
			continue
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, sep), nil
}

// ---- structural helpers --------------------------------------------

// nodeEqual reports deep structural equality, used by choice prefix/suffix
// factoring to detect shared leading/trailing elements. Mirrors nodeEqual.
func nodeEqual(a, b *RailroadNode) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case KindTerminal, KindNonTerminal, KindComment:
		return a.Text == b.Text
	case KindSkip:
		return true
	case KindSeq, KindChoice, KindDiagram:
		if len(a.Items) != len(b.Items) {
			return false
		}
		for i := range a.Items {
			if !nodeEqual(a.Items[i], b.Items[i]) {
				return false
			}
		}
		return true
	case KindOptional:
		return nodeEqual(a.Item, b.Item)
	case KindOneOrMore, KindZeroOrMore:
		repEq := (a.Rep == nil && b.Rep == nil) ||
			(a.Rep != nil && b.Rep != nil && nodeEqual(a.Rep, b.Rep))
		return repEq && nodeEqual(a.Item, b.Item)
	}
	return false
}

// jsonString returns the JSON-encoded (double-quoted, escaped) form of s,
// matching JSON.stringify for a string. Used by ToText terminal quoting.
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func writeJSON(b *strings.Builder, v any) {
	enc, _ := json.Marshal(v)
	b.Write(enc)
}
