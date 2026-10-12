package main

import (
	"fmt"
	"log"

	"github.com/wt-media/wt-media-cloud/internal/bootstrap"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/executor"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	closeResources, err := bootstrap.InitializeMigration()
	if err != nil {
		return err
	}
	defer closeResources()
	item, err := repository.PublishTemplate(executor.BasicCutV1(), "基础切段")
	if err != nil {
		return err
	}
	fmt.Printf("compose template published: id=%d code=%s version=%d status=%s\n", item.ID, item.Code, item.Version, item.Status)
	return nil
}
