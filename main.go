// Copyright©2021 Eric Grosse n2vi.com/0BSD

/*
Command lsr prints a recursive listing of the current directory, including
quoted filename, size, modtime, and sha256(contents).

This is a minor variation of a command I've been using since the early '80s
to organize backups, mirror projects, and check for filesystem corruption.
Its use in administering netlib is described in ACM TOMS (1995) 21:1:89-97.
*/
package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"time"
)

func main() {
	filepath.Walk(".", putfile)
}

func putfile(path string, info fs.FileInfo, err error) error {
	if err != nil {
		return err
	}
	if info.Mode().IsRegular() {
		fmt.Printf("%q\t%d\t%s\t%0x\n",
			path, info.Size(),
			info.ModTime().UTC().Format(time.RFC3339),
			sum(path))
	}
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
