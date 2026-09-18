package repository

import (
	"testing"
	"time"

	identitymodel "github.com/wt-media/wt-media-cloud/internal/modules/identity/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding/model"
	"gorm.io/gorm"
)

func TestRuntimeBindingRepositoryExposesPackageFunctionsUsingGORM(t *testing.T) {
	var _ func(model.BindingTicket) error = CreateTicket
	var _ func(string, time.Time) (model.BindingTicket, bool, error) = ConsumeTicket
	var _ func(identitymodel.UserID, string, time.Time, time.Duration) (bool, error) = CheckLocalTrust
	var _ func(string) (model.AgentNode, bool, error) = FindNodeByCredentialHash

	var _ func(*gorm.DB, model.BindingTicket) error = createTicket
	var _ func(*gorm.DB, string, time.Time) (model.BindingTicket, bool, error) = consumeTicket
	var _ func(*gorm.DB, identitymodel.UserID, string, time.Time, time.Duration) (bool, error) = checkLocalTrust
	var _ func(*gorm.DB, string) (model.AgentNode, bool, error) = findNodeByCredentialHash
}
