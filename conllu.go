package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html"
	"io/ioutil"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/rug-compling/alpinods"
	"github.com/ungerik/go-cairo"
)

const (
	FONT = "FreeSans, Arial, Helvetica, sans-serif"

	MIN_NODE_WIDTH = 80 // minimale breedte van nodes
	NODE_HEIGHT    = 48 // hoogte van nodes
	NODE_SPACING   = 8  // horizontale ruimte tussen nodes
	NODE_FONT_SIZE = 16 // fontsize in nodes
	NODE_TWEEK     = 2  // schuif teksten verticaal naar elkaar toe

	LVL_HEIGHT             = 40      // hoogteverschil tussen edges van opeenvolgend niveau
	EDGE_FONT_SIZE         = 16      // fontsize van label bij edge
	EDGE_FONT_OFFSET       = 8       // hoogte van baseline van label boven edge
	EDGE_FONT_WHITE_MARGIN = 2       // extra witruimte om label bij edge
	EDGE_LBL_BACKGROUND    = "white" // kleur van rechthoek achter label boven edge
	EDGE_LBL_OPACITY       = .9      // doorzichtigheid van rechthoek achter label boven edge
	EDGE_DROP              = 80      // edge curvature: te veel, en lijnen steken onder de figuur uit

	MULTI_SKIP   = 4
	MULTI_HEIGHT = 28

	MARGIN = 4 // extra ruimte rond hele figuur

	TESTING = false
)

type UdItem struct {
	lineno   int
	here     string
	there    string
	end      int
	enhanced bool
	word     string
	lemma    string
	postag   string
	xpostag  string
	attribs  string
	rel      string
	deps     string
	x1, x2   int
}

type Dependency struct {
	end     int
	headpos int
	rel     [2]string
	dist    int
	lvl     int
}

type Anchor struct {
	dist  int
	level int
}

type Line struct {
	text   string
	lineno int
}

type Multi struct {
	id     string
	word   string
	lineno int
}

var (
	dependencies []*Dependency
	anchors      [][]Anchor
)

func vizUD(chIn <-chan Item, chOut chan<- Item, extended bool, format string) {
	var tempfile string
	if format != "svg" {
		tmp := os.Getenv("TEMP")
		if tmp == "" {
			tmp = os.Getenv("TMP")
		}
		f, err := ioutil.TempFile(tmp, "alto")
		x(err)
		tempfile = f.Name()
		f.Close()
	}

	for item := range chIn {
		var alpino alpinods.AlpinoDS
		x(xml.Unmarshal([]byte(item.data), &alpino))
		if alpino.Conllu != nil && alpino.Conllu.Status == "OK" {
			chOut <- Item{
				name:  fmt.Sprintf("%s.%s", item.oriname, format),
				data:  conllu2image(alpino.Conllu.Conllu, extended, format, tempfile),
				match: make([]string, 0),
			}
		}
	}
	if format != "svg" {
		os.Remove(tempfile)
	}
	close(chOut)
}

func conllu2image(conllu string, enhanced bool, format string, tempfile string) string {
	var lines []Line
	n := 0
	for _, s := range strings.Split(conllu, "\n") {
		s := strings.TrimSpace(s)
		if s != "" {
			n++
			lines = append(lines, Line{text: s, lineno: n})
		}
	}

	dependencies = make([]*Dependency, 0)

	hasEnhanced := false
	items := make([]*UdItem, 0)
	positions := map[string]int{
		"0": 0,
	}
	multis := make([]Multi, 0)

	n = 0
	for _, line := range lines {
		aa := strings.Split(line.text, "\t")
		if len(aa) < 2 {
			aa = strings.Fields(line.text)
		}
		if len(aa) != 10 {
			x(fmt.Errorf("Line %d: Wrong number of fields", line.lineno))
		}
		for i, a := range aa {
			aa[i] = strings.TrimSpace(a)
		}

		if strings.Contains(aa[0], "-") {
			multis = append(multis, Multi{id: aa[0], word: aa[1], lineno: line.lineno})
			continue
		}
		at := ""
		if aa[5] != "_" {
			at = strings.Replace(strings.Replace(aa[5], "|", ", ", -1), "=", ": ", -1)
		}
		if enhanced || !strings.Contains(aa[0], ".") {
			items = append(items, &UdItem{
				lineno:   line.lineno,
				here:     aa[0],
				word:     aa[1],
				lemma:    aa[2],
				postag:   aa[3],
				xpostag:  aa[4],
				attribs:  at,
				there:    aa[6],
				rel:      aa[7],
				deps:     aa[8],
				enhanced: strings.Contains(aa[0], "."),
			})
			n++
		}
		positions[aa[0]] = n
	}

	for i, item := range items {
		end := positions[item.here]
		items[i].end = end

		if !enhanced {
			if !item.enhanced {
				headpos, ok := positions[item.there]
				if !ok {
					x(fmt.Errorf("Line %d: Unknown head position %s", item.lineno, item.there))
				}
				// if headpos != 0 {
				dependencies = append(dependencies, &Dependency{
					end:     end,
					headpos: headpos,
					rel:     [2]string{item.rel, ""},
					dist:    abs(end - headpos),
				})
				// }
			}
		}

		if enhanced {
			if item.deps != "_" {
				parts := strings.Split(item.deps, "|")
				for _, part := range parts {
					a := strings.SplitN(part, ":", 2)
					if len(a) != 2 {
						x(fmt.Errorf("Line %d: Invalid dependency: %s", item.lineno, part))
					}
					headpos, ok := positions[a[0]]
					if !ok {
						x(fmt.Errorf("Line %d: Unknown head position %s", item.lineno, a[0]))
					}
					dependencies = append(dependencies, &Dependency{
						end:     end,
						headpos: headpos,
						rel:     [2]string{"", a[1]},
						dist:    abs(end - headpos),
					})
					hasEnhanced = true
				}
			}
		}
	}

	// dubbele edges samenvoegen
	for i := 0; i < len(dependencies); i++ {
		d1 := dependencies[i]
		if d1.rel[0] != "" {
			for j := 0; j < len(dependencies); j++ {
				if i == j {
					continue
				}
				d2 := dependencies[j]
				if d2.rel[1] != "" && d1.end == d2.end && d1.headpos == d2.headpos && d1.dist == d2.dist {
					d1.rel[1] = d2.rel[1]
					dependencies = append(dependencies[:j], dependencies[j+1:]...)
					if j < i {
						i--
					}
					break
				}
			}
		}
	}

	// posities van de nodes

	sort.Slice(items, func(i, j int) bool { return items[i].end < items[j].end })
	width := MARGIN
	for i, item := range items {
		if item.end != i+1 {
			x(fmt.Errorf("Line %d: Wrong index: %d != %d", item.lineno, item.end, i+1))
		}
		item.x1 = width
		w1, _, _ := textwidth(item.postag+" i", NODE_FONT_SIZE, false)
		w2, _, _ := textwidth(item.word+" i", NODE_FONT_SIZE, false)
		item.x2 = width + 24 + max(MIN_NODE_WIDTH, w1, w2)
		width = item.x2 + NODE_SPACING
	}
	width -= NODE_SPACING
	width += MARGIN

	// hoogtes van de edges en aangrijppunten van de edges

	anchors = make([][]Anchor, len(items))
	for i := range items {
		anchors[i] = make([]Anchor, 0)
	}

	sort.Slice(dependencies, func(i, j int) bool { return dependencies[i].dist < dependencies[j].dist })
	grid := make([][]bool, len(items))
	for i := range grid {
		grid[i] = make([]bool, 2*len(items))
	}
	for i, dep := range dependencies {
		if dep.headpos == 0 {
			anchors[dep.end-1] = append(anchors[dep.end-1], Anchor{})
			continue
		}
		i1, i2 := dep.end-1, dep.headpos-1
		if i1 > i2 {
			i1, i2 = i2, i1
		}
		lvl := 0
		for {
			ok := true
			for i := i1; i < i2; i++ {
				if grid[i][lvl] {
					ok = false
					break
				}
			}
			if ok {
				for i := i1; i < i2; i++ {
					grid[i][lvl] = true
				}
				break
			}
			lvl++
		}
		dependencies[i].lvl = lvl
		anchors[dep.end-1] = append(anchors[dep.end-1], Anchor{dist: dep.headpos - dep.end, level: lvl})
		anchors[dep.headpos-1] = append(anchors[dep.headpos-1], Anchor{dist: dep.end - dep.headpos, level: lvl})
	}

	maxlvl := 0
	for _, dep := range dependencies {
		maxlvl = max(maxlvl, dep.lvl)
	}
	if hasEnhanced {
		maxlvl++
	} else {
		maxlvl++
	}

	// correctie voor root-dependencies
	for i, dep := range dependencies {
		if dep.headpos == 0 {
			dependencies[i].lvl = maxlvl
		}
	}
	for key, anchor := range anchors {
		for i, a := range anchor {
			if a.dist == 0 {
				anchors[key][i].level = maxlvl
			}
		}
	}

	for n := range anchors {
		sort.Slice(anchors[n], func(i, j int) bool {
			a1 := anchors[n][i]
			a2 := anchors[n][j]
			if a1.dist == 0 {
				return a2.dist > 0
			}
			if a2.dist == 0 {
				return a1.dist < 0
			}
			if a1.dist == a2.dist {
				if a1.dist < 0 {
					return a1.level < a2.level
				}
				return a1.level > a2.level
			}
			if a1.dist < 0 {
				if a2.dist > 0 {
					return true
				}
				if a1.dist < a2.dist {
					return false
				}
				return true
			}
			if a2.dist < 0 {
				return false
			}
			if a1.dist < a2.dist {
				return false
			}
			return true
		})
	}

	height := MARGIN + EDGE_FONT_SIZE + EDGE_FONT_OFFSET + LVL_HEIGHT*(maxlvl+1) + NODE_HEIGHT + MARGIN
	if len(multis) > 0 {
		height += MULTI_HEIGHT + MULTI_SKIP
	}

	// begin uitvoer

	isSVG := false
	var fp bytes.Buffer
	var surface *cairo.Surface
	switch format {
	case "svg":
		isSVG = true
	case "png":
		surface = cairo.NewSurface(cairo.FORMAT_ARGB32, width, height)
	case "eps":
		surface = cairo.NewEPSSurface(tempfile, float64(width), float64(height), cairo.PS_LEVEL_3)
	case "pdf":
		surface = cairo.NewPDFSurface(tempfile, float64(width), float64(height), cairo.PDF_VERSION_1_5)
	}

	if isSVG {
		fmt.Fprintf(&fp, `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d">
`, width, height)

		if TESTING {
			fmt.Fprintf(&fp, "<rect x=\"0\" y=\"0\" width=\"%d\" height=\"%d\" fill=\"green\" />\n", width, height)
		}
	}

	// edges

	variant := 0
	if enhanced {
		variant = 1
	}

	if isSVG {
		fmt.Fprint(&fp, "<g fill=\"none\" stroke=\"black\" stroke-width=\"1\">\n")
	} else {
		surface.SetSourceRGBA(0, 0, 0, 1)
		surface.SetLineWidth(1)
	}
	for _, dep := range dependencies {
		if dep.rel[variant] == "" {
			continue
		}
		i1, i2 := dep.end-1, dep.headpos-1
		if dep.headpos == 0 {
			i2 = i1
		}
		d1 := float64(items[i1].x2-items[i1].x1) - 20
		x1 := items[i1].x1 + 10 + int(d1*anchor(i1, i2, dep.lvl))
		d2 := float64(items[i2].x2-items[i2].x1) - 20
		x2 := items[i2].x1 + 10 + int(d2*anchor(i2, i1, dep.lvl))
		y1 := MARGIN + EDGE_FONT_SIZE + EDGE_FONT_OFFSET + LVL_HEIGHT*(maxlvl+1)
		y2 := MARGIN + EDGE_FONT_SIZE + EDGE_FONT_OFFSET + LVL_HEIGHT*(maxlvl-dep.lvl)
		if dep.headpos == 0 {
			y2 = MARGIN + EDGE_FONT_SIZE + EDGE_FONT_OFFSET
			if isSVG {
				fmt.Fprintf(&fp,
					"<path d=\"M%d %d L%d %d\" />\n",
					x1, y1, // M
					x1, y2) // L
			} else {
				surface.MoveTo(float64(x1), float64(y1))
				surface.LineTo(float64(x1), float64(y2))
				surface.Stroke()
			}
		} else {
			if isSVG {
				fmt.Fprintf(&fp,
					"<path d=\"M%d %d L%d %d C%d %d %d %d %d %d C%d %d %d %d %d %d L%d %d\" />\n",
					x1, y1, // M
					x1, y2+EDGE_DROP, // L
					x1, y2, // C
					x1, y2,
					(x1+x2)/2, y2,
					x2, y2, // C
					x2, y2,
					x2, y2+EDGE_DROP,
					x2, y1) // L
			} else {
				surface.MoveTo(float64(x1), float64(y1))
				surface.LineTo(float64(x1), float64(y2+EDGE_DROP))
				surface.CurveTo(float64(x1), float64(y2), float64(x1), float64(y2), float64((x1+x2)/2), float64(y2))
				surface.CurveTo(float64(x2), float64(y2), float64(x2), float64(y2), float64(x2), float64(y2+EDGE_DROP))
				surface.LineTo(float64(x2), float64(y1))
				surface.Stroke()
			}
		}
	}
	if isSVG {
		fmt.Fprint(&fp, "</g>\n")

		fmt.Fprint(&fp, "<g fill=\"black\" stroke-width=\"1\" stroke=\"black\">\n")
	}
	for _, dep := range dependencies {
		if dep.rel[variant] == "" {
			continue
		}
		i1, i2 := dep.end-1, dep.headpos-1
		if dep.headpos == 0 {
			i2 = i1
		}
		d1 := float64(items[i1].x2-items[i1].x1) - 20
		x1 := items[i1].x1 + 10 + int(d1*anchor(i1, i2, dep.lvl))
		y1 := MARGIN + EDGE_FONT_SIZE + EDGE_FONT_OFFSET + LVL_HEIGHT*(maxlvl+1)
		if isSVG {
			fmt.Fprintf(&fp,
				"<path d=\"M%d %d l3 -14 l-6 0 Z\" />\n",
				x1, y1)
		} else {
			surface.MoveTo(float64(x1), float64(y1))
			surface.RelLineTo(3, -14)
			surface.RelLineTo(-6, 0)
			surface.ClosePath()
			surface.StrokePreserve()
			surface.Fill()
		}
	}
	if isSVG {
		fmt.Fprint(&fp, "</g>\n")

		// dit is voor het geval er een lijn achter een label langs loopt
		// niet nodig voor basic UD?
		fmt.Fprintf(&fp, "<g fill=\"%s\" stroke=\"%s\" stroke-width=\"1\" opacity=\"%g\">\n",
			EDGE_LBL_BACKGROUND,
			EDGE_LBL_BACKGROUND,
			EDGE_LBL_OPACITY)
	} else {
		surface.SetSourceRGBA(1, 1, 1, EDGE_LBL_OPACITY)
	}
	for _, dep := range dependencies {
		if dep.rel[variant] == "" {
			continue
		}
		i1, i2 := dep.end-1, dep.headpos-1
		if dep.headpos == 0 {
			i2 = i1
		}
		d1 := float64(items[i1].x2-items[i1].x1) - 20
		x1 := items[i1].x1 + 10 + int(d1*anchor(i1, i2, dep.lvl))
		d2 := float64(items[i2].x2-items[i2].x1) - 20
		x2 := items[i2].x1 + 10 + int(d2*anchor(i2, i1, dep.lvl))
		y2 := MARGIN + EDGE_FONT_SIZE + EDGE_FONT_OFFSET + LVL_HEIGHT*(maxlvl-dep.lvl)
		w, h, l := textwidth(dep.rel[variant]+"i", EDGE_FONT_SIZE, true)
		if isSVG {
			fmt.Fprintf(&fp,
				"<rect x=\"%d\" y=\"%d\" width=\"%d\" height=\"%d\" />\n",
				(x1+x2-w)/2-EDGE_FONT_WHITE_MARGIN,
				y2-l-EDGE_FONT_OFFSET-EDGE_FONT_WHITE_MARGIN,
				w+2*EDGE_FONT_WHITE_MARGIN,
				h+2*EDGE_FONT_WHITE_MARGIN)
		} else {
			surface.Rectangle(
				float64((x1+x2-w)/2-EDGE_FONT_WHITE_MARGIN),
				float64(y2-l-EDGE_FONT_OFFSET-EDGE_FONT_WHITE_MARGIN),
				float64(w+2*EDGE_FONT_WHITE_MARGIN),
				float64(h+2*EDGE_FONT_WHITE_MARGIN))
			surface.Fill()
		}
	}
	if isSVG {
		fmt.Fprintln(&fp, "</g>")

		fmt.Fprintf(&fp, "<g font-family=\"%s\" font-size=\"%d\" text-anchor=\"middle\">\n", FONT, int(EDGE_FONT_SIZE))
	} else {
		surface.SetSourceRGBA(0, 0, 0, 1)
		surface.SelectFontFace("sans-serif", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
		surface.SetFontSize(EDGE_FONT_SIZE)
	}
	for _, dep := range dependencies {
		if dep.rel[variant] == "" {
			continue
		}
		i1, i2 := dep.end-1, dep.headpos-1
		if dep.headpos == 0 {
			i2 = i1
		}
		d1 := float64(items[i1].x2-items[i1].x1) - 20
		x1 := items[i1].x1 + 10 + int(d1*anchor(i1, i2, dep.lvl))
		d2 := float64(items[i2].x2-items[i2].x1) - 20
		x2 := items[i2].x1 + 10 + int(d2*anchor(i2, i1, dep.lvl))
		y2 := MARGIN + EDGE_FONT_SIZE + EDGE_FONT_OFFSET + LVL_HEIGHT*(maxlvl-dep.lvl)
		if isSVG {
			fmt.Fprintf(&fp,
				"<text x=\"%d\" y=\"%d\">%s</text>\n",
				(x1+x2)/2,
				y2-EDGE_FONT_OFFSET,
				html.EscapeString(dep.rel[variant]))
		} else {
			surface.MoveTo(float64((x1+x2)/2), float64(y2-EDGE_FONT_OFFSET))
			e := surface.TextExtents(dep.rel[variant])
			surface.RelMoveTo(e.Width*-0.5, 0)
			surface.ShowText(dep.rel[variant])
		}
	}
	if isSVG {
		fmt.Fprintln(&fp, "</g>")
	}

	// nodes

	offset := MARGIN + EDGE_FONT_SIZE + EDGE_FONT_OFFSET + LVL_HEIGHT*(maxlvl+1)

	if isSVG {
		fmt.Fprintln(&fp, "<g fill=\"#d0e0ff\" stroke=\"black\" stroke-width=\"1\">")
	}
	for _, item := range items {
		color := ""
		if item.enhanced {
			color = `fill="#ffe0e0" `
			// color = `stroke-dasharray="10,10" `
			if !isSVG {
				surface.SetSourceRGBA(1, float64(0xe0)/255.0, float64(0xe0)/255.0, 1)
			}
		} else {
			if !isSVG {
				surface.SetSourceRGBA(float64(0xd0)/255.0, float64(0xe0)/255.0, 1, 1)
			}
		}
		if isSVG {
			fmt.Fprintf(&fp, "<rect x=\"%d\" y=\"%d\" width=\"%d\" height=\"%d\" rx=\"5\" ry=\"5\" %s/>\n",
				item.x1,
				offset,
				item.x2-item.x1,
				int(NODE_HEIGHT),
				color)
		} else {
			roundedbox(surface,
				item.x1,
				offset,
				item.x2-item.x1,
				int(NODE_HEIGHT),
				5)
			surface.FillPreserve()
			surface.SetSourceRGBA(0, 0, 0, 1)
			surface.Stroke()
		}
	}
	if isSVG {
		fmt.Fprintln(&fp, "</g>")
	}

	_, _, y := textwidth("Xg", NODE_FONT_SIZE, false)
	lower := y / 2

	if isSVG {
		fmt.Fprintf(&fp, "<g font-family=\"%s\" font-size=\"%d\" text-anchor=\"middle\">\n", FONT, int(NODE_FONT_SIZE))
	} else {
		surface.SelectFontFace("sans-serif", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
		surface.SetFontSize(NODE_FONT_SIZE)
	}
	for _, item := range items {
		if isSVG {
			fmt.Fprintf(&fp, "<text x=\"%d\" y=\"%d\">%s</text>\n",
				(item.x1+item.x2)/2,
				offset+NODE_TWEEK+NODE_HEIGHT/4+lower,
				html.EscapeString(item.postag))
		} else {
			surface.MoveTo(float64((item.x1+item.x2)/2),
				float64(offset+NODE_TWEEK+NODE_HEIGHT/4+lower))
			e := surface.TextExtents(item.postag)
			surface.RelMoveTo(e.Width*-0.5, 0)
			surface.ShowText(item.postag)
		}
	}
	if isSVG {
		fmt.Fprintln(&fp, "</g>")

		fmt.Fprintf(&fp, "<g font-family=\"%s\" font-size=\"%d\" text-anchor=\"middle\" font-style=\"italic\">\n", FONT, int(NODE_FONT_SIZE))
	} else {
		surface.SelectFontFace("sans-serif", cairo.FONT_SLANT_ITALIC, cairo.FONT_WEIGHT_NORMAL)
		surface.SetFontSize(NODE_FONT_SIZE)
	}
	for _, item := range items {
		if isSVG {
			fmt.Fprintf(&fp, "<text x=\"%d\" y=\"%d\">%s</text>\n",
				(item.x1+item.x2)/2,
				offset-NODE_TWEEK+NODE_HEIGHT*3/4+lower,
				html.EscapeString(item.word))
		} else {
			surface.MoveTo(float64((item.x1+item.x2)/2),
				float64(offset-NODE_TWEEK+NODE_HEIGHT*3/4+lower))
			e := surface.TextExtents(item.word)
			surface.RelMoveTo(e.Width*-0.5, 0)
			surface.ShowText(item.word)
		}
	}
	if isSVG {
		fmt.Fprintln(&fp, "</g>")
	}

	if len(multis) > 0 {
		if isSVG {
			fmt.Fprintf(&fp, "<g fill=\"#D0E0FF\" stroke=\"black\" stroke-width=\"1\">\n")
		}
		for _, multi := range multis {
			aa := strings.Split(multi.id, "-")
			if len(aa) != 2 {
				x(fmt.Errorf("Line %d: Invalid range %s", multi.lineno, multi.id))
			}
			var x1, x2 int
			var found1, found2 bool
			for _, item := range items {
				if aa[0] == item.here {
					x1 = item.x1
					found1 = true
				}
				if aa[1] == item.here {
					x2 = item.x2
					found2 = true
					break
				}
			}
			if !(found1 && found2) {
				x(fmt.Errorf("Line %d: Invalid range %s", multi.lineno, multi.id))
			}
			if isSVG {
				fmt.Fprintf(&fp, "<rect x=\"%d\" y=\"%d\" width=\"%d\" height=\"%d\" rx=\"5\" ry=\"5\" />\n",
					x1,
					offset+NODE_HEIGHT+MULTI_SKIP,
					x2-x1,
					int(MULTI_HEIGHT))
			} else {
				roundedbox(surface, x1, offset+NODE_HEIGHT+MULTI_SKIP, x2-x1, MULTI_HEIGHT, 5)
				surface.SetSourceRGBA(float64(0xd0)/255.0, float64(0xe0)/255.0, 1, 1)
				surface.FillPreserve()
				surface.SetSourceRGBA(0, 0, 0, 1)
				surface.Stroke()
			}
		}
		if isSVG {
			fmt.Fprintln(&fp, "</g>")

			fmt.Fprintf(&fp, "<g font-family=\"%s\" font-size=\"%d\" font-style=\"italic\" text-anchor=\"middle\">\n", FONT, int(NODE_FONT_SIZE))
		} else {
			surface.SelectFontFace("sans-serif", cairo.FONT_SLANT_ITALIC, cairo.FONT_WEIGHT_NORMAL)
			surface.SetFontSize(NODE_FONT_SIZE)
		}
		for _, multi := range multis {
			aa := strings.Split(multi.id, "-")
			var x1, x2 int
			for _, item := range items {
				if aa[0] == item.here {
					x1 = item.x1
				}
				if aa[1] == item.here {
					x2 = item.x2
					break
				}
			}
			if isSVG {
				fmt.Fprintf(&fp, "<text x=\"%d\" y=\"%d\">%s</text>\n",
					(x1+x2)/2,
					offset+NODE_HEIGHT+MULTI_SKIP+MULTI_HEIGHT/2+lower,
					html.EscapeString(multi.word))
			} else {
				surface.MoveTo(float64((x1+x2)/2),
					float64(offset+NODE_HEIGHT+MULTI_SKIP+MULTI_HEIGHT/2+lower))
				e := surface.TextExtents(multi.word)
				surface.RelMoveTo(e.Width*-0.5, 0)
				surface.ShowText(multi.word)
			}
		}
		if isSVG {
			fmt.Fprintln(&fp, "</g>")
		}
	}

	if isSVG {
		fmt.Fprintln(&fp, "</svg>")

		return fp.String()
	}

	if format == "png" {
		surface.WriteToPNG(tempfile)
	}
	surface.Finish()
	b, err := ioutil.ReadFile(tempfile)
	x(err)
	return string(b)
}

func anchor(i1, i2, lvl int) float64 {
	a := anchors[i1]
	if len(a) == 1 {
		if i1 < i2 {
			return .75
		}
		return .25
	}
	n := 0
	for i, v := range a {
		if v.dist == i2-i1 && v.level == lvl {
			n = i
			break
		}
	}
	return (float64(n) + .5) / float64(len(a))
}

func classlbl(item *UdItem) string {
	n := item.end
	uses0 := make(map[int]bool)
	uses1 := make(map[int]bool)
	for _, dep := range dependencies {
		if dep.end == n || dep.headpos == n {
			if dep.rel[0] != "" {
				uses0[dep.end] = true
				uses0[dep.headpos] = true
			}
			if dep.rel[1] != "" {
				uses1[dep.end] = true
				uses1[dep.headpos] = true
			}
		}
	}
	lbls := make([]string, 0, len(uses0)+len(uses1))
	for use := range uses0 {
		lbls = append(lbls, fmt.Sprint("en", use))
	}
	for use := range uses1 {
		lbls = append(lbls, fmt.Sprint("ee", use))
	}
	return strings.Join(lbls, " ")
}

func tooltip(item *UdItem) string {
	return fmt.Sprintf("['%s','%s','%s','%s','%s']",
		html.EscapeString(item.word),
		html.EscapeString(item.postag),
		html.EscapeString(item.attribs),
		html.EscapeString(item.lemma),
		html.EscapeString(item.xpostag))
}

func textwidth(text string, fontsize float64, bold bool) (width, height, lift int) {
	var sizes []uint8
	var asc, desc int
	if bold {
		sizes = fontBoldSizes
		asc = fontBoldAscent
		desc = fontBoldDescent
	} else {
		sizes = fontRegularSizes
		asc = fontRegularAscent
		desc = fontRegularDescent
	}

	w := 0
	for _, c := range text {
		i := int(c)
		var w1 int
		if i >= len(sizes) {
			w1 = fontBaseSize
		} else {
			w1 = int(sizes[i])
		}
		w += w1
	}
	return int(fontsize * float64(w) / float64(fontBaseSize)),
		int(fontsize * float64(asc+desc) / float64(fontBaseSize)),
		int(fontsize * float64(asc) / float64(fontBaseSize))
}

func max(a int, b ...int) int {
	for _, i := range b {
		if i > a {
			a = i
		}
	}
	return a
}

func abs(i int) int {
	if i < 0 {
		return -i
	}
	return i
}

func roundedbox(surface *cairo.Surface, xi, yi, width, height, ri int) {
	x := float64(xi)
	y := float64(yi)
	w := float64(width)
	h := float64(height)
	r := float64(ri)
	surface.MoveTo(x+r, y)
	surface.LineTo(x+w-r, y)
	surface.Arc(x+w-r, y+r, r, math.Pi*1.5, 0)
	surface.LineTo(x+w, y+h-r)
	surface.Arc(x+w-r, y+h-r, r, 0, math.Pi*0.5)
	surface.LineTo(x+r, y+h)
	surface.Arc(x+r, y+h-r, r, math.Pi*0.5, math.Pi)
	surface.LineTo(x, y+r)
	surface.Arc(x+r, y+r, r, math.Pi, math.Pi*1.5)
	surface.ClosePath()
}
