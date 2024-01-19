package main

// TODO: alto_v6: xslt3, xpath3
// TODO: macro's
// TODO: optie: bestanden vervangen in bestaand corpus (niet voor compact corpus)
// TODO: toon macro-expansie, in stappen
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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	//"runtime"
	"strings"
	"unsafe"

	"github.com/pebbe/compactcorpus"
	"github.com/pebbe/dbxml"
	"github.com/pebbe/util"
	"github.com/rug-compling/alud/v2"
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
	overwrite     = false
	variables     = []*C.char{
		C.CString("filename"),
		cEMPTY,
		C.CString("corpusname"),
		cEMPTY,
	}

	x = util.CheckErr
)

func usage() {
	fmt.Printf(
		`
Usage: %s [option...] [infile...] [action...] outfile

Options:

    -e              : show macro-expansion
    -m filename     : use this macrofile for xpath
    -o              : overwrite xml in existing output (only for dact, zip, and directory)
    -v name=value   : set global variable

You can also set the macrofile with the environment variable ALTO_MACROFILE
The option -m has precendence

You can use the option -v more than once
The variables "filename" and "corpusname" are set automatically

Actions:

    ud:add          : insert Universal Dependencies
    ud:rm           : remove Universal Dependencies

    ff:{filename}   : filter by filename (dact, compact, zip)
    fp:{expression} : filter by XPATH2 {expression}

    tq:{xqueryfile} : transform with XQuery {xqueryfile}
    ts:{stylefile}  : transform with XSLT2 {stylefile}
    tt:{template}   : transform with {template}

    Tq:{xqueryfile} : like tq, match data as input
    Ts:{stylefile}  : like ts, match data as input

    ac:sum          : aggregated match count
    ac:relative     : aggregated relative match count


Template placeholders:

    %%%%  %%
    %%c  corpusname
    %%f  filename
    %%b  file body
    %%i  id of matching node
    %%I  sentence id
    %%s  sentence
    %%S  colored sentence
    %%m  match
    %%M  match as tree
    %%w  match words
    %%d  metadata


Infile names can be given as arguments or/and one name per line on stdin

Examples:
    %s corpus.zip *.xml
    %s corpus.dact corpus.zip
    find . '-name *.xml' | %s corpus.zip

Valid infile names:
    *.xml
    *.dact (or *.dbxml)
    *.data.dz (or *.index)
    *.zip
    directory name

Valid outfile names:
    *.dact (or *.dbxml)
    *.data.dz (or *.index)
    *.zip
    *.txt
    -  (stdout)
    directory name

`,
		os.Args[0],
		os.Args[0],
		os.Args[0],
		os.Args[0])
}

func main() {
	macrofile = os.Getenv("ALTO_MACROFILE")

	argp := 1
	argc := len(os.Args)
	if argc == 1 {
		usage()
		return
	}
	for argp < argc && strings.HasPrefix(os.Args[argp], "-") {
		if arg := os.Args[argp]; arg == "-e" {
			showExpansion = true
			argp++
		} else if arg := os.Args[argp]; arg == "-h" {
			usage()
			return
		} else if arg == "-m" {
			if argp == argc-1 {
				fmt.Fprintln(os.Stderr, "Missing filename for option -m")
				return
			}
			macrofile = os.Args[argp+1]
			argp += 2
		} else if arg := os.Args[argp]; arg == "-o" {
			overwrite = true
			argp++
		} else if arg == "-v" {
			if argp == argc-1 {
				fmt.Fprintln(os.Stderr, "Missing variable for option -v")
				return
			}
			a := strings.SplitN(os.Args[argp+1], "=", 2)
			if len(a) != 2 || a[0] == "" || a[1] == "" {
				fmt.Fprintln(os.Stderr, "Invalid name=value for option -v:", os.Args[argp+1])
				return
			}
			variables = append(variables, C.CString(a[0]), C.CString(a[1]))
			argp += 2
		} else {
			fmt.Fprintln(os.Stderr, "Unknown option", arg)
			return
		}
	}

	p1 := argp
	for argp < argc-1 {
		if arg := os.Args[argp]; len(arg) > 2 && arg[2] == ':' {
			break
		} else {
			argp++
		}
	}
	p2 := argp

	p := argp
	for argp < argc {
		if arg := os.Args[argp]; len(arg) > 2 && arg[2] == ':' {
			argp++
		} else {
			break
		}
	}
	actions := os.Args[p:argp]

	if argp == argc {
		fmt.Fprintln(os.Stderr, "Missing output filename")
		return
	}

	if argp < argc-1 {
		fmt.Fprintf(os.Stderr, "Invalid final arguments %v, should be a single output filename\n", os.Args[argp:])
		return
	}

	outfile := os.Args[argp]

	if overwrite {
		if outfile == "-" ||
			strings.HasSuffix(outfile, ".data.dz") ||
			strings.HasSuffix(outfile, ".index") ||
			strings.HasSuffix(outfile, ".txt") {
			fmt.Fprintln(os.Stderr, "Option -o only valid for output to dact, zip, or directory")
			return
		}
	}

	inputfiles := make([]string, 0)
	if !util.IsTerminal(os.Stdin) {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			inputfiles = append(inputfiles, scanner.Text())
		}
		x(scanner.Err())
	}
	inputfiles = append(inputfiles, os.Args[p1:p2]...)

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
		if action == "ud:add" {
			chOut := make(chan Item, 100)
			go doUD(chIn, chOut)
			chIn = chOut
		} else if action == "ud:rm" {
			// TODO
		} else if act == "fp" {
			if i == 0 {
				firstFilter = arg
			}
			chOut := make(chan Item, 100)
			go filterXpath(chIn, chOut, arg)
			chIn = chOut
		} else if act == "tq" || act == "ts" || act == "Tq" || act == "Ts" {
			var lang C.Language
			switch act {
			case "tq", "Tq":
				lang = C.langXQUERY
			case "ts", "Ts":
				lang = C.langXSLT
			}
			chOut := make(chan Item, 100)
			go transformStylesheet(chIn, chOut, lang, act[0] == 'T', arg)
			chIn = chOut
		} else if act == "tt" {
			chOut := make(chan Item, 100)
			go transformTemplate(chIn, chOut, arg)
			chIn = chOut
		} else if act == "ac" {
			chOut := make(chan Item, 100)
			go aggregateCount(chIn, chOut, arg == "relative")
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
	} else if outfile == "-" {
		verbose = false
		go writeStdout(chIn)
	} else {
		go writeDir(chIn, outfile)
	}

	n := len(inputfiles)
	for i, infile := range inputfiles {
		infile = filepath.Clean(infile)
		if strings.HasSuffix(infile, ".data.dz") || strings.HasSuffix(infile, ".index") {
			readCompact(chStart, infile, i+1, n)
		} else if strings.HasSuffix(infile, ".dbxml") || strings.HasSuffix(infile, ".dact") {
			readDact(chStart, infile, i+1, n, firstFilter)
		} else if strings.HasSuffix(infile, ".zip") {
			readZip(chStart, infile, i+1, n)
		} else if strings.HasSuffix(infile, ".xml") {
			readXml(chStart, infile, i+1, n)
		} else {
			readDir(chStart, infile, "", i+1, n)
		}
	}

	close(chStart)

	<-chDone
	if verbose {
		fmt.Fprintln(os.Stderr)
	}
}

func aggregateCount(chIn <-chan Item, chOut chan<- Item, relative bool) {
	var sum int
	count := make(map[string]int)
	for item := range chIn {
		for _, m := range item.match {
			m = strings.TrimSpace(m)
			if _, ok := count[m]; !ok {
				count[m] = 0
			}
			count[m]++
			sum++
		}
	}
	keys := make([]string, 0, len(count))
	for key := range count {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, len(keys))
	for i, key := range keys {
		if relative {
			lines[i] = fmt.Sprintf("%8.4f  %s", float64(count[key])/float64(sum), key)
		} else {
			lines[i] = fmt.Sprintf("%8d  %s", count[key], key)
		}
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

func filterXpath(chIn <-chan Item, chOut chan<- Item, xpath string) {
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
				chOut <- item
			}
		}
		C.xq_free(result)
	}
	close(chOut)
}

func transformStylesheet(chIn <-chan Item, chOut chan<- Item, lang C.Language, useMatch bool, stylefile string) {
	// runtime.LockOSThread()

	b, err := os.ReadFile(stylefile)
	x(err)
	style := C.CString(string(b))

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

			result := C.xq_call(cs, style, lang, cEMPTY, C.int(len(variables)/2), &(variables[0]))

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
	if strings.HasSuffix(outfile, ".data.dz") {
		outfile = outfile[:len(outfile)-8]
	} else if strings.HasSuffix(outfile, ".index") {
		outfile = outfile[:len(outfile)-6]
	}
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

	mustNotExist(outfile)
	db, err := dbxml.OpenReadWrite(outfile)
	x(err)
	for item := range chIn {
		x(db.PutXml(item.name, item.data, false))
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
			fp.WriteString("\n")
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

func readCompact(chOut chan<- Item, infile string, i, n int) {
	if strings.HasSuffix(infile, ".data.dz") {
		infile = infile[:len(infile)-8]
	} else if strings.HasSuffix(infile, ".index") {
		infile = infile[:len(infile)-6]
	}
	if compactSeen[infile] {
		return
	}
	compactSeen[infile] = true
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

func readDact(chOut chan<- Item, infile string, i, n int, filter string) {
	// runtime.LockOSThread()

	db, err := dbxml.OpenRead(infile)
	x(err)
	defer db.Close()
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

func readZip(chOut chan<- Item, infile string, i, n int) {
	zr, err := zip.OpenReader(infile)
	x(err)
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

func readDir(chOut chan<- Item, indir, subdir string, i, n int) {
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
		if entry.IsDir() {
			readDir(chOut, indir, name, i, n)
			continue
		}
		// TODO: ook dact, compact, zip
		if !strings.HasSuffix(name, ".xml") {
			continue
		}
		if verbose {
			fmt.Fprintf(os.Stderr, " %d/%d %s -- %d/%d %s        \r", i, n, indir, j+1, size, name)
		}
		fullname := filepath.Join(indir, name)
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
