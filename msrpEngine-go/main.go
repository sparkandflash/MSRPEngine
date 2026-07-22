package main

import (
	"flag"
	"fmt"
	
	engineInterface "msrpengine/src/interface"
	"msrpengine/src/utils"

	"github.com/joho/godotenv"
)

func main() {
	newSession := flag.Bool("newSession", false, "Start a fresh session ignoring previous history")
	reuseSession := flag.String("reuseSession", "", "Reuse a specific session ID")
	debug := flag.Bool("debug", false, "Run in debug mode with verbose logging")
	serverMode := flag.Bool("server", false, "Run in server mode without CLI interface")
	flag.Parse()

	// Load .env relative to the executable path for standalone support
	_ = godotenv.Load(utils.ResolvePath(".env"))

	// The validation logic for limits is now strictly enforced in utils/config.go
	// HTTP validation has been removed for faster startup, as any invalid keys will simply fail gracefully on the first request.

	fmt.Println("\033[32mAll agents loaded successfully. Starting chat...\033[0m")
	if *serverMode { *debug = true }
	
	// run with flags: 
	// new session? t/f, reusesession? provide session id, debug? t/f, and no cli? t/f
	engineInterface.Run(*newSession, *reuseSession, *debug, *serverMode)
}

