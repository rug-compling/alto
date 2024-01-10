package main

/*
#cgo LDFLAGS: -lxqilla -lxerces-c
#include <stdlib.h>
#include "mkcorpus.h"
*/
import "C"

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	// "runtime"
	"strings"
	"unsafe"

	"github.com/pebbe/compactcorpus"
	"github.com/pebbe/dbxml"
	"github.com/pebbe/util"
	"github.com/rug-compling/alud/v2"
)

type Item struct {
	name     string
	data     string
	match    string
	filtered bool // als het eerste XPath-filter al is toegepast bij inlezen vanuit DACT, dan kan het eerste filter alles doorlaten
	original bool // als dit een origineel XML-bestand is, dan hoeft er geen tijdelijk bestand gemaakt te worden voor XQilla
}

const (
	DEVIDER = "[[*gyeve!"
)

var (
	chDone      = make(chan bool)
	compactSeen = make(map[string]bool)
	tempdir     = os.TempDir()
	cDEVIDER    = C.CString(DEVIDER)

	x = util.CheckErr
)

func usage() {
	fmt.Fprintf(
		os.Stderr,
		`
Usage: %s outfile [action...] [infile...]

Actions:

    ud:add : insert Universal Dependencies
    ud:rm  : remove Universal Dependencies

    fp:{expression} : filter by XPATH2 {expression}

    ts:{stylefile}  : transform with XSLT2 {stylefile}
    tq:{xqueryfile} : transform with XQuery {xqueryfile}
    tt:{template}   : transform with Go {template}

Infile names can be given as arguments or/and on stdin one name per line

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
	actions := make([]string, 0)

	// backward compatibility
	if len(os.Args) > 1 && os.Args[1] == "-u" {
		actions = append(actions, "ud:add")
		os.Args = append(os.Args[:1], os.Args[2:]...)
	}

	if len(os.Args) == 1 {
		usage()
		return
	}

	idx := 2
	for idx < len(os.Args) {
		if len(os.Args[idx]) > 2 && os.Args[idx][2] == ':' {
			actions = append(actions, os.Args[idx])
			idx++
			continue
		}
		break
	}

	if len(os.Args) == idx && util.IsTerminal(os.Stdin) {
		usage()
		return
	}

	firstFilter := ""

	chStart := make(chan Item, 100)
	chIn := chStart

	for i, action := range actions {
		if action == "ud:add" {
			chOut := make(chan Item, 100)
			go doUD(chIn, chOut)
			chIn = chOut
		} else if action == "ud:rm" {
			// TODO
		} else if strings.HasPrefix(action, "fp:") {
			s := action[3:]
			if i == 0 {
				firstFilter = s
			}
			chOut := make(chan Item, 100)
			go filterXpath(chIn, chOut, s)
			chIn = chOut
		} else if strings.HasPrefix(action, "tq:") {
			// TODO: als transform toegepast, dan extensie .t toevoegen aan naam xml-bestanden
			// en original = false
			// TODO
		} else if strings.HasPrefix(action, "ts:") {
			// TODO: als transform toegepast, dan extensie .t toevoegen aan naam xml-bestanden
			// en original = false
			// TODO
		} else if strings.HasPrefix(action, "tt:") {
			// TODO: als transform toegepast, dan extensie .t toevoegen aan naam xml-bestanden
			// en original = false
			// TODO
		} else {
			fmt.Fprintf(os.Stderr, "Unknown action %q\n", action)
			return
		}
	}

	outfile := os.Args[1]

	if strings.HasSuffix(outfile, ".data.dz") || strings.HasSuffix(outfile, ".index") {
		go writeCompact(chIn, outfile)
	} else if strings.HasSuffix(outfile, ".dbxml") || strings.HasSuffix(outfile, ".dact") {
		go writeDact(chIn, outfile)
	} else if strings.HasSuffix(outfile, ".zip") {
		go writeZip(chIn, outfile)
		/*
			// TODO
			} else if strings.HasSuffix(outfile, ".txt") {
				go writeTxt(chIn, outfile)
			} else if outfile == "-" {
				go writeStdout(chIn)
		*/
	} else {
		go writeDir(chIn, outfile)
	}

	if !util.IsTerminal(os.Stdin) {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			os.Args = append(os.Args, scanner.Text())
		}
		x(scanner.Err())
	}

	n := len(os.Args) - idx
	for i, infile := range os.Args[idx:] {
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
	fmt.Fprintln(os.Stderr)
}

func doUD(chIn <-chan Item, chOut chan<- Item) {
	for item := range chIn {
		s, err := alud.UdAlpino([]byte(item.data), item.name, "")
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
	//	runtime.LockOSThread()

	cxpath := C.CString(xpath)
	xq := C.prepare(cxpath, C.langXPATH)
	C.free(unsafe.Pointer(cxpath))
	if C.xq_error(xq) != 0 {
		x(fmt.Errorf("prepare filter Xpath failed for %q", xpath))
	}

	for item := range chIn {
		if item.filtered {
			// eerste filter toegepast bij lezen vanuit dbxml-bestand
			item.filtered = false
			chOut <- item
			continue
		}

		if item.original {
			// oorspronkelijk, onveranderd xml-bestand
			cs := C.CString(item.name)
			output := C.run(xq, cs, cDEVIDER)
			C.free(unsafe.Pointer(cs))
			item.match = C.GoString(output)
			if len(item.match) > 0 {
				chOut <- item
			}
			continue
		}

		// Dit is niet efficient, steeds een nieuwe bestandsnaam aanmaken, maar als
		// je steeds dezelfde naam gebruikt denk xqilla dat het bestand al verwerkt is
		fp, err := os.CreateTemp(tempdir, "mkcFXP")
		x(err)
		_, err = fp.WriteString(item.data)
		x(err)
		filename := fp.Name()
		x(fp.Close())
		cs := C.CString(filename)
		output := C.run(xq, cs, cDEVIDER)
		C.free(unsafe.Pointer(cs))
		os.Remove(filename)
		item.match = C.GoString(output)
		if len(item.match) > 0 {
			chOut <- item
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
	//	runtime.LockOSThread()

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
		fmt.Fprintf(os.Stderr, " %d/%d %s -- %d/? %s        \r", i, n, infile, j, name)
		chOut <- Item{name: name, data: string(data)}
	}
}

func readDact(chOut chan<- Item, infile string, i, n int, filter string) {
	//	runtime.LockOSThread()

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
			fmt.Fprintf(os.Stderr, " %d/%d %s -- %d/%d %s        \r", i, n, infile, j, size, name)
			chOut <- Item{name: name, data: docs.Content()}
		}
	} else {
		docs, err := db.Query(filter)
		x(err)
		name := ""
		content := ""
		match := ""
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
					chOut <- Item{name: name, data: content, match: match, filtered: true}
				}
				name = newname
				match = ""
			}
			content = docs.Content()
			match += docs.Match() + DEVIDER
			fmt.Fprintf(os.Stderr, " %d/%d %s -- %d/? %s        \r", i, n, infile, j, name)
		}
		if content != "" {
			chOut <- Item{name: name, data: content, match: match, filtered: true}
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
		fmt.Fprintf(os.Stderr, " %d/%d %s -- %d/? %s        \r", i, n, infile, j+1, name)
		f, err := file.Open()
		x(err)
		data, err := io.ReadAll(f)
		x(err)
		chOut <- Item{name: name, data: string(data)}
	}
}

func readXml(chOut chan<- Item, infile string, i, n int) {
	fmt.Fprintf(os.Stderr, " %d/%d %s        \r", i, n, infile)
	data, err := os.ReadFile(infile)
	x(err)
	chOut <- Item{name: infile, data: string(data), original: true}
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
		if !strings.HasSuffix(name, ".xml") {
			continue
		}
		fmt.Fprintf(os.Stderr, " %d/%d %s -- %d/%d %s        \r", i, n, indir, j+1, size, name)
		data, err := os.ReadFile(filepath.Join(indir, name))
		x(err)
		chOut <- Item{name: name, data: string(data)}
	}
}

func mustNotExist(filename string) {
	_, err := os.Stat(filename)
	if err == nil {
		x(fmt.Errorf("File exists: %s", filename))
	}
}
