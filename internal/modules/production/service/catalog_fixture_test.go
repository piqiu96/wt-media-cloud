package service

import (
	"github.com/wt-media/wt-media-cloud/internal/modules/production/executor"
	"github.com/wt-media/wt-media-cloud/internal/testutil/composefixture"
)

func init() {
	released := deployedTemplate
	deployedTemplate = func(code string, version uint32) (executor.Definition, error) {
		for _, d := range []executor.Definition{composefixture.Cut(), composefixture.Resource()} {
			if d.Code == code && d.Version == version {
				return d, nil
			}
		}
		return released(code, version)
	}
}
