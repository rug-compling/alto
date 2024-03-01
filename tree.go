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

func vizTree(chIn <-chan Item, chOut chan<- Item, subtree bool, format string) {
	cFormat := C.CString(format)
	for item := range chIn {
		if subtree {
			var alpino alpinods.AlpinoDS
			x(xml.Unmarshal([]byte(item.data), &alpino))
			for i, match := range item.match {
				// TODO: lege nodes invullen als de definitie buiten de subboom staat
				chOut <- Item{
					name:  fmt.Sprintf("%s.%d.%s", item.oriname, i+1, format),
					data:  getTree(match, true, cFormat, format == "dot", alpino.Sentence.Sentence),
					match: make([]string, 0),
				}
			}
		} else {
			chOut <- Item{
				name:  fmt.Sprintf("%s.%s", item.oriname, format),
				data:  getTree(item.data, false, cFormat, format == "dot", ""),
				match: make([]string, 0),
			}
		}
	}
	close(chOut)
}

func getTree(data string, subtree bool, cFormat *C.char, wantDot bool, sentence string) string {
	var alpino alpinods.AlpinoDS
	var node alpinods.Node
	if subtree {
		x(xml.Unmarshal([]byte(data), &node))
	} else {
		x(xml.Unmarshal([]byte(data), &alpino))
		node = *alpino.Node
		sentence = alpino.Sentence.Sentence
	}

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
	print_nodes(ctx, &node)

	// Terminals
	ctx.graph.WriteString("\n    node [fontname=\"Helvetica-Oblique\", shape=box, color=\"#d3d3d3\", style=filled];\n\n")
	ctx.start = node.Begin
	terms := print_terms(ctx, &node)
	sames := strings.Split(strings.Join(terms, " "), "|")
	for _, same := range sames {
		same = strings.TrimSpace(same)
		if same != "" {
			ctx.graph.WriteString("\n    {rank=same; " + same + " }\n")
		}
	}

	// Edges
	ctx.graph.WriteString("\n    edge [sametail=true, color=\"#d3d3d3\"];\n\n")
	print_edges(ctx, &node)

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
			idx := ""
			col := ""
			if node.Begin != ctx.start {
				// Onderbeking
				terms = append(terms, "|")
				// Onzichtbare node invoegen om te scheiden van node die links staat
				ctx.graph.WriteString(fmt.Sprintf("    e%v [label=\" \", tooltip=\" \", style=invis];\n", node.ID))
				terms = append(terms, fmt.Sprintf("e%v", node.ID))
				ctx.SkipThis[node.ID] = true
			}
			ctx.start = node.End
			terms = append(terms, fmt.Sprintf("t%v", node.ID))
			if node.Lemma == "" {
				ctx.graph.WriteString(fmt.Sprintf("    t%v [label=\"%s%s\", tooltip=\"%s\"%s];\n",
					node.ID, idx, dotquote(node.Word), dotquote2(node.Postag), col))
			} else {
				ctx.graph.WriteString(fmt.Sprintf("    t%v [label=\"%s%s\", tooltip=\"%s:%s\"%s];\n",
					node.ID, idx, dotquote(node.Word), dotquote2(node.Lemma), dotquote(node.Postag), col))
			}
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
