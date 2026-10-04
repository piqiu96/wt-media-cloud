package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/wt-media/wt-media-cloud/internal/config"
)

func main() {
	configDir := flag.String("config-dir", config.ConfigDir(), "configuration directory to validate")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "config-check does not accept positional arguments")
		os.Exit(2)
	}
	if _, err := config.LoadFromDir(*configDir); err != nil {
		fmt.Fprintf(os.Stderr, "configuration invalid: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("configuration valid")
}
