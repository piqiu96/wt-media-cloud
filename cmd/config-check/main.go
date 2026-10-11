package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/wt-media/wt-media-cloud/internal/config"
)

func main() {
	config.MustInitializeRuntimePaths()
	paths := config.GetRuntimePaths()
	configDir := flag.String("config-dir", paths.Config, "configuration directory to validate")
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
