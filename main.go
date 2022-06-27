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

This is not a security tool. Anyone that can maliciously change a file can
just as well change .lsr. But it is a good way to review what you've worked
on in the recent past or catch unintended changes.

Lsr is a minor variation of a command I've been using since the early '80s
to organize backups, mirror projects, and check for filesystem corruption.
Its use in administering netlib is described in ACM TOMS (1995) 21:1:89-97.
I created it at Bell Labs when the Interdata hardware (first Unix port)
was silently corrupting files.
*/
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type info struct {
	name string // relative to "."
	size int64
	mtime time.Time
	sum []byte
	eof bool
}

var oldinfo info
var newlsr *os.File
var oldscan *bufio.Scanner

func main() {
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
	getOldinfo()

	filepath.Walk(".", gotNewinfo)

	// TODO remaining lines from oldlsr are 'd'

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
	fld := strings.Split(oldscan.Text(), "\t")
	if len(fld) != 4 {
		log.Fatalf("bad input at %s", fld[0])
	}
	oldinfo.name, err = strconv.Unquote(fld[0])
	if err != nil {
		log.Fatalf(".lsr %s: %s", fld[0], err)
	}
	oldinfo.size, err = strconv.ParseInt(fld[1], 10, 64)
	if err != nil {
		log.Fatalf(".lsr %s, %s not an int64: %s", fld[0], fld[1], err)
	}
	oldinfo.mtime, err = time.Parse(time.RFC3339, fld[2])
	if err != nil {
		log.Fatalf(".lsr %s, %s bad time format: %s", fld[0], fld[2], err)
	}
	oldinfo.sum, err = hex.DecodeString(fld[3])
	if err != nil {
		log.Fatalf(".lsr %s, %s not hex format: %s", fld[0], fld[3], err)
	}
}

func gotNewinfo(path string, info fs.FileInfo, err error) error {
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || path==".lsr" || path==".lsrTEMPORARY" {
		return nil
	}
	newsum := sum(path)
	fmt.Fprintf(newlsr, "%q\t%d\t%s\t%0x\n",
		path, info.Size(),
		info.ModTime().UTC().Format(time.RFC3339),
		newsum)
	for !oldinfo.eof && oldinfo.name < path {
		fmt.Printf("D %s\n", oldinfo.name)
		getOldinfo()
	}
	if oldinfo.eof || oldinfo.name > path {
		fmt.Printf("N %s\n", path)
		return nil
	}
	// now oldinfo.eof = false && oldinfo.name == path
	newmtime := info.ModTime()
	if oldinfo.size != info.Size() || !bytes.Equal(oldinfo.sum, newsum) {
		if oldinfo.mtime.Before(newmtime) {
			fmt.Printf("M %s\n", path)
		} else if oldinfo.mtime.After(newmtime) {
			fmt.Printf("R %s\n", path)
		} else {
			fmt.Printf("C %s\n", path)
		}
	} else {
		if math.Abs(oldinfo.mtime.Sub(newmtime).Seconds()) > 1. {
			fmt.Printf("T %s\n", path)
		} // else all fields equal; file is unchanged
	}
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
