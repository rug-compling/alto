package main

/*
#cgo LDFLAGS: -lgvc -lcgraph
#include <graphviz/gvc.h>
#include <graphviz/cgraph.h>
#include <stdlib.h>

typedef struct {
	char *s;
	int n;
} c_result;

c_result *makeGraph(char *data, char const *format) {
        Agraph_t *G;
        char *s;
        unsigned int n;
        GVC_t *gvc;
		c_result *result;

        s = NULL;
        gvc = gvContext();
        G = agmemread(data);
        free(data);
        if (G == NULL) {
                gvFreeContext(gvc);
                return NULL;
        }
        gvLayout(gvc, G, "dot");
        gvRenderData(gvc, G, format, &s, &n);
        gvFreeLayout(gvc, G);
        agclose(G);
        gvFreeContext(gvc);

		result = (c_result *) malloc(sizeof(c_result));
		if (result != NULL) {
			result->s = s;
			result->n = n;
		}

        return result;
}

void freeResult(c_result *result) {
	free(result->s);
	free(result);
}

*/
import "C"

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/rug-compling/alpinods"
)

type TreeContext struct {
	//      marks    map[string]bool
	refs  map[string]bool
	graph bytes.Buffer // definitie dot-bestand
	start int
	words []string
	// ud1      map[string]bool
	// ud2      map[string]bool
	SkipThis map[int]bool
	fp       io.Writer
}

type Node struct {
	node     *alpinods.Node
	index    int
	cat      string
	pt       string
	pos      string
	word     string
	nodelist []*alpinods.Node
}

func leeg(node *alpinods.Node) bool {
	return node.Word == "" && (node.Node == nil || len(node.Node) == 0)
}

func vizTree(chIn <-chan Item, chOut chan<- Item, subtree bool, format string) {
	cFormat := C.CString(format)
	for item := range chIn {
		var alpino alpinods.AlpinoDS
		x(xml.Unmarshal([]byte(item.data), &alpino))
		if subtree {
			// subtree van elke match
			// nodes met index uit de hele boom bewaren
			indexed := make(map[int]*alpinods.Node)
			var f1 func(*alpinods.Node)
			f1 = func(node *alpinods.Node) {
				node.Data = nil
				if node.Index > 0 && !leeg(node) {
					indexed[node.Index] = node
				}
				if node.Node != nil {
					for _, n := range node.Node {
						f1(n)
					}
				}
			}
			f1(alpino.Node)
			for i, match := range item.match {
				var subnode alpinods.Node
				x(xml.Unmarshal([]byte(match), &subnode))
				save := make([]Node, 0)
				count := make(map[int]int)
				seen := make(map[int]bool)
				var f2 func(*alpinods.Node)
				f2 = func(node *alpinods.Node) {
					if node.Index > 0 {
						// tel hoe vaak deze index voorkomt
						if _, ok := count[node.Index]; !ok {
							count[node.Index] = 0
						}
						count[node.Index]++
						if leeg(node) {
							// oude waarde van lege indexnode bewaren
							save = append(save, Node{
								node:     node,
								index:    node.Index,
								nodelist: []*alpinods.Node{},
							})
							// als deze index niet eerder gezien, dan lege node invullen
							if !seen[node.Index] {
								n := indexed[node.Index]
								node.Cat = n.Cat
								node.Pt = n.Pt
								node.Pos = n.Pos
								node.Word = n.Word
								node.Node = n.Node
							}
						} else {
							// oude waarde van niet-lege indexnode bewaren
							save = append(save, Node{
								node:     node,
								index:    node.Index,
								cat:      node.Cat,
								pt:       node.Pt,
								pos:      node.Pos,
								word:     node.Word,
								nodelist: node.Node,
							})
							if seen[node.Index] {
								// als we deze indexnode al eerder hebben gezien, dan leegmaken
								// zou niet voor moeten komen, maar voor alle zekerheid...
								node.Cat = ""
								node.Pt = ""
								node.Pos = ""
								node.Word = ""
								node.Node = []*alpinods.Node{}
							}
						}
						seen[node.Index] = true
					}
					if node.Node != nil {
						for _, n := range node.Node {
							f2(n)
						}
					}
				}
				var f3 func(node *alpinods.Node)
				f3 = func(node *alpinods.Node) {
					// als index maar 1 keer gebruikt wordt in subtree, dan verwijderen.
					if count[node.Index] == 1 {
						node.Index = 0
					}
					if node.Node != nil {
						for _, n := range node.Node {
							f3(n)
						}
					}
				}
				if len(indexed) > 0 {
					f2(&subnode)
					f3(&subnode)
				}
				chOut <- Item{
					name:  fmt.Sprintf("%s.%d.%s", item.oriname, i+1, format),
					data:  getTree(&subnode, alpino.Sentence.Sentence, cFormat, format == "dot"),
					match: make([]string, 0),
				}
				/*
					alles herstellen omdat op dit punt niet meer bekend is
					welke node uit de subtree komt (geen herstel nodig), en
					welke uit de complete boom (wel herstel nodig)
				*/
				for _, sn := range save {
					sn.node.Index = sn.index
					sn.node.Cat = sn.cat
					sn.node.Pos = sn.pos
					sn.node.Pt = sn.pt
					sn.node.Word = sn.word
					sn.node.Node = sn.nodelist

				}
			}
		} else {
			// de hele boom
			chOut <- Item{
				name:  fmt.Sprintf("%s.%s", item.oriname, format),
				data:  getTree(alpino.Node, alpino.Sentence.Sentence, cFormat, format == "dot"),
				match: make([]string, 0),
			}
		}
	}
	close(chOut)
}

func getTree(node *alpinods.Node, sentence string, cFormat *C.char, wantDot bool) string {
	ctx := &TreeContext{
		//              marks:    make(map[string]bool), // node met vette rand en edges van en naar de node, inclusief coindex
		refs:  make(map[string]bool),
		words: strings.Fields(sentence),
		// ud1:      make(map[string]bool),
		// ud2:      make(map[string]bool),
		SkipThis: make(map[int]bool),
	}

	ctx.graph.WriteString(`strict graph gr {

    ranksep=".25 equally"
    nodesep=.05
    ordering=out

    node [shape=plaintext, height=0, width=0, fontsize=12, fontname="Helvetica"];

`)

	// Nodes
	print_nodes(ctx, node)

	// Terminals
	ctx.graph.WriteString("\n    node [fontname=\"Helvetica-Oblique\", shape=box, color=\"#d3d3d3\", style=filled];\n\n")
	ctx.start = node.Begin
	terms := print_terms(ctx, node)
	sames := strings.Split(strings.Join(terms, " "), "|")
	for _, same := range sames {
		same = strings.TrimSpace(same)
		if same != "" {
			ctx.graph.WriteString("\n    {rank=same; " + same + " }\n")
		}
	}

	// Edges
	ctx.graph.WriteString("\n    edge [sametail=true, color=\"#d3d3d3\"];\n\n")
	print_edges(ctx, node)

	ctx.graph.WriteString("}\n")

	if wantDot {
		return ctx.graph.String()
	}

	result := C.makeGraph(C.CString(ctx.graph.String()), cFormat)
	if result == nil {
		x(fmt.Errorf("dot failed"))
	}
	output := C.GoStringN(result.s, result.n)
	C.freeResult(result)

	return output
}

func print_nodes(ctx *TreeContext, node *alpinods.Node) {
	idx := ""
	style := ""

	if node.Index > 0 {
		idx = fmt.Sprintf("\\n%v", node.Index)
		style += ", color=\"#d3d3d3\""
		style += ", shape=box"
	}

	if node.Data != nil {
		for _, d := range node.Data {
			if d.Name == "match" {
				style += ", color=\"#ffa07a\", style=filled"
			}
		}
	}

	lbl := dotquote(node.Rel) + idx
	// als dit geen lege index-node is, dan attributen toevoegen
	if !(node.Index > 0 && (node.Node == nil || len(node.Node) == 0) && node.Word == "") {
		if node.Cat != "" && node.Cat != node.Rel {
			lbl += "\\n" + dotquote(node.Cat)
		} else if node.Pt != "" && node.Pt != node.Rel {
			lbl += "\\n" + dotquote(node.Pt)
		}
	}

	ctx.graph.WriteString(fmt.Sprintf("    n%v [label=\"%v\"%s];\n", node.ID, lbl, style))
	for _, d := range node.Node {
		print_nodes(ctx, d)
	}
}

// Geeft een lijst terminals terug die op hetzelfde niveau moeten komen te staan,
// met "|" ingevoegd voor onderbrekingen in niveaus.
func print_terms(ctx *TreeContext, node *alpinods.Node) []string {
	terms := make([]string, 0)

	if node.Node == nil || len(node.Node) == 0 {
		if node.Word != "" {
			// Een terminal
			if node.Begin != ctx.start {
				// Onderbeking
				terms = append(terms, "|")
				// Onzichtbare node invoegen om te scheiden van node die links staat
				ctx.graph.WriteString(fmt.Sprintf("    e%v [label=\" \", style=invis];\n", node.ID))
				terms = append(terms, fmt.Sprintf("e%v", node.ID))
				ctx.SkipThis[node.ID] = true
			}
			ctx.start = node.End
			terms = append(terms, fmt.Sprintf("t%v", node.ID))
			ctx.graph.WriteString(fmt.Sprintf("    t%v [label=\"%s\"];\n", node.ID, dotquote(node.Word)))
			//} else {
			// Een lege node met index
		}
	} else {
		for _, d := range node.Node {
			t := print_terms(ctx, d)
			terms = append(terms, t...)
		}
	}
	return terms
}

func print_edges(ctx *TreeContext, node *alpinods.Node) {
	if node.Node == nil || len(node.Node) == 0 {
		if ctx.SkipThis[node.ID] {
			// Extra: Onzichtbare edge naar extra onzichtbare terminal
			ctx.graph.WriteString(fmt.Sprintf("    n%v -- e%v [style=invis];\n", node.ID, node.ID))
		}

		// geen edge voor lege indexen
		if node.Index == 0 || node.Word != "" {
			// Gewone edge naar terminal
			ctx.graph.WriteString(fmt.Sprintf("    n%v -- t%v;\n", node.ID, node.ID))
		}
	} else {
		// Edges naar dochters
		for _, d := range node.Node {
			// Gewone edge naar dochter
			ctx.graph.WriteString(fmt.Sprintf("    n%v -- n%v;\n", node.ID, d.ID))
		}
		for _, d := range node.Node {
			print_edges(ctx, d)
		}
	}
}

func dotquote(s string) string {
	s = strings.Replace(s, "\\", "\\\\", -1)
	s = strings.Replace(s, "\"", "\\\"", -1)
	return s
}

func dotquote2(s string) string {
	s = strings.Replace(s, "\\", "\\\\\\\\", -1)
	s = strings.Replace(s, "\"", "\\\"", -1)
	return s
}
