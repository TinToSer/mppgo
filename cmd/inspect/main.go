// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

// Command inspect dumps the CFB directory tree of a file, for inspection.
package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/tintoser/mppgo/cfb"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: inspect <file>")
		os.Exit(2)
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	defer f.Close()

	cf, err := cfb.Open(f)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	printTree(cf.Root, "")
}

func printTree(e *cfb.Entry, indent string) {
	kind := "stream"
	if e.IsStorage() {
		kind = "storage"
	}
	fmt.Printf("%s%q [%s] size=%d\n", indent, e.Name, kind, e.Size)
	if !e.IsStorage() {
		return
	}
	// Sort for stable output; the on-disk order is a red-black tree.
	names := make([]string, 0, len(e.Children))
	for name := range e.Children {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		printTree(e.Children[name], indent+"  ")
	}
}
