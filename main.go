package main

import (
	"flag"
	"lyra/interface"
)

func main() {
	newSession := flag.Bool("newSession", false, "Start a fresh session ignoring previous history")
	reuseSession := flag.String("reuseSession", "", "Reuse a specific session ID")
	debug := flag.Bool("debug", false, "Run in debug mode with verbose logging")
	flag.Parse()

	cli.Run(*newSession, *reuseSession, *debug)
}

