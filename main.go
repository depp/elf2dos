package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

func mainE() error {
	var output string
	flag.StringVar(&output, "output", "", "Output file")
	flag.Parse()
	if output == "" {
		return errors.New("flag -output is required")
	}
	args := flag.Args()
	if len(args) != 1 {
		return fmt.Errorf("got %d arguments, expected 1", len(args))
	}
	input := args[0]
	prog, err := readExecutable(input)
	if err != nil {
		return wrapError(err, input)
	}
	fp, err := os.Create(output)
	if err != nil {
		return err
	}
	defer fp.Close()
	if err := prog.Write(fp); err != nil {
		return err
	}
	return fp.Close() // Double-close is OK
}

func main() {
	if err := mainE(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
