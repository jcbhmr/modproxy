package main

import (
	"flag"
	"log"
	"os"
	"runtime/debug"
)

var version = func() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		panic("could not read build info")
	}
	return bi.Main.Version
}()

func Usage() {
	o := flag.CommandLine.Output()
	o.Write([]byte(`gomodzip ` + version + `
Prototype 'go mod zip'

usage: gomodzip [-o output] [path]

gomodzip creates a module zip file for the given path.

https://go.dev/ref/mod#zip-files

example:
gomodzip
gomodzip ./submodule
gomodzip -o tools0.31.0.zip
`))
	flag.PrintDefaults()
}

func Help() {
	flag.CommandLine.SetOutput(os.Stdout)
	flag.Usage()
	os.Exit(0)
}

var path string
var output string

func init() {
	flag.StringVar(&output, "o", "", "output file name")
	flag.Usage = Usage
}

func Parse() {
	flag.Parse()
	if output == "" {
		output = "gomod.zip"
	}
	if flag.NArg() > 1 {
		log.Println("too many arguments")
		flag.Usage()
		os.Exit(2)
	}
	path = flag.Arg(0)
	if path == "" {
		path = "."
	}
}

func main() {
	Parse()

	f, err := os.Create(output)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

}
