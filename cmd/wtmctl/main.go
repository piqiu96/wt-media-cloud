package main

import (
	"context"
	"os"

	"github.com/wt-media/wt-media-cloud/internal/deploy"
)

func main() {
	runner := deploy.Runner{Stdout: os.Stdout, Stderr: os.Stderr}
	os.Exit(runner.Run(context.Background(), os.Args[1:]))
}
