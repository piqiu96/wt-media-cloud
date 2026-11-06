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
	for _, entry := range []struct {
		definition executor.Definition
		name       string
	}{
		{executor.DynamicPIPEndingV1(), "动态画中画与配乐片尾"},
	} {
		item, err := repository.PublishTemplate(entry.definition, entry.name)
		if err != nil {
			return err
		}
		fmt.Printf("compose template published: id=%d code=%s version=%d status=%s\n", item.ID, item.Code, item.Version, item.Status)
	}
	return nil
}
