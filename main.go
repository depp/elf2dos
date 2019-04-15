package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"

	"moria.us/elf2dos/module"
)

func cmdObjDump(input string) error {
	p, err := module.Open(input)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(os.Stdout)
	p.DumpText(w, "")
	if err := w.Flush(); err != nil {
		return err
	}
	return nil
}

func cmdConvert(input, output string) error {
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

func mainE() error {
	var output string
	var objdump bool
	flag.StringVar(&output, "output", "", "Output file")
	flag.BoolVar(&objdump, "objdump", false, "Dump input file")
	flag.Parse()
	args := flag.Args()
	if objdump {
		if len(args) != 1 {
			return fmt.Errorf("got %d arguments, expected 1", len(args))
		}
		return cmdObjDump(args[0])
	} else {
		if len(args) != 1 {
			return fmt.Errorf("got %d arguments, expected 1", len(args))
		}
		if output == "" {
			return errors.New("flag -output is required")
		}
		return cmdConvert(args[0], output)
	}
}

func main() {
	if err := mainE(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
