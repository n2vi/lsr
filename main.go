// Copyright©2021,2022 Eric Grosse n2vi.com/0BSD

/*
Command lsr prints a recursive listing of the current directory, including
quoted filename, size, modtime, and sha256(contents).

Main output goes to local file ".lsr", which is created 0600 if it doesn't exist.
Diagnostic output goes to Stdout as lines of the form: status filename
where status is one of

	N new
	D deleted
	M modified (size or hash changed, and mtime advanced)
	R reverted (size or hash changed, and mtime went backwards)
	T touched (mtime changed but hash did not)
	C corrupted (size or hash changed but mtime did not)

or silent for files that are same as before.
*/
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/ascii85"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math"
	"os"
	"time"
)

type info struct {
	name  string // relative to "."
	size  int64
	mtime time.Time
	sum   []byte
	eof   bool
}

var oldinfo info
var newlsr *os.File
var oldscan *bufio.Scanner
var relax bool

func main() {
	_, relax = os.LookupEnv("RELAX")
	oldlsr, err := os.OpenFile(".lsr", os.O_CREATE, 0600)
	if err != nil {
		log.Fatal(err)
	}
	newlsr, err = os.Create(".lsrTEMPORARY")
	if err != nil {
		log.Fatal(err)
	}
	err = newlsr.Chmod(0600)
	if err != nil {
		log.Fatal(err)
	}
	oldscan = bufio.NewScanner(oldlsr)
	oldinfo.sum = make([]byte, 32)
	getOldinfo()

	filesystem := os.DirFS(".")
	fs.WalkDir(filesystem, ".", gotNewinfo)

	for !oldinfo.eof {
		fmt.Printf("D %s\n", oldinfo.name)
		getOldinfo()
	}

	err = newlsr.Close()
	if err != nil {
		log.Fatal(err)
	}
	err = oldlsr.Close()
	if err != nil {
		log.Fatal(err)
	}
	// TODO  Change rename to copy, to preserve an .lsr symlink?
	err = os.Rename(".lsrTEMPORARY", ".lsr")
	if err != nil {
		log.Fatal(err)
	}
}

func getOldinfo() {
	// We are unforgiving here; we wrote it so we should be able to read it.
	var err error
	var timefld string
	var sumfld string
	if oldinfo.eof {
		return
	}
	if !oldscan.Scan() {
		err = oldscan.Err()
		if err != nil {
			log.Fatalf("reading .lsr: %s\n", err)
		}
		oldinfo.eof = true
		return
	}
	n, err := fmt.Sscanf(oldscan.Text(), "%q %d %s %s",
		&oldinfo.name, &oldinfo.size, &timefld, &sumfld)
	if err != nil || n != 4 {
		log.Fatalf("%s: %d %s", oldinfo.name, n, err)
	}
	oldinfo.mtime, err = time.Parse(time.RFC3339, timefld)
	if err != nil {
		log.Fatalf(".lsr %s, %s bad time format: %s", oldinfo.name, timefld, err)
	}
	ndst, _, err := ascii85.Decode(oldinfo.sum, []byte(sumfld), true)
	if err != nil || ndst != 32 {
		log.Fatalf(".lsr %s, %s not ascii85 format? %d %s",
			oldinfo.name, sumfld, ndst, err)
	}
}

func gotNewinfo(path string, d fs.DirEntry, err error) error {
	if err != nil {
		return err
	}
	info, err := d.Info()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || path == ".lsr" || path == ".lsrTEMPORARY" {
		return nil
	}
	newsize := info.Size()
	newmtime := info.ModTime()

	samesize := oldinfo.size == newsize
	sametime := math.Abs(oldinfo.mtime.Sub(newmtime).Seconds()) <= 1.

	newsum := make([]byte, sha256.Size)
	if relax && !oldinfo.eof && oldinfo.name == path && samesize && sametime {
		newsum = oldinfo.sum // good enough for some purposes
	} else {
		newsum = sum(path)
	}
	b85 := make([]byte, 40)
	ascii85.Encode(b85, newsum)
	fmt.Fprintf(newlsr, "%q %d %s %s\n",
		path, newsize,
		info.ModTime().UTC().Format(time.RFC3339),
		b85)
	cmp := pathCompare(oldinfo.name, path)
	for !oldinfo.eof && cmp < 0 {
		fmt.Printf("D %s\n", oldinfo.name)
		getOldinfo()
		cmp = pathCompare(oldinfo.name, path)
		samesize = oldinfo.size == newsize
		sametime = math.Abs(oldinfo.mtime.Sub(newmtime).Seconds()) <= 1.
	}
	if oldinfo.eof || cmp > 0 {
		fmt.Printf("N %s\n", path)
		return nil
	}
	// now oldinfo.eof = false && oldinfo.name == path
	if !samesize || !bytes.Equal(oldinfo.sum, newsum) {
		if oldinfo.mtime.Before(newmtime) {
			fmt.Printf("M %s\n", path)
		} else if oldinfo.mtime.After(newmtime) {
			fmt.Printf("R %s\n", path)
		} else {
			fmt.Printf("C %s\n", path)
		}
	} else if !sametime {
		fmt.Printf("T %s\n", path)
	}
	// else all fields equal; file is unchanged
	getOldinfo()
	return nil
}

func sum(path string) []byte {
	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		log.Fatal(err)
	}
	return h.Sum(nil)
}

// Function pathCompare is almost but not quite strings.Compare.
// The subtlety is that "a-b" sorts before "a/c" as strings, but "a/c"
// comes first in an fs.WalkDir enumeration.
func pathCompare(a, b string) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		aa, bb := a[i], b[i]
		if aa == '/' && bb != '/' {
			return -1
		}
		if aa != '/' && bb == '/' {
			return +1
		}
		if aa < bb {
			return -1
		}
		if aa > bb {
			return +1
		}
	}
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return +1
	}
	return 0
}
