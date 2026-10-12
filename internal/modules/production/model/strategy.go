package model

import (
	"fmt"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

type StrategyParameters struct {
	VideoRatio string `json:"video_ratio"`
	Subtitle   bool   `json:"subtitle"`
	BGM        bool   `json:"bgm"`
	Outro      bool   `json:"outro"`
}

type StrategyTemplate struct {
	Key              string   `json:"key"`
	Version          string   `json:"version"`
	Name             string   `json:"name"`
	VideoRatios      []string `json:"video_ratios"`
	AllowedOverrides []string `json:"allowed_overrides"`
}

// StrategyTemplates is the Cloud-owned catalog. Only basic_cut 1.0 has a
// defined parameter contract; media assets for subtitles, music, and outros
// are absent, so those switches cannot be enabled in this version.
func StrategyTemplates() []StrategyTemplate {
	return []StrategyTemplate{{
		Key: "basic_cut", Version: "1.0", Name: "基础剪辑",
		VideoRatios:      []string{"16:9", "9:16"},
		AllowedOverrides: []string{"video_ratio"},
	}}
}

func ValidateStrategyParameters(template, version string, params StrategyParameters, overrides []string) error {
	for _, candidate := range StrategyTemplates() {
		if candidate.Key != template || candidate.Version != version {
			continue
		}
		validRatio := false
		for _, ratio := range candidate.VideoRatios {
			validRatio = validRatio || ratio == params.VideoRatio
		}
		if !validRatio || params.Subtitle || params.BGM || params.Outro {
			return fmt.Errorf("unsupported parameters for %s %s", template, version)
		}
		seen := make(map[string]bool, len(overrides))
		for _, override := range overrides {
			allowed := false
			for _, field := range candidate.AllowedOverrides {
				allowed = allowed || field == override
			}
			if !allowed || seen[override] {
				return fmt.Errorf("unsupported or duplicate override %q", override)
			}
			seen[override] = true
		}
		return nil
	}
	return fmt.Errorf("unknown strategy template %s %s", template, version)
}

type ComposeStrategy struct {
	ID               int64              `json:"id"`
	TeamID           identity.TeamID    `json:"team_id"`
	GameID           string             `json:"game_id"`
	Name             string             `json:"name"`
	ContentType      string             `json:"content_type"`
	TemplateKey      string             `json:"template_key"`
	TemplateVersion  string             `json:"template_version"`
	Parameters       StrategyParameters `json:"parameters"`
	AllowedOverrides []string           `json:"allowed_overrides"`
	Status           string             `json:"status"`
	CurrentVersion   uint32             `json:"current_version"`
	CreatedBy        identity.UserID    `json:"created_by"`
	UpdatedBy        identity.UserID    `json:"updated_by"`
	CreatedAt        time.Time          `json:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at"`
}

type ComposeStrategyVersion struct {
	StrategyID int64           `json:"strategy_id"`
	Version    uint32          `json:"version"`
	Snapshot   ComposeStrategy `json:"snapshot"`
	CreatedBy  identity.UserID `json:"created_by"`
	CreatedAt  time.Time       `json:"created_at"`
}
