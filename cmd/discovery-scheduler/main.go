package main

import (
	"log"

	"github.com/wt-media/wt-media-cloud/internal/bootstrap"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	runProcess, err := bootstrap.InitializeScheduler()
	if err != nil {
		return err
	}
	return runProcess()
}
