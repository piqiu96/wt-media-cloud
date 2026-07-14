package main

import (
	"log"

	"github.com/wt-media/wt-media-cloud/internal/app"
)

func main() {
	server, err := app.NewServer()
	if err != nil {
		log.Fatal(err)
	}
	defer server.Close()
	if err := server.Run(); err != nil {
		log.Fatal(err)
	}
}
