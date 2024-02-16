package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"github.com/rug-compling/alpinods"
)

type Fields struct {
	Corpusname     string
	Filename       string
	CorpusFilename string
	Body           string
	ID             int
	IDs            string
	Sentid         string
	Sentence       string
	MarkedSentence string
	Comments       string
	Match          string
	Tree           string
	Words          string
	Metadata       string
}

var (
	re   = regexp.MustCompile(`%[- +.#0-9]*[a-zA-Z%]`)
	reBS = regexp.MustCompile(`\\.`)
)

/*
  %%  %
  %c  corpusname
  %f  filename
  %F  if corpusname then corpusname::filename else filename
  %b  file body
  %i  *id
  %j  all ids
  %I  sentid
  %s  sentence
  %S  *sentence marked
  %o  comments
  %m  *match
  %M  *match tree
  %w  *match, words only
  %d  metadata
*/

func transformTemplate(chIn <-chan Item, chOut chan<- Item, tmpl string) {
	var needAlpino, needMeta, multi, needID, needIDs, needMatch, needMarked, needWords, needTree bool
	format := reBS.ReplaceAllStringFunc(tmpl, func(s string) string {
		if s == `\n` {
			return "\n"
		}
		if s == `\t` {
			return "\t"
		}
		if s == `\\` {
			return "\\"
		}
		return s
	})
	format = re.ReplaceAllStringFunc(format, func(s string) string {
		if s == "%%" {
			return "%"
		}
		toS := ` | printf "` + s[:len(s)-1] + `s"}}`
		toD := ` | printf "` + s[:len(s)-1] + `d"}}`
		switch s[len(s)-1] {
		case 'c':
			return "{{.Corpusname" + toS
		case 'f':
			return "{{.Filename" + toS
		case 'F':
			return "{{.CorpusFilename" + toS
		case 'b':
			return "{{.Body" + toS
		case 'i':
			needAlpino = true
			needID = true
			multi = true
			return "{{.ID" + toD
		case 'j':
			needAlpino = true
			needIDs = true
			return "{{.IDs}}"
		case 'I':
			needAlpino = true
			return "{{.Sentid" + toS
		case 's':
			needAlpino = true
			return "{{.Sentence" + toS
		case 'S':
			needAlpino = true
			needMarked = true
			multi = true
			return "{{.MarkedSentence" + toS
		case 'm':
			needMatch = true
			multi = true
			return "{{.Match" + toS
		case 'M':
			needAlpino = true
			needTree = true
			multi = true
			return "{{.Tree" + toS
		case 'o':
			needAlpino = true
			return "{{.Comments" + toS
		case 'w':
			needAlpino = true
			needWords = true
			multi = true
			return "{{.Words" + toS
		case 'd':
			needAlpino = true
			needMeta = true
			return "{{.Metadata" + toS
		default:
			x(fmt.Errorf("Unknown flag %q", s))
		}
		return ""
	})
	if !strings.HasSuffix(format, "\n") {
		format += "\n"
	}
	myTemplate, err := template.New("tmpl").Parse(format)
	x(err)

	for item := range chIn {

		if !item.transformed {
			item.transformed = true
			item.name += ".t"
		}

		var out bytes.Buffer

		var data Fields

		data.Corpusname = item.arch
		data.Filename = item.oriname
		if item.arch == "" {
			data.CorpusFilename = item.oriname
		} else {
			data.CorpusFilename = item.arch + "::" + item.oriname
		}
		data.Body = item.data
		var alpino alpinods.AlpinoDS
		if needAlpino {
			x(xml.Unmarshal([]byte(item.data), &alpino))
			data.Sentid = alpino.Sentence.SentID
			data.Sentence = alpino.Sentence.Sentence
			if alpino.Comments != nil && alpino.Comments.Comment != nil {
				data.Comments = strings.Join(alpino.Comments.Comment, "\n\t")
			}
			if needMeta && alpino.Metadata != nil && alpino.Metadata.Meta != nil {
				metas := make([]string, len(alpino.Metadata.Meta))
				for i, meta := range alpino.Metadata.Meta {
					metas[i] = fmt.Sprintf("%s: %q", meta.Name, meta.Value)
				}
				data.Metadata = strings.Join(metas, ", ")
			}
		}

		var i int
		for {

			if multi {
				var node alpinods.Node
				if needAlpino {
					x(xml.Unmarshal([]byte(item.match[i]), &node))
				}
				if needID {
					data.ID = node.ID
				}
				if needWords || needMarked {
					data.Words, data.MarkedSentence = doWords(&alpino, &node)
				}
				if needTree {
					data.Tree = doTree(&alpino, &node)
				}
				if needMatch {
					data.Match = item.match[i]
				}
			}
			if needIDs {
				idlist := make([]string, 0)
				for _, match := range item.match {
					var node alpinods.Node
					x(xml.Unmarshal([]byte(match), &node))
					idlist = append(idlist, fmt.Sprint(node.ID))
				}
				data.IDs = strings.Join(idlist, " ")
			}

			x(myTemplate.Execute(&out, data))

			i++
			if i == len(item.match) || !multi {
				break
			}

		}

		item.data = out.String()
		item.match = make([]string, 0)
		item.original = false
		chOut <- item

	} // for item
	close(chOut)
}

func doTree(alpino *alpinods.AlpinoDS, node *alpinods.Node) string {
	var out bytes.Buffer
	first := true
	nodelist := make([]*alpinods.Node, 1)
	nodelist[0] = node

	seen := make(map[int]bool)
	handled := make(map[int]bool)

	var f func(*alpinods.Node, string, bool)
	f = func(node *alpinods.Node, indent string, doRel bool) {
		if node == nil {
			return
		}
		p := indent
		if doRel {
			fmt.Fprint(&out, p, node.Rel)
			p = " "
		}
		if node.Index != 0 {
			seen[node.Index] = true
			if node.Word != "" || node.Node != nil && len(node.Node) > 0 {
				handled[node.Index] = true
			}
			fmt.Fprintf(&out, "%s[%d]", p, node.Index)
			p = " "
		}
		if node.Cat != "" {
			fmt.Fprint(&out, p, node.Cat)
		} else if node.Pt != "" {
			fmt.Fprintf(&out, "%s%s %q", p, node.Pt, node.Word)
		}
		fmt.Fprintln(&out)
		if node.Node != nil {
			indent += "    "
			for _, n := range node.Node {
				f(n, indent, true)
			}
		}
		for n := range seen {
			if !handled[n] {
				nodelist = append(nodelist, findNodeByIndex(alpino.Node, n))
				handled[n] = true
			}
		}
	}

	for len(nodelist) > 0 {
		current := nodelist[0]
		nodelist = nodelist[1:]
		f(current, "", first)
		first = false
	}

	return out.String()
}

func doWords(alpino *alpinods.AlpinoDS, node *alpinods.Node) (words string, sentence string) {
	nwords := alpino.Node.End
	wordslist := make([]string, nwords)
	use := make([]bool, nwords)

	var f func(*alpinods.Node)
	f = func(node *alpinods.Node) {
		if node.Word != "" {
			use[node.Begin] = true
		}
		if node.Node != nil {
			for _, n := range node.Node {
				f(n)
			}
		}
		if node.Index > 0 && node.Word == "" && (node.Node == nil || len(node.Node) == 0) {
			f(findNodeByIndex(alpino.Node, node.Index))
		}
	}
	f(node)

	first := nwords
	last := 0
	inUse := false
	swords := strings.Fields(alpino.Sentence.Sentence)
	for i, w := range swords {
		if use[i] {
			if i < first {
				first = i
			}
			if i > last {
				last = i
			}
			if !inUse {
				inUse = true
				wordslist[i] = "\x1B[7m" + w
			} else {
				wordslist[i] = w
			}
		} else {
			if inUse {
				inUse = false
				wordslist[i-1] += "\x1B[0m"
			}
			wordslist[i] = w
		}
	}
	if inUse {
		wordslist[nwords-1] += "\x1B[0m"
	}

	sentence = strings.Join(wordslist, " ")

	if last >= first {
		wlist := make([]string, 0, last-first+1)
		inUse = true
		for i := first; i <= last; i++ {
			if use[i] {
				inUse = true
				wlist = append(wlist, swords[i])
			} else {
				if inUse {
					wlist = append(wlist, "[...]")
					inUse = false
				}
			}
		}
		words = strings.Join(wlist, " ")
	}

	return words, sentence
}

func findNodeByIndex(node *alpinods.Node, index int) *alpinods.Node {
	if node.Index == index {
		if node.Word != "" {
			return node
		}
		if node.Node != nil && len(node.Node) > 0 {
			return node
		}
	}
	if node.Node != nil {
		for _, n := range node.Node {
			nn := findNodeByIndex(n, index)
			if nn != nil {
				return nn
			}
		}
	}
	return nil
}
