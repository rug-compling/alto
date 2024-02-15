package main

// TODO: documentatie
// TODO: code opschonen, documenteren, opsplitsen over bestanden

/*
#cgo LDFLAGS: -lxqilla -lxerces-c
#include <stdlib.h>
#include "alto.h"
*/
import "C"

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	//"runtime"
	"strings"
	"unsafe"

	"github.com/jbowtie/gokogiri"
	"github.com/jbowtie/gokogiri/xpath"
	"github.com/pebbe/compactcorpus"
	"github.com/pebbe/dbxml"
	"github.com/pebbe/util"
	"github.com/rug-compling/alpinods"
	"github.com/rug-compling/alud/v2"
	"github.com/wamuir/go-xslt"
)

type Item struct {
	arch        string
	name        string
	oriname     string
	data        string
	match       []string
	skipfilter  bool // als het eerste XPath-filter al is toegepast bij inlezen vanuit DACT, dan kan het eerste filter alles doorlaten
	original    bool // als dit een origineel XML-bestand is, dan hoeft er geen tijdelijk bestand gemaakt te worden voor XQilla
	transformed bool
}

const (
	DEVIDER = "((<<<))"
)

var (
	chDone      = make(chan bool)
	compactSeen = make(map[string]bool)
	tempdir     = os.TempDir()
	cDEVIDER    = C.CString(DEVIDER)
	cEMPTY      = C.CString("")
	cFILENAME   = C.CString("filename")

	verbose       = true
	macrofile     = ""
	showExpansion = false
	replace       = false
	markMatch     = false
	readStdin     = false
	version1      = false
	version2xpath = false
	version2xslt  = false
	variables     = []*C.char{
		C.CString("filename"),
		cEMPTY,
		C.CString("corpusname"),
		cEMPTY,
	}
	xsltVariables = make([]xslt.Parameter, 0)
	macros        = make(map[string]string)
	macroRE       = regexp.MustCompile(`([a-zA-Z][_a-zA-Z0-9]*)\s*=\s*"""((?s:.*?))"""`)
	macroKY       = regexp.MustCompile(`%[a-zA-Z][_a-zA-Z0-9]*%`)
	macroCOM      = regexp.MustCompile(`(?m:^\s*#.*)`)
	x             = util.CheckErr
)

func usage() {
	major, minor, patch := dbxml.Version()
	fmt.Printf(
		`
Usage: %s (option | action | filename) ...

Options:

    -e              : show macro-expansion
    -i              : read input filenames from stdin
    -m filename     : use this macrofile for xpath
                      (or use environment variable ALTO_MACROFILE)
    -n              : mark matching node
    -o filename     : output
    -r              : replace xml in existing dact file
    -v name=value   : set global variable (can be used multiple times)
    -1              : use XPath version 1 for searching in DACT files
    -2              : use XPath2 and XSLT2 (slow)
    -2p             : use XPath2 (slow)
    -2s             : use XSLT2 (slow)

Actions:

    ds:ud           : insert Universal Dependencies
    ds:noud         : remove Universal Dependencies
    ds:extra        : add extra attributes: is_np, is_vorfeld, is_nachfeld
    ds:minimal      : removes all but essential entities and attributes

    fp:{expression} : filter by XPath {expression}

    tq:{xqueryfile} : transform with XQuery {xqueryfile}
    ts:{stylefile}  : transform with XSLT {stylefile}
    tt:{template}   : transform with {template}

    Tq:{xqueryfile} : like tq, match data as input
    Ts:{stylefile}  : like ts, match data as input

    ac:item         : item count
    ac:line         : line count
    ac:node         : count of cat, pos, postag, rel
    ac:word         : count of lemma, root, sense, word
    ac:nw           : combination of ac:node and ac:word

Template placeholders:

    %%%%  %%
    %%c  corpusname
    %%f  filename
    %%F  if corpusname then corpusname::filename else filename
    %%b  file body
    %%i  id of matching node
    %%j  ids of all matching nodes
    %%I  sentence id
    %%s  sentence
    %%S  colored sentence
    %%o  comments
    %%m  match
    %%M  match as tree
    %%w  match words
    %%d  metadata
    \t  tab
    \n  newline

Input filenames can be given as arguments, or/and
one name per line on stdin, using option -i

Examples:
    %s *.xml -o output.zip
    %s -o output.dact input.zip
    find . -name '*.xml' | %s -o output.zip -i

Valid input filenames:
    *.xml
    *.dact (or *.dbxml)
    *.data.dz (or *.index)
    *.zip
    directory name

    an input filename can also be in the format corpusfile::xmlfile

Valid output filenames:
    *.dact (or *.dbxml)
    *.data.dz (or *.index)
    *.zip
    *.txt
    directory name

Default output is stdout

%s uses DbXML version %d.%d.%d

`,
		os.Args[0],
		os.Args[0],
		os.Args[0],
		os.Args[0],
		os.Args[0],
		major, minor, patch)
}

func main() {
	if len(os.Args) == 1 && util.IsTerminal(os.Stdin) {
		usage()
		return
	}

	macrofile = os.Getenv("ALTO_MACROFILE")
	outfile := ""
	inputfiles := make([]string, 0)
	actions := make([]string, 0)

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if strings.HasPrefix(arg, "-") {
			switch arg {
			case "-e":
				showExpansion = true
			case "-h":
				usage()
				return
			case "-i":
				readStdin = true
			case "-m":
				i++
				if i == len(os.Args) {
					fmt.Fprintln(os.Stderr, "Missing filename for option -m")
					return
				}
				macrofile = os.Args[i]
			case "-n":
				markMatch = true
			case "-o":
				i++
				if i == len(os.Args) {
					fmt.Fprintln(os.Stderr, "Missing filename for option -o")
					return
				}
				outfile = os.Args[i]
			case "-r":
				replace = true
			case "-v":
				i++
				if i == len(os.Args) {
					fmt.Fprintln(os.Stderr, "Missing variable for option -v")
					return
				}
				a := strings.SplitN(os.Args[i], "=", 2)
				if len(a) != 2 || a[0] == "" /* || a[1] == "" */ {
					fmt.Fprintln(os.Stderr, "Invalid name=value for option -v:", os.Args[i])
					return
				}
				variables = append(variables, C.CString(a[0]), C.CString(a[1]))
				xsltVariables = append(xsltVariables, xslt.Parameter(xslt.StringParameter{Name: a[0], Value: a[1]}))
			case "-1":
				version1 = true
			case "-2":
				version2xpath = true
				version2xslt = true
			case "-2p":
				version2xpath = true
			case "-2x":
				version2xslt = true
			default:
				fmt.Fprintln(os.Stderr, "Unknown option", arg)
				return
			}
		} else if len(arg) > 2 && arg[2] == ':' {
			actions = append(actions, arg)
		} else {
			inputfiles = append(inputfiles, arg)
		}
	}

	if replace {
		if !(strings.HasSuffix(outfile, ".dact") || strings.HasSuffix(outfile, ".dbxml")) {
			fmt.Fprintln(os.Stderr, "Option -r only valid for output to dact")
			return
		}
	}

	if readStdin {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			inputfiles = append(inputfiles, scanner.Text())
		}
		x(scanner.Err())
	}

	if len(inputfiles) == 0 {
		fmt.Fprintln(os.Stderr, "Missing input file(s)")
		return
	}

	firstFilter := ""

	chStart := make(chan Item, 100)
	chIn := chStart

	for i, action := range actions {
		act := action[:2]
		arg := action[3:]
		if action == "ds:ud" {
			chOut := make(chan Item, 100)
			go doUD(chIn, chOut)
			chIn = chOut
		} else if action == "ds:noud" {
			chOut := make(chan Item, 100)
			go undoUD(chIn, chOut)
			chIn = chOut
		} else if action == "ds:extra" {
			chOut := make(chan Item, 100)
			go doExtra(chIn, chOut)
			chIn = chOut
		} else if action == "ds:minimal" {
			chOut := make(chan Item, 100)
			go doMinimal(chIn, chOut)
			chIn = chOut
		} else if act == "fp" {
			arg = expandMacros(arg)
			if i == 0 && !version1 {
				firstFilter = arg
			}
			chOut := make(chan Item, 100)
			if version2xpath {
				go filterXpath2(chIn, chOut, arg)
			} else {
				go filterXpath(chIn, chOut, arg)
			}
			chIn = chOut
		} else if act == "tq" || act == "ts" || act == "Tq" || act == "Ts" {
			var lang C.Language
			switch act {
			case "tq", "Tq":
				lang = C.langXQUERY
			case "ts", "Ts":
				lang = C.langXSLT
			}
			b, err := os.ReadFile(arg)
			x(err)
			style := expandMacros(string(b))
			chOut := make(chan Item, 100)
			if act[1] == 's' && !version2xslt {
				go transformLibXSLT(chIn, chOut, act[0] == 'T', style)
			} else {
				go transformStylesheet(chIn, chOut, lang, act[0] == 'T', style)
			}
			chIn = chOut
		} else if act == "tt" {
			chOut := make(chan Item, 100)
			go transformTemplate(chIn, chOut, arg)
			chIn = chOut
		} else if act == "ac" {
			chOut := make(chan Item, 100)
			if arg == "item" || arg == "line" {
				go aggregateCount(chIn, chOut, arg == "line")
			} else if arg == "node" || arg == "word" || arg == "nw" {
				go aggregateAttribs(chIn, chOut, arg == "node" || arg == "nw", arg == "word" || arg == "nw")
			} else {
				fmt.Fprintf(os.Stderr, "Unknown action %q\n", action)
				return
			}
			chIn = chOut
		} else {
			fmt.Fprintf(os.Stderr, "Unknown action %q\n", action)
			return
		}
	}

	if strings.HasSuffix(outfile, ".data.dz") || strings.HasSuffix(outfile, ".index") {
		go writeCompact(chIn, outfile)
	} else if strings.HasSuffix(outfile, ".dbxml") || strings.HasSuffix(outfile, ".dact") {
		go writeDact(chIn, outfile)
	} else if strings.HasSuffix(outfile, ".zip") {
		go writeZip(chIn, outfile)
	} else if strings.HasSuffix(outfile, ".txt") {
		go writeTxt(chIn, outfile)
	} else if outfile == "" {
		verbose = false
		go writeStdout(chIn)
	} else {
		go writeDir(chIn, outfile)
	}

	n := len(inputfiles)
	for i, infile := range inputfiles {
		var xmlfiles []string
		a := strings.Split(infile, "::")
		if len(a) > 1 {
			infile = a[0]
			xmlfiles = a[1:]
		}
		infile = filepath.Clean(infile)
		if strings.HasSuffix(infile, ".data.dz") || strings.HasSuffix(infile, ".index") {
			readCompact(chStart, infile, i+1, n, xmlfiles)
		} else if strings.HasSuffix(infile, ".dbxml") || strings.HasSuffix(infile, ".dact") {
			readDact(chStart, infile, i+1, n, firstFilter, xmlfiles)
		} else if strings.HasSuffix(infile, ".zip") {
			readZip(chStart, infile, i+1, n, xmlfiles)
		} else if xmlfiles != nil {
			fmt.Fprintf(os.Stderr, "Invalid corpus/file combination: %s::%s\n", infile, strings.Join(xmlfiles, "::"))
			return
		} else if strings.HasSuffix(infile, ".xml") {
			readXml(chStart, infile, i+1, n)
		} else {
			readDir(chStart, infile, "", i+1, n, firstFilter)
		}
	}

	close(chStart)

	<-chDone
	if verbose {
		fmt.Fprintln(os.Stderr)
	}
}

func aggregateAttribs(chIn <-chan Item, chOut chan<- Item, doNode, doWord bool) {
	var cat, pos, postag, rel int
	var lemma, root, sense, word int
	cats := make(map[string]int)
	poss := make(map[string]int)
	postags := make(map[string]int)
	rels := make(map[string]int)
	lemmas := make(map[string]int)
	roots := make(map[string]int)
	senses := make(map[string]int)
	words := make(map[string]int)
	for item := range chIn {
		for _, match := range item.match {
			var node alpinods.Node
			x(xml.Unmarshal([]byte(match), &node))
			if doNode {
				if node.Cat != "" {
					if _, ok := cats[node.Cat]; !ok {
						cats[node.Cat] = 0
					}
					cats[node.Cat] += 1
					cat++
				}
				if node.Pos != "" {
					if _, ok := poss[node.Pos]; !ok {
						poss[node.Pos] = 0
					}
					poss[node.Pos] += 1
					pos++
				}
				if node.Postag != "" {
					if _, ok := postags[node.Postag]; !ok {
						postags[node.Postag] = 0
					}
					postags[node.Postag] += 1
					postag++
				}
				if node.Rel != "" {
					if _, ok := rels[node.Rel]; !ok {
						rels[node.Rel] = 0
					}
					rels[node.Rel] += 1
					rel++
				}
			}
			if doWord {
				if node.Lemma != "" {
					if _, ok := lemmas[node.Lemma]; !ok {
						lemmas[node.Lemma] = 0
					}
					lemmas[node.Lemma] += 1
					lemma++
				}
				if node.Root != "" {
					if _, ok := roots[node.Root]; !ok {
						roots[node.Root] = 0
					}
					roots[node.Root] += 1
					root++
				}
				if node.Sense != "" {
					if _, ok := senses[node.Sense]; !ok {
						senses[node.Sense] = 0
					}
					senses[node.Sense] += 1
					sense++
				}
				if node.Word != "" {
					if _, ok := words[node.Word]; !ok {
						words[node.Word] = 0
					}
					words[node.Word] += 1
					word++
				}
			}
		}
	}
	var buf bytes.Buffer
	f := func(label string, sum int, count map[string]int) {
		fmt.Fprintf(&buf, "%s:\n", label)
		keys := make([]string, 0, len(count))
		for key := range count {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		fsum := float64(sum)
		for _, key := range keys {
			fmt.Fprintf(&buf, "%8d  %8.4f  %s\n", count[key], float64(count[key])/fsum, key)
		}
	}
	if doNode {
		f("cat", cat, cats)
		f("pos", pos, poss)
		f("postag", postag, postags)
		f("rel", rel, rels)
	}
	if doWord {
		f("lemma", lemma, lemmas)
		f("root", root, roots)
		f("sense", sense, senses)
		f("word", word, words)
	}
	chOut <- Item{
		name:  "result",
		data:  buf.String(),
		match: make([]string, 0),
	}

	close(chOut)
}

func aggregateCount(chIn <-chan Item, chOut chan<- Item, byline bool) {
	var sum int
	count := make(map[string]int)
	for item := range chIn {
		for _, m := range item.match {
			m = strings.TrimSpace(m)
			if byline {
				for _, ml := range strings.Split(m, "\n") {
					ml = strings.TrimSpace(ml)
					if _, ok := count[ml]; !ok {
						count[ml] = 0
					}
					count[ml]++
					sum++
				}
			} else {
				if _, ok := count[m]; !ok {
					count[m] = 0
				}
				count[m]++
				sum++
			}
		}
	}
	keys := make([]string, 0, len(count))
	for key := range count {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, len(keys))
	fsum := float64(sum)
	for i, key := range keys {
		lines[i] = fmt.Sprintf("%8d  %8.4f  %s", count[key], float64(count[key])/fsum, key)
	}

	chOut <- Item{
		name:  "result",
		data:  strings.Join(lines, "\n"),
		match: make([]string, 0),
	}

	close(chOut)
}

func doUD(chIn <-chan Item, chOut chan<- Item) {
	for item := range chIn {
		s, err := alud.UdAlpino([]byte(item.data), item.oriname, "")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error %s: %v\n", item.name, err)
		}
		if s != "" {
			item.data = s
		}
		item.original = false
		chOut <- item
	}
	close(chOut)
}

func undoUD(chIn <-chan Item, chOut chan<- Item) {
	var f func(*alpinods.Node)
	f = func(node *alpinods.Node) {
		node.Ud = nil
		if node.Node != nil {
			for _, n := range node.Node {
				f(n)
			}
		}
	}

	for item := range chIn {
		var alpino alpinods.AlpinoDS
		x(xml.Unmarshal([]byte(item.data), &alpino))
		f(alpino.Node)
		alpino.Root = nil
		alpino.Conllu = nil
		item.data = alpino.String()
		item.original = false
		chOut <- item
	}
	close(chOut)
}

func doExtra(chIn <-chan Item, chOut chan<- Item) {
	for item := range chIn {
		var alpino alpinods.AlpinoDS
		x(xml.Unmarshal([]byte(item.data), &alpino))
		alpino.Enhance(alpinods.Fall)
		item.data = alpino.String()
		item.original = false
		chOut <- item
	}
	close(chOut)
}

func doMinimal(chIn <-chan Item, chOut chan<- Item) {
	for item := range chIn {
		var alpino alpinods.AlpinoDS
		x(xml.Unmarshal([]byte(item.data), &alpino))
		item.data = alpinods.Reduce(&alpino).String()
		item.original = false
		chOut <- item
	}
	close(chOut)
}

func filterXpath(chIn <-chan Item, chOut chan<- Item, xp string) {
	var expr *xpath.Expression

	for item := range chIn {
		if item.skipfilter {
			// eerste filter is toegepast bij lezen vanuit dbxml-bestand
			item.skipfilter = false
			chOut <- item
			continue
		}

		if expr == nil {
			expr = xpath.Compile(xp)
			if expr == nil {
				os.Exit(1)
			}
		}

		doc, err := gokogiri.ParseXml([]byte(item.data))
		x(err)
		root := doc.Root()
		matches, err := root.Search(expr)
		x(err)
		if len(matches) > 0 {
			item.match = item.match[0:0]
			for _, match := range matches {
				item.match = append(item.match, match.String())
			}
			if markMatch {
				var alpino alpinods.AlpinoDS
				x(xml.Unmarshal([]byte(item.data), &alpino))
				markMatchingNode(alpino.Node, item.match...)
				item.data = alpino.String()
				item.original = false
			}
			chOut <- item
		}
		doc.Free()
	}
	close(chOut)
}

func filterXpath2(chIn <-chan Item, chOut chan<- Item, xpath string) {
	// runtime.LockOSThread()

	cxpath := C.CString(xpath)
	vars := make([]*C.char, 1)

	for item := range chIn {
		if item.skipfilter {
			// eerste filter is toegepast bij lezen vanuit dbxml-bestand
			item.skipfilter = false
			chOut <- item
			continue
		}

		var cs *C.char
		var filename string
		if item.original {
			cs = C.CString(item.oriname)
		} else {
			fp, err := os.CreateTemp(tempdir, "mkcFXP")
			x(err)
			_, err = fp.WriteString(item.data)
			x(err)
			filename = fp.Name()
			x(fp.Close())
			cs = C.CString(filename)
		}

		result := C.xq_call(cs, cxpath, C.langXPATH, cDEVIDER, 0, &(vars[0]))

		C.free(unsafe.Pointer(cs))
		if !item.original {
			os.Remove(filename)
		}

		if C.xq_error(result) == 0 {
			item.match = make([]string, 0)
			for _, m := range strings.Split(C.GoString(C.xq_text(result)), DEVIDER) {
				if len(m) > 0 {
					item.match = append(item.match, m)
				}
			}
			if len(item.match) > 0 {
				if markMatch {
					var alpino alpinods.AlpinoDS
					x(xml.Unmarshal([]byte(item.data), &alpino))
					markMatchingNode(alpino.Node, item.match...)
					item.data = alpino.String()
					item.original = false
				}
				chOut <- item
			}
		}
		C.xq_free(result)
	}
	close(chOut)
}

func markMatchingNode(node *alpinods.Node, matches ...string) {
	var f func(*alpinods.Node, int) bool
	f = func(n *alpinods.Node, id int) bool {
		if n.ID == id {
			if n.Data == nil {
				n.Data = make([]*alpinods.Data, 0)
			}
			n.Data = append(n.Data, &alpinods.Data{
				Name: "match",
			})
			return true
		}
		if n.Node != nil {
			for _, n2 := range n.Node {
				if f(n2, id) {
					return true
				}
			}
		}
		return false
	}

	for _, match := range matches {
		var matchNode alpinods.Node
		if xml.Unmarshal([]byte(match), &matchNode) == nil {
			f(node, matchNode.ID)
		}
	}
}

func transformLibXSLT(chIn <-chan Item, chOut chan<- Item, useMatch bool, style string) {
	xs, err := xslt.NewStylesheet([]byte(style))
	x(err)

	for item := range chIn {
		matchdata := item.match
		for i := 0; ; i++ {
			if useMatch {
				if i >= len(matchdata) {
					break
				}
			} else if i > 0 {
				break
			}

			if !item.transformed {
				item.transformed = true
				item.name += ".t"
			}

			params := []xslt.Parameter{
				xslt.Parameter(xslt.StringParameter{Name: "filename", Value: item.oriname}),
				xslt.Parameter(xslt.StringParameter{Name: "corpusname", Value: item.arch}),
			}
			params = append(params, xsltVariables...)

			var result []byte
			if useMatch {
				result, err = xs.Transform([]byte(matchdata[i]), params...)
			} else {
				result, err = xs.Transform([]byte(item.data), params...)
			}
			x(err)

			item.data = string(result)
			item.match = []string{item.data}
			if len(item.data) > 0 {
				chOut <- item
			}
		}
	}
	close(chOut)
}

func transformStylesheet(chIn <-chan Item, chOut chan<- Item, lang C.Language, useMatch bool, style string) {
	// runtime.LockOSThread()

	cstyle := C.CString(style)

	for item := range chIn {
		matchdata := item.match
		for i := 0; ; i++ {
			if useMatch {
				if i >= len(matchdata) {
					break
				}
			} else if i > 0 {
				break
			}

			variables[1] = C.CString(item.oriname)
			variables[3] = C.CString(item.arch)

			if !item.transformed {
				item.transformed = true
				item.name += ".t"
			}

			var cs *C.char
			var filename string
			if item.original && !useMatch {
				cs = C.CString(item.oriname)
			} else {
				fp, err := os.CreateTemp(tempdir, "mkcTST")
				x(err)
				if useMatch {
					_, err = fp.WriteString(matchdata[i])
				} else {
					_, err = fp.WriteString(item.data)
				}
				x(err)
				filename = fp.Name()
				x(fp.Close())
				cs = C.CString(filename)
			}

			result := C.xq_call(cs, cstyle, lang, cEMPTY, C.int(len(variables)/2), &(variables[0]))

			C.free(unsafe.Pointer(cs))
			C.free(unsafe.Pointer(variables[1]))
			C.free(unsafe.Pointer(variables[3]))
			if useMatch {
				item.original = false
				os.Remove(filename)
			} else {
				if item.original {
					item.original = false
				} else {
					os.Remove(filename)
				}
			}

			if C.xq_error(result) == 0 {
				item.data = C.GoString(C.xq_text(result))
				item.match = []string{item.data}
				if len(item.data) > 0 {
					chOut <- item
				}
			}
			C.xq_free(result)
		}
	}
	close(chOut)
}

func writeCompact(chIn <-chan Item, outfile string) {
	seen := make(map[string]bool)
	outfile = strings.TrimSuffix(outfile, ".data.gz")
	outfile = strings.TrimSuffix(outfile, ".index")
	mustNotExist(outfile + ".data.dz")
	mustNotExist(outfile + ".index")
	corpus, err := compactcorpus.NewCorpus(outfile)
	x(err)
	for item := range chIn {
		name := filepath.Base(item.name)
		if seen[name] {
			x(fmt.Errorf("Duplicate filename: %s", name))
		}
		seen[name] = true
		x(corpus.WriteString(name, item.data))
	}
	x(corpus.Close())
	close(chDone)
}

func writeDact(chIn <-chan Item, outfile string) {
	// runtime.LockOSThread()

	if !replace {
		mustNotExist(outfile)
	}
	db, err := dbxml.OpenReadWrite(outfile)
	x(err)
	for item := range chIn {
		x(db.PutXml(item.name, item.data, replace))
	}
	db.Close()
	close(chDone)
}

func writeZip(chIn <-chan Item, outfile string) {
	mustNotExist(outfile)
	fp, err := os.Create(outfile)
	x(err)
	w := zip.NewWriter(fp)
	for item := range chIn {
		f, err := w.Create(item.name)
		x(err)
		_, err = f.Write([]byte(item.data))
		x(err)
	}
	x(w.Close())
	x(fp.Close())
	close(chDone)
}

func writeTxt(chIn <-chan Item, outfile string) {
	mustNotExist(outfile)
	fp, err := os.Create(outfile)
	x(err)
	for item := range chIn {
		_, err := fp.WriteString(item.data)
		x(err)
		if !strings.HasSuffix(item.data, "\n") {
			_, err := fp.WriteString("\n")
			x(err)
		}
	}
	x(fp.Close())
	close(chDone)
}

func writeStdout(chIn <-chan Item) {
	for item := range chIn {
		_, err := os.Stdout.WriteString(item.data)
		x(err)
		if !strings.HasSuffix(item.data, "\n") {
			os.Stdout.WriteString("\n")
		}
	}
	close(chDone)
}

func writeDir(chIn <-chan Item, outdir string) {
	entries, err := os.ReadDir(outdir)
	x(err)
	if len(entries) > 0 {
		x(fmt.Errorf("Directory %q is not empty", outdir))
	}
	for item := range chIn {
		outfile := filepath.Join(outdir, item.name)
		x(os.MkdirAll(filepath.Dir(outfile), 0777))
		if _, err := os.Stat(outfile); err == nil {
			x(fmt.Errorf("File exists: %s", outfile))
		}
		fp, err := os.Create(outfile)
		x(err)
		_, err = fp.WriteString(item.data)
		x(err)
		x(fp.Close())
	}
	close(chDone)
}

func readCompact(chOut chan<- Item, infile string, i, n int, xmlfiles []string) {
	infile = strings.TrimSuffix(infile, ".data.dz")
	infile = strings.TrimSuffix(infile, ".index")
	if compactSeen[infile] {
		return
	}
	compactSeen[infile] = true

	if xmlfiles != nil {
		corpus, err := compactcorpus.RaOpen(infile)
		x(err)
		for _, xmlfile := range xmlfiles {
			if xmlfile != "" {
				data, err := corpus.Get(xmlfile)
				x(err)
				chOut <- Item{
					arch:    infile + ".data.dz",
					name:    xmlfile,
					oriname: xmlfile,
					data:    string(data),
					match:   make([]string, 0),
				}
			}
		}
		corpus.Close()
		return
	}

	corpus, err := compactcorpus.Open(infile)
	x(err)
	r, err := corpus.NewRange()
	x(err)
	j := 0
	for r.HasNext() {
		j++
		name, data := r.Next()
		if verbose {
			fmt.Fprintf(os.Stderr, " %d/%d %s -- %d/? %s        \r", i, n, infile, j, name)
		}
		chOut <- Item{
			arch:    infile + ".data.dz",
			name:    name,
			oriname: name,
			data:    string(data),
			match:   make([]string, 0),
		}
	}
}

func readDact(chOut chan<- Item, infile string, i, n int, filter string, xmlfiles []string) {
	// runtime.LockOSThread()

	db, err := dbxml.OpenRead(infile)
	x(err)
	defer db.Close()

	if xmlfiles != nil {
		for _, xmlfile := range xmlfiles {
			if xmlfile != "" {
				data, err := db.Get(xmlfile)
				x(err)
				chOut <- Item{
					arch:    infile,
					name:    xmlfile,
					oriname: xmlfile,
					data:    data,
					match:   make([]string, 0),
				}
			}
		}
		return
	}

	if filter == "" {
		size, err := db.Size()
		x(err)
		docs, err := db.All()
		x(err)
		j := 0
		for docs.Next() {
			j++
			name := docs.Name()
			if verbose {
				fmt.Fprintf(os.Stderr, " %d/%d %s -- %d/%d %s        \r", i, n, infile, j, size, name)
			}
			chOut <- Item{
				arch:    infile,
				name:    name,
				oriname: name,
				data:    docs.Content(),
				match:   make([]string, 0),
			}
		}
	} else {
		docs, err := db.Query(filter)
		x(err)
		name := ""
		content := ""
		match := make([]string, 0)
		j := 0
		for docs.Next() {
			// TODO
			// Hier ga ik ervan uit dat als er meerdere matches per xml-bestand zijn
			// dat die dan allemaal achter elkaar zitten.
			// Is dit zo?
			newname := docs.Name()
			if name != newname {
				j++
				if content != "" {
					if markMatch {
						var alpino alpinods.AlpinoDS
						x(xml.Unmarshal([]byte(content), &alpino))
						markMatchingNode(alpino.Node, match...)
						content = alpino.String()
					}
					chOut <- Item{
						arch:       infile,
						name:       name,
						oriname:    name,
						data:       content,
						match:      match,
						skipfilter: true,
					}
				}
				name = newname
				content = docs.Content()
				match = make([]string, 0)
			}
			match = append(match, docs.Match())
			if verbose {
				fmt.Fprintf(os.Stderr, " %d/%d %s -- %d/? %s        \r", i, n, infile, j, name)
			}
		}
		if content != "" {
			if markMatch {
				var alpino alpinods.AlpinoDS
				x(xml.Unmarshal([]byte(content), &alpino))
				markMatchingNode(alpino.Node, match...)
				content = alpino.String()
			}
			chOut <- Item{
				arch:       infile,
				name:       name,
				oriname:    name,
				data:       content,
				match:      match,
				skipfilter: true,
			}
		}
	}
}

func readZip(chOut chan<- Item, infile string, i, n int, xmlfiles []string) {
	zr, err := zip.OpenReader(infile)
	x(err)

	if xmlfiles != nil {
		for _, xmlfile := range xmlfiles {
			if xmlfile != "" {
				fp, err := zr.Open(xmlfile)
				x(err)
				data, err := io.ReadAll(fp)
				x(err)
				fp.Close()
				chOut <- Item{
					arch:    infile,
					name:    xmlfile,
					oriname: xmlfile,
					data:    string(data),
					match:   make([]string, 0),
				}
			}
		}
		return
	}

	for j, file := range zr.File {
		if file.FileInfo().IsDir() {
			continue
		}
		name := file.Name
		if verbose {
			fmt.Fprintf(os.Stderr, " %d/%d %s -- %d/? %s        \r", i, n, infile, j+1, name)
		}
		f, err := file.Open()
		x(err)
		data, err := io.ReadAll(f)
		x(err)
		chOut <- Item{
			arch:    infile,
			name:    name,
			oriname: name,
			data:    string(data),
			match:   make([]string, 0),
		}
	}
}

func readXml(chOut chan<- Item, infile string, i, n int) {
	if verbose {
		fmt.Fprintf(os.Stderr, " %d/%d %s        \r", i, n, infile)
	}
	data, err := os.ReadFile(infile)
	x(err)
	chOut <- Item{
		name:     infile,
		oriname:  infile,
		data:     string(data),
		match:    make([]string, 0),
		original: true,
	}
}

func readDir(chOut chan<- Item, indir, subdir string, i, n int, firstfilter string) {
	dirname := indir
	if subdir != "" {
		dirname = filepath.Join(indir, subdir)
	}
	entries, err := os.ReadDir(dirname)
	x(err)
	size := len(entries) + 1
	for j, entry := range entries {
		j++
		name := entry.Name()
		if subdir != "" {
			name = filepath.Join(subdir, name)
		}
		fullname := filepath.Join(indir, name)
		if entry.IsDir() {
			readDir(chOut, indir, name, j+1, size, firstfilter)
			continue
		}
		if strings.HasSuffix(name, ".dact") || strings.HasSuffix(name, ".dbxml") {
			readDact(chOut, fullname, j+1, size, firstfilter, nil)
			continue
		}
		if strings.HasSuffix(name, ".data.dz") || strings.HasSuffix(name, ".index") {
			readCompact(chOut, fullname, j+1, size, nil)
			continue
		}
		if strings.HasSuffix(name, ".zip") {
			readZip(chOut, fullname, j+1, size, nil)
			continue
		}
		if !strings.HasSuffix(name, ".xml") {
			continue
		}
		if verbose {
			fmt.Fprintf(os.Stderr, " %d/%d %s -- %d/%d %s        \r", i, n, indir, j+1, size, name)
		}
		data, err := os.ReadFile(fullname)
		x(err)
		chOut <- Item{
			name:     fullname,
			oriname:  fullname,
			data:     string(data),
			match:    make([]string, 0),
			original: true,
		}
	}
}

func mustNotExist(filename string) {
	_, err := os.Stat(filename)
	if err == nil {
		x(fmt.Errorf("File exists: %s", filename))
	}
}

func expandMacros(s string) string {
	if macrofile == "" {
		return s
	}

	if len(macros) == 0 {
		b, err := os.ReadFile(macrofile)
		x(err)
		for _, set := range macroRE.FindAllStringSubmatch(macroCOM.ReplaceAllLiteralString(string(b), ""), -1) {
			s := strings.Replace(set[2], "\r\n", "\n", -1)
			s = strings.Replace(s, "\n\r", "\n", -1)
			s = strings.Replace(s, "\r", "\n", -1)
			macros["%"+set[1]+"%"] = untabify(s)
		}
	}

	if showExpansion {
		fmt.Println(strings.Repeat("=", 72))
	}
	original := s
	for i := 0; ; i++ {
		if i == 100 || len(s) > 65535 {
			fmt.Fprintln(os.Stderr, "Macro recursion too deep in:", original)
			os.Exit(1)
		}
		if showExpansion {
			fmt.Printf("%d: %s\n", i, s)
			fmt.Println(strings.Repeat("-", 72))
		}
		s2 := macroKY.ReplaceAllStringFunc(s, func(match string) string {
			r, ok := macros[match]
			if !ok {
				fmt.Fprintln(os.Stderr, "Undefined macro:", match)
				os.Exit(1)
			}
			return r
		})
		if s == s2 {
			break
		}
		s = s2
	}

	return s
}

func untabify(s string) string {
	var b bytes.Buffer
	i := 0
	for _, chr := range s {
		i++
		if chr == '\n' {
			i = 0
			b.WriteRune('\n')
		} else if chr == '\t' {
			b.WriteRune(' ')
			for (i % 8) != 0 {
				i++
				b.WriteRune(' ')
			}
		} else {
			b.WriteRune(chr)
		}
	}
	return strings.TrimSpace(b.String())
}
