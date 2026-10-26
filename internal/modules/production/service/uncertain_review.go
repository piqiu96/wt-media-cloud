package service

import (
	"fmt"
	"strings"
	"unicode/utf8"

	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
)

type ResolveComposeUncertainInput struct {
	OldExecutionStopped bool   `json:"old_execution_stopped"`
	NoOutputCommitted   bool   `json:"no_output_committed"`
	Evidence            string `json:"evidence"`
}

type ComposeUncertainResolution struct {
	Task   model.ComposeTask            `json:"task"`
	Review model.ComposeUncertainReview `json:"review"`
}

type composeUncertainReviewStore interface {
	Find(int64) (model.ComposeTask, bool, error)
	ResolveUncertain(taskID, actorID int64, evidence string) (model.ComposeUncertainReview, bool, error)
	ListUncertainReviews(taskID int64) ([]model.ComposeUncertainReview, error)
}

type ComposeUncertainReviewService struct{ store composeUncertainReviewStore }

func NewComposeUncertainReviewService(store composeUncertainReviewStore) *ComposeUncertainReviewService {
	return &ComposeUncertainReviewService{store: store}
}

func wiredComposeUncertainReviewService() *ComposeUncertainReviewService {
	return NewComposeUncertainReviewService(repository.ComposeTaskStore{})
}

func ResolveComposeUncertain(actor identity.PublicUser, taskID int64, input ResolveComposeUncertainInput) (ComposeUncertainResolution, error) {
	return wiredComposeUncertainReviewService().Resolve(actor, taskID, input)
}

func ListComposeUncertainReviews(actor identity.PublicUser, taskID int64) ([]model.ComposeUncertainReview, error) {
	return wiredComposeUncertainReviewService().List(actor, taskID)
}

// visibleReviewTask applies the read scope first (a task the actor cannot read
// answers 404), then the review-role gate: admins and same-team senior
// operators may review an uncertain execution. Operators who can read the task
// still get 403 — reading is not reviewing.
func (s *ComposeUncertainReviewService) visibleReviewTask(actor identity.PublicUser, taskID int64) (model.ComposeTask, error) {
	item, found, err := s.store.Find(taskID)
	if err != nil {
		return model.ComposeTask{}, err
	}
	if !found || !canReadComposeTask(actor, item) {
		return model.ComposeTask{}, ErrComposeTaskNotFound
	}
	if actor.Role != identity.RoleAdmin && actor.Role != identity.RoleSeniorOperator {
		return model.ComposeTask{}, ErrComposeTaskForbidden
	}
	return item, nil
}

func (s *ComposeUncertainReviewService) Resolve(actor identity.PublicUser, taskID int64, input ResolveComposeUncertainInput) (ComposeUncertainResolution, error) {
	if _, err := s.visibleReviewTask(actor, taskID); err != nil {
		return ComposeUncertainResolution{}, err
	}
	evidence := strings.TrimSpace(input.Evidence)
	count := utf8.RuneCountInString(evidence)
	if !input.OldExecutionStopped || !input.NoOutputCommitted || count < 20 || count > 2000 || strings.Contains(evidence, "://") {
		return ComposeUncertainResolution{}, fmt.Errorf("%w: explicit verification and a 20-2000 character evidence note are required; do not include URLs", ErrComposeTaskInvalid)
	}
	review, changed, err := s.store.ResolveUncertain(taskID, int64(actor.ID), evidence)
	if err != nil {
		return ComposeUncertainResolution{}, err
	}
	if !changed {
		return ComposeUncertainResolution{}, ErrComposeTaskNotRetryable
	}
	item, found, err := s.store.Find(taskID)
	if err != nil {
		return ComposeUncertainResolution{}, err
	}
	if !found {
		return ComposeUncertainResolution{}, ErrComposeTaskNotFound
	}
	return ComposeUncertainResolution{Task: item, Review: review}, nil
}

func (s *ComposeUncertainReviewService) List(actor identity.PublicUser, taskID int64) ([]model.ComposeUncertainReview, error) {
	if _, err := s.visibleReviewTask(actor, taskID); err != nil {
		return nil, err
	}
	return s.store.ListUncertainReviews(taskID)
}
