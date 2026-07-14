package main

import (
	"log"

	"github.com/wt-media/wt-media-cloud/internal/app"
)

func main() {
	server := app.NewServer()
	if err := server.Run(); err != nil {
		log.Fatal(err)
	}
}
