package service

import "github.com/wt-media/wt-media-cloud/internal/config"

const platformRulesRevision = "2026.11.26.1"

var bilibiliStatements = []string{"内容无需标注", "含AI生成内容", "含虚构演绎内容", "内容含营销信息", "个人观点，仅供参考", "内容为转载"}

type CoverRule struct {
	Field       string   `json:"field"`
	WidthRatio  int      `json:"width_ratio"`
	HeightRatio int      `json:"height_ratio"`
	MaxBytes    int64    `json:"max_bytes"`
	Required    bool     `json:"required"`
	Formats     []string `json:"formats"`
}

type PlatformRule struct {
	Platform                string         `json:"platform"`
	Name                    string         `json:"name"`
	Enabled                 bool           `json:"enabled"`
	TitleMaxLength          int            `json:"title_max_length"`
	DescriptionMaxLength    int            `json:"description_max_length"`
	TagsMaxCount            int            `json:"tags_max_count"`
	TagMaxLength            int            `json:"tag_max_length"`
	LengthUnit              string         `json:"length_unit"`
	StatementOptions        []string       `json:"statement_options"`
	SourceMaxLength         int            `json:"source_max_length"`
	SourceRequiredStatement string         `json:"source_required_statement,omitempty"`
	VisibilityOptions       []string       `json:"visibility_options"`
	Cover                   CoverRule      `json:"cover"`
	SecondaryCover          CoverRule      `json:"secondary_cover"`
	Defaults                map[string]any `json:"defaults"`
}

type PlatformRules struct {
	Revision  string         `json:"revision"`
	Platforms []PlatformRule `json:"platforms"`
}

func GetPlatformRules() PlatformRules {
	return platformRules(config.Get().App.PublicationPlatforms)
}

func platformRules(cfg config.PublicationPlatformsConfig) PlatformRules {
	visibility := cfg.Bilibili.Visibility
	if visibility == "" {
		visibility = "public"
	}
	tags := append([]string{}, cfg.Bilibili.Tags...)
	cover := func(field string, width, height int) CoverRule {
		formats := []string{"png", "jpeg", "webp"}
		if field == "space_cover_file_id" || width == 4 && height == 3 {
			formats = []string{"png", "jpeg"}
		}
		return CoverRule{Field: field, WidthRatio: width, HeightRatio: height, MaxBytes: 5 * 1024 * 1024, Required: true, Formats: formats}
	}
	return PlatformRules{Revision: platformRulesRevision, Platforms: []PlatformRule{
		{Platform: "baidu", Name: "百家号", Enabled: true, TitleMaxLength: 50, DescriptionMaxLength: 50, LengthUnit: "unicode_code_points", StatementOptions: []string{"无需声明"}, VisibilityOptions: []string{"public"},
			Cover: cover("cover_file_id", 16, 9), SecondaryCover: cover("portrait_cover_file_id", 3, 4), Defaults: map[string]any{"declaration": "无需声明"}},
		{Platform: "bilibili", Name: "B站", Enabled: cfg.BilibiliEnabled, TitleMaxLength: 80, DescriptionMaxLength: 2000, TagsMaxCount: 10, TagMaxLength: 20, LengthUnit: "utf16_code_units",
			StatementOptions: append([]string{}, bilibiliStatements...), SourceMaxLength: 200, SourceRequiredStatement: "内容为转载", VisibilityOptions: []string{"public", "private"},
			Cover: cover("cover_file_id", 4, 3), SecondaryCover: cover("space_cover_file_id", 16, 9),
			Defaults: map[string]any{"partition_id": cfg.Bilibili.PartitionID, "partition_name": cfg.Bilibili.PartitionName, "tags": tags, "visibility": visibility}},
	}}
}

func enabledPlatform(platform string, cfg config.PublicationPlatformsConfig) bool {
	return platform == "baidu" || platform == "bilibili" && cfg.BilibiliEnabled
}

func (s *Service) platformEnabled(platform string) bool {
	return enabledPlatform(platform, s.PlatformSettings)
}

func platformEnabled(platform string) bool {
	if platform == "baidu" {
		return true
	}
	if platform != "bilibili" {
		return false
	}
	return enabledPlatform(platform, config.Get().App.PublicationPlatforms)
}
