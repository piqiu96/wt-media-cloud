package main

import (
	"log"

	"github.com/wt-media/wt-media-cloud/internal/bootstrap"
)

func main() {
	runProcess, err := bootstrap.InitializeComposeScheduler()
	if err != nil {
		log.Fatal(err)
	}
	if err := runProcess(); err != nil {
		log.Fatal(err)
	}
}
