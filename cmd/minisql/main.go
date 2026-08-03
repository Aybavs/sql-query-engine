package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aybavs/sql-query-engine/internal/repl"
)

func main() {
	dir := flag.String("data", "examples", "directory holding schema.txt and CSV files")
	flag.Parse()

	cat, err := repl.LoadSchema(filepath.Join(*dir, "schema.txt"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "schema: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("minisql — one SQL statement per line (Ctrl-D to exit)")
	repl.Run(cat, *dir, os.Stdin, os.Stdout)
}
