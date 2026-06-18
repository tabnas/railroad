// Copyright (c) 2026 Richard Rodger and other contributors, MIT License

// ascii.go
// Vertical-flow ASCII renderer for railroad diagrams. Same flow model as
// the SVG renderer (sequences stack downward, choices fan sideways, loops
// return on the right), painted onto a character grid.
//
// Rails are tracked as direction bits per cell (Up/Down/Left/Right) so
// junctions resolve automatically to the right box-drawing glyph. Boxes
// draw literal corners/sides. Plain mode (the CLI --ascii-plain) swaps the
// Unicode glyph set for plain | - +.
//
// This file mirrors ts/src/ascii.ts.
package tabnasrailroad

import "strings"

// AsciiOptions configures the ASCII renderer.
type AsciiOptions struct {
	// Plain renders pure-ASCII glyphs (| - +) instead of Unicode.
	Plain bool
}

const (
	dirU = 1
	dirD = 2
	dirL = 4
	dirR = 8
)

var glyphTable = map[int]string{
	3: "│", 12: "─", 10: "┌", 6: "┐", 9: "└", 5: "┘",
	11: "├", 7: "┤", 14: "┬", 13: "┴", 15: "┼",
	1: "│", 2: "│", 4: "─", 8: "─",
}

func glyphFor(bits int, plain bool) string {
	if bits == 0 {
		return " "
	}
	if plain {
		v := bits & 3
		h := bits & 12
		if v != 0 && h != 0 {
			return "+"
		}
		if v != 0 {
			return "|"
		}
		return "-"
	}
	if g, ok := glyphTable[bits]; ok {
		return g
	}
	return " "
}

type glyphSet struct {
	tl, tr, bl, br     string
	rtl, rtr, rbl, rbr string
	v                  string
}

func glyphs(plain bool) glyphSet {
	if plain {
		return glyphSet{tl: "+", tr: "+", bl: "+", br: "+", rtl: "+", rtr: "+", rbl: "+", rbr: "+", v: "|"}
	}
	return glyphSet{tl: "┌", tr: "┐", bl: "└", br: "┘", rtl: "╭", rtr: "╮", rbl: "╰", rbr: "╯", v: "│"}
}

// canvas paints box-drawing onto a grid: per-cell direction bits resolve to
// junction glyphs; literal cells override.
type canvas struct {
	bits [][]int
	lit  [][]string // "" means unset
	set  [][]bool
}

func (cv *canvas) grow(r, c int) {
	for len(cv.bits) <= r {
		cv.bits = append(cv.bits, []int{})
		cv.lit = append(cv.lit, []string{})
		cv.set = append(cv.set, []bool{})
	}
	for len(cv.bits[r]) <= c {
		cv.bits[r] = append(cv.bits[r], 0)
		cv.lit[r] = append(cv.lit[r], "")
		cv.set[r] = append(cv.set[r], false)
	}
}

func (cv *canvas) line(r, c, mask int) {
	cv.grow(r, c)
	cv.bits[r][c] |= mask
}

func (cv *canvas) put(r, c int, ch string) {
	cv.grow(r, c)
	cv.lit[r][c] = ch
	cv.set[r][c] = true
}

func (cv *canvas) textAt(r, c int, s string) {
	runes := []rune(s)
	for i, ru := range runes {
		cv.put(r, c+i, string(ru))
	}
}

func (cv *canvas) vline(r1, r2, c int) {
	if r1 > r2 {
		r1, r2 = r2, r1
	}
	for r := r1; r <= r2; r++ {
		mask := 0
		if r > r1 {
			mask |= dirU
		}
		if r < r2 {
			mask |= dirD
		}
		cv.line(r, c, mask)
	}
}

func (cv *canvas) hline(c1, c2, r int) {
	if c1 > c2 {
		c1, c2 = c2, c1
	}
	for c := c1; c <= c2; c++ {
		mask := 0
		if c > c1 {
			mask |= dirL
		}
		if c < c2 {
			mask |= dirR
		}
		cv.line(r, c, mask)
	}
}

func (cv *canvas) render(plain bool) string {
	lines := make([]string, len(cv.bits))
	for r := range cv.bits {
		var line strings.Builder
		for c := 0; c < len(cv.bits[r]); c++ {
			if cv.set[r][c] {
				line.WriteString(cv.lit[r][c])
			} else {
				line.WriteString(glyphFor(cv.bits[r][c], plain))
			}
		}
		lines[r] = strings.TrimRight(line.String(), " \t\n\r\f\v")
	}
	return strings.Join(lines, "\n")
}

// ---- measure model -------------------------------------------------

type measureM struct {
	cols     int
	rows     int
	entryCol int
	exitCol  int
	paint    func(cv *canvas, x, y int)
}

const (
	asciiVG = 1 // vertical gap rows between stacked items
	asciiHG = 3 // horizontal gap cols between choice branches
)

func boxM(text string, terminal bool, g glyphSet) measureM {
	inner := " " + text + " "
	innerLen := len([]rune(inner))
	w := innerLen + 2
	mid := w / 2
	return measureM{
		cols: w, rows: 3, entryCol: mid, exitCol: mid,
		paint: func(cv *canvas, x, y int) {
			tl, tr, bl, br := g.tl, g.tr, g.bl, g.br
			if terminal {
				tl, tr, bl, br = g.rtl, g.rtr, g.rbl, g.rbr
			}
			cv.put(y, x, tl)
			cv.put(y, x+w-1, tr)
			cv.hline(x+1, x+w-2, y)
			cv.put(y+1, x, g.v)
			cv.put(y+1, x+w-1, g.v)
			cv.textAt(y+1, x+1, inner)
			cv.put(y+2, x, bl)
			cv.put(y+2, x+w-1, br)
			cv.hline(x+1, x+w-2, y+2)
		},
	}
}

func commentM(text string) measureM {
	s := "/* " + text + " */"
	sl := len([]rune(s))
	return measureM{
		cols: sl, rows: 1, entryCol: sl / 2, exitCol: sl / 2,
		paint: func(cv *canvas, x, y int) { cv.textAt(y, x, s) },
	}
}

func skipM() measureM {
	return measureM{cols: 1, rows: 1, entryCol: 0, exitCol: 0, paint: func(cv *canvas, x, y int) {
		cv.line(y, x, dirU|dirD)
	}}
}

func seqM(children []measureM) measureM {
	if len(children) == 0 {
		return skipM()
	}
	if len(children) == 1 {
		return children[0]
	}
	railCol := 0
	for _, c := range children {
		if c.entryCol > railCol {
			railCol = c.entryCol
		}
	}
	offs := make([]int, len(children))
	cols := 0
	for i, c := range children {
		offs[i] = railCol - c.entryCol
		if offs[i]+c.cols > cols {
			cols = offs[i] + c.cols
		}
	}
	rows := 0
	for _, c := range children {
		rows += c.rows
	}
	rows += asciiVG * (len(children) - 1)
	return measureM{
		cols: cols, rows: rows, entryCol: railCol, exitCol: railCol,
		paint: func(cv *canvas, x, y int) {
			cy := y
			for i, c := range children {
				if i > 0 {
					cv.vline(cy-asciiVG-1, cy, x+railCol)
				}
				c.paint(cv, x+offs[i], cy)
				cy += c.rows + asciiVG
			}
		},
	}
}

func choiceM(branches []measureM) measureM {
	if len(branches) == 1 {
		return branches[0]
	}
	n := len(branches)
	cols := 0
	for _, c := range branches {
		cols += c.cols
	}
	cols += asciiHG * (n - 1)
	maxR := 0
	for _, c := range branches {
		if c.rows > maxR {
			maxR = c.rows
		}
	}
	rows := 1 + maxR + 1
	bxs := make([]int, n)
	bx := 0
	for i, c := range branches {
		bxs[i] = bx
		bx += c.cols + asciiHG
	}
	relCenters := make([]int, n)
	for i, c := range branches {
		relCenters[i] = bxs[i] + c.entryCol
	}
	entryCol := int(roundHalfAwayFromZero(float64(relCenters[0]+relCenters[n-1]) / 2))
	return measureM{
		cols: cols, rows: rows, entryCol: entryCol, exitCol: entryCol,
		paint: func(cv *canvas, x, y int) {
			splitRow := y
			branchTop := y + 1
			mergeRow := y + 1 + maxR
			lo := x + entryCol
			hi := x + entryCol
			for _, c := range relCenters {
				if x+c < lo {
					lo = x + c
				}
				if x+c > hi {
					hi = x + c
				}
			}
			cv.hline(lo, hi, splitRow)
			cv.hline(lo, hi, mergeRow)
			for i, c := range branches {
				cx := x + bxs[i]
				ce := cx + c.entryCol
				cv.vline(splitRow, branchTop, ce)
				c.paint(cv, cx, branchTop)
				cv.vline(branchTop+c.rows-1, mergeRow, cx+c.exitCol)
			}
		},
	}
}

func oneOrMoreM(item measureM, repLabel string) measureM {
	railGap := 0
	if repLabel != "" {
		railGap = len([]rune(repLabel)) + 1
	}
	cols := item.cols + 1 + railGap
	rows := item.rows + 2
	railCol := item.cols
	return measureM{
		cols: cols, rows: rows, entryCol: item.entryCol, exitCol: item.exitCol,
		paint: func(cv *canvas, x, y int) {
			topRow := y
			itemTop := y + 1
			itemBot := y + item.rows
			botRow := y + item.rows + 1
			item.paint(cv, x, itemTop)
			cv.vline(topRow, itemTop, x+item.entryCol)
			cv.vline(itemBot, botRow, x+item.exitCol)
			rc := x + railCol
			cv.hline(x+item.entryCol, rc, topRow)
			cv.hline(x+item.exitCol, rc, botRow)
			cv.vline(topRow, botRow, rc)
			if repLabel != "" {
				cv.textAt(y+rows/2, rc+1, repLabel)
			}
		},
	}
}

func labelOf(node *RailroadNode) string {
	if node == nil {
		return ""
	}
	if node.Kind == KindTerminal || node.Kind == KindNonTerminal {
		return node.Text
	}
	return "+"
}

func measure(node *RailroadNode, g glyphSet) (measureM, error) {
	switch node.Kind {
	case KindTerminal:
		return boxM(jsonString(node.Text), true, g), nil
	case KindNonTerminal:
		return boxM(node.Text, false, g), nil
	case KindComment:
		return commentM(node.Text), nil
	case KindSkip:
		return skipM(), nil
	case KindSeq:
		ch, err := measureChildren(node.Items, g)
		if err != nil {
			return measureM{}, err
		}
		return seqM(ch), nil
	case KindChoice:
		ch, err := measureChildren(node.Items, g)
		if err != nil {
			return measureM{}, err
		}
		return choiceM(ch), nil
	case KindOptional:
		item, err := measure(node.Item, g)
		if err != nil {
			return measureM{}, err
		}
		return choiceM([]measureM{item, skipM()}), nil
	case KindOneOrMore:
		item, err := measure(node.Item, g)
		if err != nil {
			return measureM{}, err
		}
		return oneOrMoreM(item, labelOf(node.Rep)), nil
	case KindZeroOrMore:
		item, err := measure(node.Item, g)
		if err != nil {
			return measureM{}, err
		}
		return choiceM([]measureM{oneOrMoreM(item, labelOf(node.Rep)), skipM()}), nil
	case KindDiagram:
		ch, err := measureChildren(node.Items, g)
		if err != nil {
			return measureM{}, err
		}
		return seqM(ch), nil
	default:
		return measureM{}, &RailroadError{Message: "railroad: unknown node kind " + jsonString(node.Kind), Node: node}
	}
}

func measureChildren(items []*RailroadNode, g glyphSet) ([]measureM, error) {
	out := make([]measureM, len(items))
	for i, n := range items {
		m, err := measure(n, g)
		if err != nil {
			return nil, err
		}
		out[i] = m
	}
	return out, nil
}

// RenderNodeAscii renders a single node to an ASCII block.
func RenderNodeAscii(node *RailroadNode, opts ...AsciiOptions) (string, error) {
	var o AsciiOptions
	if len(opts) > 0 {
		o = opts[0]
	}
	plain := o.Plain
	g := glyphs(plain)
	n, err := norm(node)
	if err != nil {
		return "", err
	}
	cv := &canvas{}
	m, err := measure(n, g)
	if err != nil {
		return "", err
	}
	cv.line(0, m.entryCol, dirD)
	m.paint(cv, 0, 1)
	cv.line(1, m.entryCol, dirU)
	cv.line(m.rows, m.exitCol, dirD)
	cv.line(m.rows+1, m.exitCol, dirU)
	return cv.render(plain), nil
}

// ModelToAscii renders a whole grammar: each rule as a titled vertical block.
func ModelToAscii(model *GrammarModel, opts ...AsciiOptions) (string, error) {
	var o AsciiOptions
	if len(opts) > 0 {
		o = opts[0]
	}
	plain := o.Plain
	g := glyphs(plain)
	names := orderRules(model)
	blocks := []string{}
	for _, name := range names {
		cv := &canvas{}
		m, err := measure(model.Rules[name], g)
		if err != nil {
			return "", err
		}
		cv.line(0, m.entryCol, dirD)
		m.paint(cv, 0, 1)
		cv.line(1, m.entryCol, dirU)
		cv.line(m.rows, m.exitCol, dirD)
		cv.line(m.rows+1, m.exitCol, dirU)
		blocks = append(blocks, name+":\n"+cv.render(plain))
	}
	keyBlock := func(title string, entries []LegendEntry) string {
		w := 0
		for _, e := range entries {
			if l := len([]rune(e.Token)); l > w {
				w = l
			}
		}
		lines := make([]string, len(entries))
		for i, e := range entries {
			lines[i] = "  " + padEnd(e.Token, w) + " = " + e.Meaning
		}
		return title + ":\n" + strings.Join(lines, "\n")
	}
	if len(model.Legend) > 0 {
		blocks = append(blocks, keyBlock("Tokens", model.Legend))
	}
	if len(model.Ignored) > 0 {
		blocks = append(blocks, keyBlock("Ignored tokens", model.Ignored))
	}
	return strings.Join(blocks, "\n\n"), nil
}

func padEnd(s string, w int) string {
	for len([]rune(s)) < w {
		s += " "
	}
	return s
}

// roundHalfAwayFromZero mirrors JS Math.round: ties round toward +Infinity.
func roundHalfAwayFromZero(f float64) float64 {
	return float64(int64(f + 0.5))
}
