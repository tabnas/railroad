// Copyright (c) 2026 Richard Rodger and other contributors, MIT License

// svg.go
// Vertical-flow SVG renderer for railroad diagrams. Flow runs top->bottom:
// sequences stack vertically, choice branches fan out side-by-side, and
// optional / repetition rails run parallel on the side. This biases the
// output tall-and-narrow, which suits laptop browsers and phones.
//
// Each node measures to a vLayout{width, height, entryX, exitX, draw}. The
// rail enters the top edge at entryX and leaves the bottom edge at exitX
// (both equal — all nodes are horizontally symmetric). draw(x,y) emits SVG
// with the node's bounding box at top-left (x,y).
//
// ModelToSvg stacks one titled, anchored sub-diagram per rule and turns
// nonterminal boxes into <a href="#rule"> links.
//
// This file mirrors ts/src/svg.ts.
package tabnasrailroad

import (
	"math"
	"strconv"
	"strings"
)

// ---- geometry constants --------------------------------------------
const (
	svgCHARW   = 8.0
	svgPADX    = 10.0
	svgBOXH    = 26.0
	svgMINW    = 30.0
	svgVGAP    = 18.0 // vertical gap between stacked items / split-merge stubs
	svgHGAP    = 26.0 // horizontal gap between choice branches
	svgAR      = 10.0 // loop rail inset
	svgPAD     = 16.0 // outer padding
	svgTITLE_H = 26.0 // height reserved for a rule title
	svgTRACK   = 34.0 // track gap
	svgLEAD    = 14.0 // rail lead between a cap dot and the content
)

// SvgOptions configures the SVG renderer.
type SvgOptions struct {
	// LinkFor maps a nonterminal name to an href (whole-grammar linking).
	// Returns ("", false) when the name should not be linked.
	LinkFor func(name string) (string, bool)
}

type vLayout struct {
	width  float64
	height float64
	entryX float64
	exitX  float64
	isSkip bool // a bypass branch — gets a tighter gap in a choice
	draw   func(x, y float64) string
}

// ---- primitives ----------------------------------------------------

func svgEsc(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;")
	return r.Replace(s)
}

func svgPath(d, cls string) string {
	return `<path class="` + cls + `" d="` + d + `"/>`
}

func svgVline(x, y1, y2 float64) string {
	if y1 == y2 {
		return ""
	}
	return svgPath("M"+num(x)+" "+num(y1)+"V"+num(y2), "rr-line")
}

func svgHline(x1, x2, y float64) string {
	if x1 == x2 {
		return ""
	}
	return svgPath("M"+num(x1)+" "+num(y)+"H"+num(x2), "rr-line")
}

func svgCap(x, y float64) string {
	return `<circle class="rr-cap" cx="` + num(x) + `" cy="` + num(y) + `" r="3"/>`
}

// ---- node layouts --------------------------------------------------

func boxLayout(text, cls string, terminal bool, href string, hasHref bool) vLayout {
	w := math.Max(float64(len([]rune(text)))*svgCHARW+2*svgPADX, svgMINW)
	h := svgBOXH
	return vLayout{
		width: w, height: h, entryX: w / 2, exitX: w / 2,
		draw: func(x, y float64) string {
			r := 4.0
			if terminal {
				r = h / 2
			}
			inner := `<rect class="` + cls + `" x="` + num(x) + `" y="` + num(y) +
				`" width="` + num(w) + `" height="` + num(h) + `" rx="` + num(r) + `" ry="` + num(r) + `"/>` +
				`<text class="rr-label" x="` + num(x+w/2) + `" y="` + num(y+h/2) + `">` + svgEsc(text) + `</text>`
			if hasHref {
				return `<a href="` + svgEsc(href) + `">` + inner + `</a>`
			}
			return inner
		},
	}
}

func commentLayout(text string) vLayout {
	w := math.Max(float64(len([]rune(text)))*svgCHARW+2*svgPADX, svgMINW)
	return vLayout{
		width: w, height: svgBOXH, entryX: w / 2, exitX: w / 2,
		draw: func(x, y float64) string {
			return svgVline(x+w/2, y, y+svgBOXH) +
				`<text class="rr-comment" x="` + num(x+w/2) + `" y="` + num(y+svgBOXH/2) + `">` + svgEsc(text) + `</text>`
		},
	}
}

func skipLayout() vLayout {
	w, h := 16.0, 12.0
	return vLayout{
		width: w, height: h, entryX: w / 2, exitX: w / 2, isSkip: true,
		draw: func(x, y float64) string { return svgVline(x+w/2, y, y+h) },
	}
}

func seqLayout(children []vLayout) vLayout {
	if len(children) == 0 {
		return skipLayout()
	}
	if len(children) == 1 {
		return children[0]
	}
	railX := 0.0
	for _, c := range children {
		railX = math.Max(railX, c.entryX)
	}
	offs := make([]float64, len(children))
	width := 0.0
	for i, c := range children {
		offs[i] = railX - c.entryX
		width = math.Max(width, offs[i]+c.width)
	}
	height := 0.0
	for _, c := range children {
		height += c.height
	}
	height += svgVGAP * float64(len(children)-1)
	return vLayout{
		width: width, height: height, entryX: railX, exitX: railX,
		draw: func(x, y float64) string {
			var out strings.Builder
			cy := y
			for i, c := range children {
				if i > 0 {
					out.WriteString(svgVline(x+railX, cy-svgVGAP, cy))
				}
				out.WriteString(c.draw(x+offs[i], cy))
				cy += c.height + svgVGAP
			}
			return out.String()
		},
	}
}

func choiceLayout(branches []vLayout) vLayout {
	if len(branches) == 1 {
		return branches[0]
	}
	n := len(branches)
	maxBH := 0.0
	for _, c := range branches {
		maxBH = math.Max(maxBH, c.height)
	}
	height := 4*svgVGAP + maxBH
	bxs := make([]float64, n)
	bx := 0.0
	for i, c := range branches {
		bxs[i] = bx
		gap := svgHGAP
		if i+1 < n && (c.isSkip || branches[i+1].isSkip) {
			gap = svgAR
		}
		bx += c.width + gap
	}
	width := bxs[n-1] + branches[n-1].width
	relCenters := make([]float64, n)
	for i, c := range branches {
		relCenters[i] = bxs[i] + c.entryX
	}
	entryX := (relCenters[0] + relCenters[n-1]) / 2
	return vLayout{
		width: width, height: height, entryX: entryX, exitX: entryX,
		draw: func(x, y float64) string {
			splitY := y + svgVGAP
			branchY := splitY + svgVGAP
			mergeY := branchY + maxBH + svgVGAP
			lo := x + entryX
			hi := x + entryX
			for _, c := range relCenters {
				lo = math.Min(lo, x+c)
				hi = math.Max(hi, x+c)
			}
			var out strings.Builder
			out.WriteString(svgVline(x+entryX, y, splitY))
			out.WriteString(svgHline(lo, hi, splitY))
			out.WriteString(svgHline(lo, hi, mergeY))
			out.WriteString(svgVline(x+entryX, mergeY, y+height))
			for i, c := range branches {
				cx := x + bxs[i]
				bex := cx + c.entryX
				out.WriteString(svgVline(bex, splitY, branchY))
				out.WriteString(c.draw(cx, branchY))
				out.WriteString(svgVline(bex, branchY+c.height, mergeY))
			}
			return out.String()
		},
	}
}

func oneOrMoreLayout(item vLayout, rep *vLayout) vLayout {
	repW := 0.0
	if rep != nil {
		repW = rep.width
	}
	railX0 := item.width + svgAR + repW/2
	width := item.width + 2*svgAR + repW
	topGap := svgVGAP
	botGap := svgVGAP
	height := topGap + item.height + botGap
	entryX := item.entryX
	return vLayout{
		width: width, height: height, entryX: entryX, exitX: item.exitX,
		draw: func(x, y float64) string {
			mainX := x + entryX
			itemY := y + topGap
			railX := x + railX0
			loopTopY := y + topGap/2
			loopBottomY := y + height - botGap/2
			var out strings.Builder
			out.WriteString(svgVline(mainX, y, itemY))
			out.WriteString(item.draw(x, itemY))
			out.WriteString(svgVline(x+item.exitX, itemY+item.height, y+height))
			out.WriteString(svgHline(mainX, railX, loopBottomY))
			out.WriteString(svgHline(mainX, railX, loopTopY))
			if rep != nil {
				repH := rep.height
				midY := (loopTopY + loopBottomY) / 2
				rTop := midY - repH/2
				rBot := midY + repH/2
				out.WriteString(svgVline(railX, rBot, loopBottomY))
				out.WriteString(svgVline(railX, loopTopY, rTop))
				out.WriteString(rep.draw(railX-rep.entryX, rTop))
			} else {
				out.WriteString(svgVline(railX, loopTopY, loopBottomY))
			}
			return out.String()
		},
	}
}

func layoutNode(node *RailroadNode, opts SvgOptions) (vLayout, error) {
	switch node.Kind {
	case KindTerminal:
		return boxLayout(node.Text, "rr-term", true, "", false), nil
	case KindNonTerminal:
		href, has := "", false
		if opts.LinkFor != nil {
			href, has = opts.LinkFor(node.Text)
		}
		return boxLayout(node.Text, "rr-nonterm", false, href, has), nil
	case KindComment:
		return commentLayout(node.Text), nil
	case KindSkip:
		return skipLayout(), nil
	case KindSeq:
		ch, err := layoutChildren(node.Items, opts)
		if err != nil {
			return vLayout{}, err
		}
		return seqLayout(ch), nil
	case KindChoice:
		ch, err := layoutChildren(node.Items, opts)
		if err != nil {
			return vLayout{}, err
		}
		return choiceLayout(ch), nil
	case KindOptional:
		item, err := layoutNode(node.Item, opts)
		if err != nil {
			return vLayout{}, err
		}
		return choiceLayout([]vLayout{item, skipLayout()}), nil
	case KindOneOrMore:
		item, err := layoutNode(node.Item, opts)
		if err != nil {
			return vLayout{}, err
		}
		rep, err := layoutRep(node.Rep, opts)
		if err != nil {
			return vLayout{}, err
		}
		return oneOrMoreLayout(item, rep), nil
	case KindZeroOrMore:
		item, err := layoutNode(node.Item, opts)
		if err != nil {
			return vLayout{}, err
		}
		rep, err := layoutRep(node.Rep, opts)
		if err != nil {
			return vLayout{}, err
		}
		return choiceLayout([]vLayout{oneOrMoreLayout(item, rep), skipLayout()}), nil
	case KindDiagram:
		ch, err := layoutChildren(node.Items, opts)
		if err != nil {
			return vLayout{}, err
		}
		return seqLayout(ch), nil
	default:
		return vLayout{}, &RailroadError{Message: "railroad: unknown node kind " + jsonString(node.Kind), Node: node}
	}
}

func layoutChildren(items []*RailroadNode, opts SvgOptions) ([]vLayout, error) {
	out := make([]vLayout, len(items))
	for i, n := range items {
		l, err := layoutNode(n, opts)
		if err != nil {
			return nil, err
		}
		out[i] = l
	}
	return out, nil
}

func layoutRep(rep *RailroadNode, opts SvgOptions) (*vLayout, error) {
	if rep == nil {
		return nil, nil
	}
	l, err := layoutNode(rep, opts)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

// ---- document assembly ---------------------------------------------

const svgSTYLE = "svg.railroad{background:#fff;font-family:monospace;font-size:13px}" +
	".rr-line{stroke:#334;stroke-width:2;fill:none}" +
	".rr-cap{fill:#334}" +
	".rr-term{fill:#e8f0ff;stroke:#334;stroke-width:2}" +
	".rr-nonterm{fill:#fff7e8;stroke:#334;stroke-width:2}" +
	".rr-label{fill:#111;text-anchor:middle;dominant-baseline:middle}" +
	".rr-comment{fill:#666;font-style:italic;text-anchor:middle;dominant-baseline:middle}" +
	".rr-title{fill:#113;font-weight:bold;font-size:15px}" +
	".rr-legend{fill:#333;font-size:12px}" +
	".rr-legend-tok{fill:#113;font-weight:bold}" +
	"a:hover .rr-nonterm{fill:#ffe6b3;cursor:pointer}"

func svgDoc(body string, w, h float64) string {
	W := int(math.Ceil(w))
	H := int(math.Ceil(h))
	return `<svg xmlns="http://www.w3.org/2000/svg" class="railroad" ` +
		`width="` + strconv.Itoa(W) + `" height="` + strconv.Itoa(H) + `" viewBox="0 0 ` +
		strconv.Itoa(W) + ` ` + strconv.Itoa(H) + `"><style>` + svgSTYLE + `</style><g>` + body + `</g></svg>`
}

// withCaps draws a layout with entry/exit cap dots, separated from the
// content by a short rail lead so the dots never sit on top of a box.
func withCaps(L vLayout, x, top float64) string {
	ct := top + svgLEAD
	cb := ct + L.height
	ex := x + L.entryX
	xx := x + L.exitX
	return svgCap(ex, top) + svgVline(ex, top, ct) +
		L.draw(x, ct) +
		svgVline(xx, cb, cb+svgLEAD) + svgCap(xx, cb+svgLEAD)
}

// RenderNodeSvg renders a single node (wrapped in its own SVG document).
func RenderNodeSvg(node *RailroadNode, opts ...SvgOptions) (string, error) {
	var o SvgOptions
	if len(opts) > 0 {
		o = opts[0]
	}
	n, err := norm(node)
	if err != nil {
		return "", err
	}
	L, err := layoutNode(n, o)
	if err != nil {
		return "", err
	}
	return svgDoc(withCaps(L, svgPAD, svgPAD), L.width+2*svgPAD, L.height+2*svgLEAD+2*svgPAD), nil
}

// ModelToSvg renders a whole grammar: a vertical stack of titled, anchored
// rule tracks; nonterminal boxes link to the referenced rule's track.
func ModelToSvg(model *GrammarModel, _opts ...SvgOptions) (string, error) {
	ruleNames := map[string]bool{}
	for name := range model.Rules {
		ruleNames[name] = true
	}
	linkFor := func(name string) (string, bool) {
		if ruleNames[name] {
			return "#" + name, true
		}
		return "", false
	}
	opts := SvgOptions{LinkFor: linkFor}

	type track struct {
		name string
		L    vLayout
		h    float64
	}
	order := orderRules(model)
	tracks := make([]track, 0, len(order))
	for _, name := range order {
		L, err := layoutNode(model.Rules[name], opts)
		if err != nil {
			return "", err
		}
		tracks = append(tracks, track{name, L, svgTITLE_H + L.height + 2*svgLEAD})
	}

	twoCol := len(tracks) > 1
	total := 0.0
	for _, t := range tracks {
		total += t.h + svgTRACK
	}
	cols := [2][]track{}
	acc := 0.0
	ci := 0
	for _, t := range tracks {
		cols[ci] = append(cols[ci], t)
		acc += t.h + svgTRACK
		if twoCol && ci == 0 && acc >= total/2 {
			ci = 1
		}
	}

	const colGap = 48.0
	x := svgPAD
	pageH := 0.0
	var body strings.Builder
	for _, col := range cols {
		if len(col) == 0 {
			continue
		}
		y := svgPAD
		colW := 0.0
		for _, t := range col {
			dy := y + svgTITLE_H
			body.WriteString(`<g id="` + svgEsc(t.name) + `">`)
			body.WriteString(`<text class="rr-title" x="` + num(x) + `" y="` + num(y+15) + `">` + svgEsc(t.name) + `</text>`)
			body.WriteString(withCaps(t.L, x, dy))
			body.WriteString(`</g>`)
			colW = math.Max(colW, t.L.width)
			y = dy + t.L.height + 2*svgLEAD + svgTRACK
		}
		pageH = math.Max(pageH, y-svgTRACK)
		x += colW + colGap
	}
	pageW := x - colGap + svgPAD

	renderKey := func(title string, entries []LegendEntry) {
		ly := pageH + svgTRACK
		body.WriteString(`<text class="rr-title" x="` + num(svgPAD) + `" y="` + num(ly+15) + `">` + svgEsc(title) + `</text>`)
		ly += svgTITLE_H
		for _, e := range entries {
			body.WriteString(`<text class="rr-legend" x="` + num(svgPAD) + `" y="` + num(ly+11) + `">` +
				`<tspan class="rr-legend-tok">` + svgEsc(e.Token) + `</tspan>  —  ` + svgEsc(e.Meaning) + `</text>`)
			w := svgPAD + float64(len([]rune(e.Token))+len([]rune(e.Meaning))+5)*7.5 + svgPAD
			pageW = math.Max(pageW, w)
			ly += 18
		}
		pageH = ly
	}
	if len(model.Legend) > 0 {
		renderKey("Tokens", model.Legend)
	}
	if len(model.Ignored) > 0 {
		renderKey("Ignored tokens", model.Ignored)
	}
	return svgDoc(body.String(), pageW, pageH+svgPAD), nil
}

func orderRules(model *GrammarModel) []string {
	names := model.RuleOrder
	if len(names) == 0 {
		for k := range model.Rules {
			names = append(names, k)
		}
	}
	if model.Start != "" {
		for _, n := range names {
			if n == model.Start {
				out := []string{model.Start}
				for _, m := range names {
					if m != model.Start {
						out = append(out, m)
					}
				}
				return out
			}
		}
	}
	return names
}

// num formats a float for SVG coordinates the way JS Number->string does:
// integers without a decimal point, fractionals with the minimal digits.
func num(f float64) string {
	if f == math.Trunc(f) && !math.IsInf(f, 0) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}
