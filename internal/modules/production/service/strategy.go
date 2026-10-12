package service

import (
	"errors"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
	"github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

var (
	ErrStrategyInvalid   = errors.New("compose strategy input is invalid")
	ErrStrategyForbidden = errors.New("compose strategy operation is forbidden")
	ErrStrategyNotFound  = errors.New("compose strategy not found")
	ErrStrategyConflict  = errors.New("compose strategy version conflict")
)

type StrategyInput struct {
	TeamID           identity.TeamID          `json:"team_id"`
	GameID           string                   `json:"game_id"`
	Name             string                   `json:"name"`
	ContentType      string                   `json:"content_type"`
	TemplateKey      string                   `json:"template_key"`
	TemplateVersion  string                   `json:"template_version"`
	Parameters       model.StrategyParameters `json:"parameters"`
	AllowedOverrides []string                 `json:"allowed_overrides"`
	Status           string                   `json:"status"`
}

type StrategyStore interface {
	CreateStrategy(model.ComposeStrategy) (model.ComposeStrategy, error)
	UpdateStrategy(model.ComposeStrategy, uint32) (model.ComposeStrategy, bool, error)
	FindStrategy(int64) (model.ComposeStrategy, bool, error)
	ListStrategies(repository.StrategyFilter) ([]model.ComposeStrategy, error)
	ListStrategyVersions(int64) ([]model.ComposeStrategyVersion, error)
}

type StrategyService struct {
	store      StrategyStore
	gameExists func(string) (bool, error)
	now        func() time.Time
}

func NewStrategyService(store StrategyStore, gameExists func(string) (bool, error)) *StrategyService {
	return &StrategyService{store: store, gameExists: gameExists, now: time.Now}
}

func wiredStrategyService() *StrategyService {
	return NewStrategyService(repository.StrategyStore{}, func(id string) (bool, error) {
		game, found, err := identityservice.ResolveGame(id)
		return found && game.Status == identityservice.GameStatusEnabled, err
	})
}

func ListStrategyTemplates(actor identityservice.PublicUser) ([]model.StrategyTemplate, error) {
	if actor.Status != identityservice.UserStatusEnabled {
		return nil, ErrStrategyForbidden
	}
	return model.StrategyTemplates(), nil
}

func CreateStrategy(actor identityservice.PublicUser, input StrategyInput) (model.ComposeStrategy, error) {
	return wiredStrategyService().Create(actor, input)
}
func UpdateStrategy(actor identityservice.PublicUser, id int64, expected uint32, input StrategyInput) (model.ComposeStrategy, error) {
	return wiredStrategyService().Update(actor, id, expected, input)
}
func GetStrategy(actor identityservice.PublicUser, id int64) (model.ComposeStrategy, error) {
	return wiredStrategyService().Get(actor, id)
}
func ListStrategies(actor identityservice.PublicUser) ([]model.ComposeStrategy, error) {
	return wiredStrategyService().List(actor)
}
func CopyStrategy(actor identityservice.PublicUser, id int64, name string) (model.ComposeStrategy, error) {
	return wiredStrategyService().Copy(actor, id, name)
}
func ListStrategyVersions(actor identityservice.PublicUser, id int64) ([]model.ComposeStrategyVersion, error) {
	return wiredStrategyService().Versions(actor, id)
}

func (s *StrategyService) validate(actor identityservice.PublicUser, input StrategyInput) (StrategyInput, error) {
	if actor.Status != identityservice.UserStatusEnabled || (actor.Role != identityservice.RoleSeniorOperator && actor.Role != identityservice.RoleAdmin) {
		return input, ErrStrategyForbidden
	}
	input.GameID = strings.TrimSpace(input.GameID)
	input.Name = strings.TrimSpace(input.Name)
	input.ContentType = strings.TrimSpace(input.ContentType)
	if input.TeamID <= 0 || input.GameID == "" || len(input.GameID) > 32 || input.Name == "" || utf8.RuneCountInString(input.Name) > 120 || input.ContentType == "" || len(input.ContentType) > 64 || (input.Status != "active" && input.Status != "inactive") {
		return input, ErrStrategyInvalid
	}
	if actor.Role != identityservice.RoleAdmin && (actor.TeamID == nil || *actor.TeamID != input.TeamID || !hasGame(actor, input.GameID)) {
		return input, ErrStrategyForbidden
	}
	if err := model.ValidateStrategyParameters(input.TemplateKey, input.TemplateVersion, input.Parameters, input.AllowedOverrides); err != nil {
		return input, ErrStrategyInvalid
	}
	input.AllowedOverrides = append([]string{}, input.AllowedOverrides...)
	if s.gameExists != nil {
		found, err := s.gameExists(input.GameID)
		if err != nil {
			return input, err
		}
		if !found {
			return input, ErrStrategyInvalid
		}
	}
	return input, nil
}

func hasGame(actor identityservice.PublicUser, gameID string) bool {
	for _, game := range actor.GameIDs {
		if game == gameID {
			return true
		}
	}
	return false
}

func visibleStrategy(actor identityservice.PublicUser, row model.ComposeStrategy) bool {
	if actor.Status != identityservice.UserStatusEnabled {
		return false
	}
	if actor.Role == identityservice.RoleAdmin {
		return true
	}
	if actor.TeamID == nil || *actor.TeamID != row.TeamID || !hasGame(actor, row.GameID) {
		return false
	}
	if actor.Role == identityservice.RoleSeniorOperator {
		return true
	}
	return actor.Role == identityservice.RoleOperator && row.Status == "active"
}

func (s *StrategyService) Create(actor identityservice.PublicUser, input StrategyInput) (model.ComposeStrategy, error) {
	input, err := s.validate(actor, input)
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	now := s.now().UTC()
	return s.store.CreateStrategy(model.ComposeStrategy{TeamID: input.TeamID, GameID: input.GameID, Name: input.Name, ContentType: input.ContentType, TemplateKey: input.TemplateKey, TemplateVersion: input.TemplateVersion, Parameters: input.Parameters, AllowedOverrides: input.AllowedOverrides, Status: input.Status, CurrentVersion: 1, CreatedBy: actor.ID, UpdatedBy: actor.ID, CreatedAt: now, UpdatedAt: now})
}

func (s *StrategyService) Get(actor identityservice.PublicUser, id int64) (model.ComposeStrategy, error) {
	if id <= 0 {
		return model.ComposeStrategy{}, ErrStrategyInvalid
	}
	row, found, err := s.store.FindStrategy(id)
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	if !found || !visibleStrategy(actor, row) {
		return model.ComposeStrategy{}, ErrStrategyNotFound
	}
	return row, nil
}

func (s *StrategyService) List(actor identityservice.PublicUser) ([]model.ComposeStrategy, error) {
	if actor.Status != identityservice.UserStatusEnabled {
		return nil, ErrStrategyForbidden
	}
	filter := repository.StrategyFilter{IncludeInactive: actor.Role != identityservice.RoleOperator}
	if actor.Role != identityservice.RoleAdmin {
		if actor.TeamID == nil || len(actor.GameIDs) == 0 {
			return []model.ComposeStrategy{}, nil
		}
		filter.TeamID, filter.GameIDs = actor.TeamID, actor.GameIDs
	}
	rows, err := s.store.ListStrategies(filter)
	if err != nil {
		return nil, err
	}
	visible := make([]model.ComposeStrategy, 0, len(rows))
	for _, row := range rows {
		if visibleStrategy(actor, row) {
			visible = append(visible, row)
		}
	}
	return visible, nil
}

func (s *StrategyService) Update(actor identityservice.PublicUser, id int64, expected uint32, input StrategyInput) (model.ComposeStrategy, error) {
	if id <= 0 || expected == 0 || expected == math.MaxUint32 {
		return model.ComposeStrategy{}, ErrStrategyInvalid
	}
	old, found, err := s.store.FindStrategy(id)
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	if !found {
		return model.ComposeStrategy{}, ErrStrategyNotFound
	}
	if actor.Role != identityservice.RoleAdmin && actor.Role != identityservice.RoleSeniorOperator || !visibleStrategy(actor, old) {
		return model.ComposeStrategy{}, ErrStrategyForbidden
	}
	input, err = s.validate(actor, input)
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	if old.TeamID != input.TeamID || old.GameID != input.GameID {
		return model.ComposeStrategy{}, ErrStrategyInvalid
	}
	row := old
	row.Name, row.ContentType, row.TemplateKey, row.TemplateVersion = input.Name, input.ContentType, input.TemplateKey, input.TemplateVersion
	row.Parameters, row.AllowedOverrides, row.Status = input.Parameters, input.AllowedOverrides, input.Status
	row.UpdatedBy, row.UpdatedAt = actor.ID, s.now().UTC()
	updated, changed, err := s.store.UpdateStrategy(row, expected)
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	if !changed {
		return model.ComposeStrategy{}, ErrStrategyConflict
	}
	return updated, nil
}

func (s *StrategyService) Copy(actor identityservice.PublicUser, id int64, name string) (model.ComposeStrategy, error) {
	if actor.Role != identityservice.RoleAdmin && actor.Role != identityservice.RoleSeniorOperator {
		return model.ComposeStrategy{}, ErrStrategyForbidden
	}
	old, err := s.Get(actor, id)
	if err != nil {
		return model.ComposeStrategy{}, err
	}
	return s.Create(actor, StrategyInput{TeamID: old.TeamID, GameID: old.GameID, Name: name, ContentType: old.ContentType, TemplateKey: old.TemplateKey, TemplateVersion: old.TemplateVersion, Parameters: old.Parameters, AllowedOverrides: old.AllowedOverrides, Status: old.Status})
}

func (s *StrategyService) Versions(actor identityservice.PublicUser, id int64) ([]model.ComposeStrategyVersion, error) {
	if _, err := s.Get(actor, id); err != nil {
		return nil, err
	}
	return s.store.ListStrategyVersions(id)
}
