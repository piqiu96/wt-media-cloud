package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/model"
	"sort"
	"strings"
)

var ErrImportConflict = model.ErrImportConflict

const ImportMaxRows = 100
const ImportMaxBytes = 2 * 1024 * 1024

type ImportRow struct {
	RowNumber   int    `json:"row_number"`
	InputCookie string `json:"input_cookie"`
}
type ImportInput struct {
	RequestKey  string                       `json:"request_key"`
	Platform    Platform                     `json:"platform"`
	GroupID     string                       `json:"group_id"`
	CategoryIDs []identityservice.CategoryID `json:"category_ids"`
	Tags        []string                     `json:"tags"`
	Remark      string                       `json:"remark"`
	Items       []ImportRow                  `json:"items"`
}
type ImportPreviewRow struct {
	RowNumber int    `json:"row_number"`
	Valid     bool   `json:"valid"`
	Message   string `json:"message"`
}
type ImportMutation struct {
	ExpectedRevision   uint64           `json:"expected_revision"`
	Attempt            uint64           `json:"attempt"`
	Step               string           `json:"step"`
	Action             string           `json:"action"`
	CandidateProfileID string           `json:"candidate_profile_id"`
	Confirmed          bool             `json:"confirmed"`
	BrowserProfileID   string           `json:"browser_profile_id"`
	Fingerprint        string           `json:"fingerprint"`
	BeforeCookie       []map[string]any `json:"before_cookie"`
	CookieResultInput
}
type ImportView struct {
	model.ImportBatch
	Counts       map[string]int    `json:"counts"`
	Execution    *model.ImportItem `json:"execution,omitempty"`
	AllowExecute bool              `json:"allow_execute"`
	CanApply     bool              `json:"can_apply"`
	CanInspect   bool              `json:"can_inspect"`
}
type importStore interface {
	CreateImport(model.ImportBatch) (model.ImportBatch, error)
	GetImport(string, identityservice.UserID) (model.ImportBatch, error)
	ListImports(identityservice.UserID) ([]model.ImportBatch, error)
	MutateImport(string, identityservice.UserID, func(*model.ImportBatch, model.ImportResources) error) (model.ImportBatch, error)
}

func importDigest(v any) string {
	raw, _ := json.Marshal(v)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func importMarker(b string, row int, attempt uint64) string {
	return fmt.Sprintf("WT_IMPORT:%s:%d:%d", b, row, attempt)
}
func projectImport(b model.ImportBatch) ImportView {
	counts := map[string]int{"total": len(b.Content.Items), "pending": 0, "running": 0, "succeeded": 0, "failed": 0, "skipped": 0, "uncertain": 0}
	b.Content.RequestDigest = ""
	items := make([]model.ImportItem, len(b.Content.Items))
	copy(items, b.Content.Items)
	for n := range items {
		i := &items[n]
		counts[i.Status]++
		i.InputCookie = ""
		i.BeforeCookie = nil
		i.Fingerprint = ""
		i.LastResultDigest = ""
		i.LastResultStep = ""
		i.PreviousStatus = ""

	}
	b.Content.Items = items
	return ImportView{ImportBatch: b, Counts: counts}
}
func (s *accountService) importStore(actor identityservice.PublicUser) (importStore, error) {
	if !validActor(actor) || !CanStartLocalOperation(actor.Role) {
		return nil, ErrForbidden
	}
	st, ok := s.store.(importStore)
	if !ok {
		return nil, ErrProfileUnavailable
	}
	return st, nil
}
func validateImportCookie(platform Platform, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 1048576 {
		return ErrInvalidInput
	}
	domains := map[Platform][]string{PlatformBilibili: {"bilibili.com", "biliapi.net", "biliapi.com"}, PlatformBaidu: {"baidu.com"}, PlatformDouyin: {"douyin.com"}}
	if strings.HasPrefix(raw, "[") || strings.HasPrefix(raw, "{") {
		var v any
		if json.Unmarshal([]byte(raw), &v) != nil {
			return ErrInvalidInput
		}
		if m, ok := v.(map[string]any); ok {
			v = m["cookies"]
		}
		arr, ok := v.([]any)
		if !ok || len(arr) == 0 {
			return ErrInvalidInput
		}
		for _, v := range arr {
			c, ok := v.(map[string]any)
			if !ok {
				return ErrInvalidInput
			}
			name, nok := c["name"].(string)
			value, vok := c["value"].(string)
			domain, _ := c["domain"].(string)
			domain = strings.TrimPrefix(strings.ToLower(domain), ".")
			match := false
			for _, d := range domains[platform] {
				if domain == d || strings.HasSuffix(domain, "."+d) {
					match = true
				}
			}
			if !nok || !vok || !match || strings.ContainsAny(name, "\r\n;=") || strings.ContainsAny(value, "\r\n") {
				return ErrInvalidInput
			}
		}
	} else {
		parsed := 0
		for _, pair := range strings.Split(raw, ";") {
			if strings.TrimSpace(pair) == "" {
				continue
			}
			name, _, ok := strings.Cut(pair, "=")
			if !ok || strings.TrimSpace(name) == "" || strings.ContainsAny(pair, "\r\n") {
				return ErrInvalidInput
			}
			parsed++
		}
		if parsed == 0 {
			return ErrInvalidInput
		}
	}
	return nil
}
func (s *accountService) PreviewImport(actor identityservice.PublicUser, in ImportInput) ([]ImportPreviewRow, error) {
	if _, e := s.importStore(actor); e != nil {
		return nil, e
	}
	if !validPlatform(in.Platform) || len(in.Items) == 0 || len(in.Items) > ImportMaxRows {
		return nil, ErrInvalidInput
	}
	seen := map[int]bool{}
	bytes := 0
	out := make([]ImportPreviewRow, 0, len(in.Items))
	for _, i := range in.Items {
		bytes += len(i.InputCookie)
		if bytes > ImportMaxBytes || i.RowNumber <= 0 || i.RowNumber > ImportMaxRows || seen[i.RowNumber] {
			return nil, ErrInvalidInput
		}
		seen[i.RowNumber] = true
		valid := validateImportCookie(in.Platform, i.InputCookie) == nil
		message := "可接入"
		if !valid {
			message = "Cookie 格式或平台不正确"
		}
		out = append(out, ImportPreviewRow{i.RowNumber, valid, message})
	}
	return out, nil
}
func (s *accountService) CreateImport(actor identityservice.PublicUser, in ImportInput) (ImportView, error) {
	st, e := s.importStore(actor)
	if e != nil {
		return ImportView{}, e
	}
	in.Items = append([]ImportRow(nil), in.Items...)
	sort.Slice(in.Items, func(i, j int) bool { return in.Items[i].RowNumber < in.Items[j].RowNumber })
	rows, e := s.PreviewImport(actor, in)
	if e != nil {
		return ImportView{}, e
	}
	for _, r := range rows {
		if !r.Valid {
			return ImportView{}, ErrInvalidInput
		}
	}
	if strings.TrimSpace(in.RequestKey) == "" || len(in.RequestKey) > 64 || strings.TrimSpace(in.GroupID) == "" || len(in.GroupID) > 128 || len(in.Remark) > 500 {
		return ImportView{}, ErrInvalidInput
	}
	categories, e := s.resolveCreateCategoryIDs(in.CategoryIDs)
	if e != nil {
		return ImportView{}, e
	}
	if e = validateCategoryScope(actor, categories); e != nil {
		return ImportView{}, e
	}
	tags, e := normalizeTags(in.Tags)
	if e != nil {
		return ImportView{}, e
	}
	c := model.ImportContent{CategoryIDs: categories, Tags: tags, Remark: strings.TrimSpace(in.Remark), GroupID: strings.TrimSpace(in.GroupID), Items: []model.ImportItem{}}
	for _, r := range in.Items {
		c.Items = append(c.Items, model.ImportItem{RowNumber: r.RowNumber, InputCookie: strings.TrimSpace(r.InputCookie), Status: "pending", Step: "waiting"})
	}
	c.RequestDigest = importDigest(struct {
		Platform Platform
		Content  model.ImportContent
	}{in.Platform, c})
	now := s.now()
	b, e := st.CreateImport(model.ImportBatch{UserID: actor.ID, RequestKey: in.RequestKey, Platform: in.Platform, Content: c, CreatedAt: now, UpdatedAt: now})
	if e != nil {
		return ImportView{}, e
	}
	if b.Content.RequestDigest != c.RequestDigest {
		return ImportView{}, ErrImportConflict
	}
	return projectImport(b), nil
}
func (s *accountService) GetImport(actor identityservice.PublicUser, id string) (ImportView, error) {
	st, e := s.importStore(actor)
	if e != nil {
		return ImportView{}, e
	}
	b, e := st.GetImport(id, actor.ID)
	return projectImport(b), e
}
func (s *accountService) ListImports(actor identityservice.PublicUser) ([]ImportView, error) {
	st, e := s.importStore(actor)
	if e != nil {
		return nil, e
	}
	bs, e := st.ListImports(actor.ID)
	if e != nil {
		return nil, e
	}
	out := []ImportView{}
	for _, b := range bs {
		out = append(out, projectImport(b))
	}
	return out, nil
}
func (s *accountService) MutateImportItem(actor identityservice.PublicUser, id string, row int, kind string, in ImportMutation) (ImportView, error) {
	st, e := s.importStore(actor)
	if e != nil {
		return ImportView{}, e
	}
	var execution *model.ImportItem
	allow, apply, inspect := false, false, false
	b, e := st.MutateImport(id, actor.ID, func(b *model.ImportBatch, res model.ImportResources) error {
		var item *model.ImportItem
		for n := range b.Content.Items {
			if b.Content.Items[n].RowNumber == row {
				item = &b.Content.Items[n]
				break
			}
		}
		if item == nil {
			return ErrNotFound
		}
		i := item
		digest := importDigest(in.CookieResultInput)
		if kind == "result" {
			digest = importDigest(struct {
				Step    string
				Profile string
				FP      string
				Before  []map[string]any
				Result  CookieResultInput
			}{in.Step, in.BrowserProfileID, in.Fingerprint, in.BeforeCookie, in.CookieResultInput})
			if in.Attempt == i.Attempt && i.LastResultStep == in.Step && i.LastResultDigest == digest {
				return nil
			}
		}
		// Repeated starts may read the current receipt but never regain a grant.
		if kind == "start" && (i.Status == "running" || i.Status == "uncertain" || i.Status == "succeeded") {
			return nil
		}
		if in.ExpectedRevision != b.Revision {
			return ErrImportConflict
		}
		active := func() bool {
			for _, other := range b.Content.Items {
				if other.RowNumber != row && (other.Status == "running" || other.Status == "uncertain") {
					return true
				}
			}
			return false
		}
		expose := func() { copy := *i; execution = &copy }
		release := func() error { return res.ReleaseProfile(i.BrowserProfileID, i.OperationID) }
		switch kind {
		case "start":
			if i.Status != "pending" || active() {
				return ErrImportConflict
			}
			i.Attempt++
			i.OperationID = fmt.Sprintf("import:%s:%d:%d", id, row, i.Attempt)
			i.Status = "running"
			i.LastResultDigest = ""
			i.LastResultStep = ""
			i.Message = "正在准备窗口"
			if i.BrowserProfileID == "" {
				i.Step = "creating"
			} else {
				bit, e := res.ReserveProfile(i.BrowserProfileID, i.OperationID, b.Platform, "")
				if e != nil {
					return e
				}
				i.BitProfileID = bit.BitID
				i.WindowName = bit.Name
				i.Step = "preparing"
			}
			allow = true
			expose()
		case "actions":
			switch in.Action {
			case "skip":
				if i.Status == "running" || i.Status == "uncertain" || i.Status == "succeeded" || i.Status == "skipped" {
					return ErrImportConflict
				}
				i.PreviousStatus = i.Status
				i.Status = "skipped"
				i.Message = "已跳过"
				return release()
			case "restore":
				if i.Status != "skipped" {
					return ErrImportConflict
				}
				i.Status = i.PreviousStatus
				if i.Status != "failed" {
					i.Status = "pending"
				}
				i.Message = "已恢复"
			case "retry":
				if i.Status != "failed" || i.Step == "manual" || i.ExistingAccountID != "" {
					return ErrImportConflict
				}
				i.Status = "pending"
				if i.Step == "blocked" {
					i.BrowserProfileID = ""
					i.BitProfileID = ""
					i.BeforeCookie = nil
					i.Fingerprint = ""

				}
				i.Message = "等待重试"
			case "reuse":
				if !i.CreationUnavailable || i.Status != "failed" || !in.Confirmed || in.CandidateProfileID == "" || active() {
					return ErrImportConflict
				}
				bit, e := res.ReserveProfile(in.CandidateProfileID, i.OperationID, b.Platform, "")
				if e != nil {
					return e
				}
				i.BrowserProfileID = in.CandidateProfileID
				i.BitProfileID = bit.BitID
				i.WindowName = bit.Name
				i.Status = "pending"
				i.Step = "waiting"
				i.Message = "待检查空闲窗口"
				if e = release(); e != nil {
					return e
				}
			case "reconcile":
				if active() || (i.Status != "uncertain" && i.Status != "running" && !(i.Status == "failed" && i.Step == "manual")) {
					return ErrImportConflict
				}
				if i.Step == "preparing" {
					i.Status = "failed"
					i.Message = "尚未开始写入，可重新发起"
					return release()
				}
				if i.BrowserProfileID != "" {
					bit, e := res.ReserveProfile(i.BrowserProfileID, i.OperationID, b.Platform, "")
					if e != nil {
						return e
					}
					i.BitProfileID = bit.BitID
					i.WindowName = bit.Name
				}
				inspect = true
				expose()
			default:
				return ErrInvalidInput
			}
		case "result":
			if in.Attempt != i.Attempt || (i.Status != "running" && i.Status != "uncertain" && !(i.Status == "failed" && i.Step == "manual")) {
				return ErrImportConflict
			}
			switch in.Step {
			case "created":
				if i.Step != "creating" || in.BrowserProfileID == "" {
					return ErrImportConflict
				}
				bit, e := res.ReserveProfile(in.BrowserProfileID, i.OperationID, b.Platform, importMarker(id, row, i.Attempt))
				if e != nil {
					return e
				}
				i.BrowserProfileID = in.BrowserProfileID
				i.BitProfileID = bit.BitID
				i.WindowName = bit.Name
				i.Status = "running"
				i.Step = "preparing"
				allow = true
				expose()
			case "creation_failed":
				if i.Step != "creating" {
					return ErrImportConflict
				}
				i.Status = "failed"
				i.CreationUnavailable = true
				i.Message = "无法新建窗口，请选择空闲窗口或稍后重试"
			case "creation_unknown":
				if i.Step != "creating" {
					return ErrImportConflict
				}
				i.Status = "uncertain"
				i.Message = "窗口创建结果待确认，请先核实"
			case "preparation_failed":
				if i.Step != "preparing" {
					return ErrImportConflict
				}
				i.Status = "failed"
				i.Message = "窗口检查未完成，请重试"
				if e := release(); e != nil {
					return e
				}
			case "prepared":
				if i.Step != "preparing" || in.BeforeCookie == nil || len(in.Fingerprint) != 64 {
					return ErrImportConflict
				}
				if in.LoginStatus != "not_logged_in" && in.LoginStatus != "expired" || in.PlatformAccountID != "" || len(in.BeforeCookie) > 0 {
					i.Status = "failed"
					i.Step = "blocked"
					i.Message = "窗口已有平台登录信息或无法确认，请使用新窗口"
					if e := release(); e != nil {
						return e
					}
					break
				}
				i.BeforeCookie = &in.BeforeCookie

				i.Fingerprint = in.Fingerprint
				i.Step = "applying"
				i.Message = "正在接入"
				apply = true
				expose()
			case "completed":
				if i.Step != "applying" && i.Step != "manual" {
					return ErrImportConflict
				}
				if in.Status == "applied" && in.LoginStatus == "normal" && in.PlatformAccountID != "" && in.Applied && len(in.Cookies) > 0 && i.BeforeCookie != nil && (in.InputMatches || (i.Step == "manual" && in.Verified)) {
					existing, e := res.FindIdentity(b.Platform, in.PlatformAccountID)
					if e != nil {
						return e
					}
					if existing != "" {
						i.Status = "failed"
						i.ExistingAccountID = existing
						i.Message = "此账号已存在，请查看已有账号"
						if e = release(); e != nil {
							return e
						}
						break
					}
					if _, e = res.ReserveProfile(i.BrowserProfileID, i.OperationID, b.Platform, ""); e != nil {
						return e
					}
					now := s.now()
					before, _ := json.Marshal(i.BeforeCookie)
					cookies, _ := json.Marshal(in.Cookies)
					receipt := &model.CookieUpdate{OperationID: i.OperationID, RequestKey: b.RequestKey, BrowserProfileID: i.BrowserProfileID, BitProfileID: i.BitProfileID, Platform: b.Platform, ExpectedPlatformAccountID: in.PlatformAccountID, Status: "succeeded", Step: "completed", Message: "接入完成"}
					a := model.AccountRecord{Account: model.Account{UserID: actor.ID, TeamID: actor.TeamID, Platform: b.Platform, PlatformAccountID: in.PlatformAccountID, PlatformName: in.Name, PlatformAvatarURL: in.AvatarURL, BrowserProfileID: i.BrowserProfileID, Enabled: true, IdentificationStatus: IdentificationIdentified, PlatformLoginStatus: PlatformLoginNormal, CategoryIDs: b.Content.CategoryIDs, Tags: b.Content.Tags, Remark: b.Content.Remark, CookieStatus: "active", CookieUpdatedAt: &now, LastCheckAt: &now, LastCheckIdentity: &model.CheckIdentity{PlatformAccountID: in.PlatformAccountID, PlatformName: in.Name, PlatformAvatarURL: in.AvatarURL}, CreatedAt: now, UpdatedAt: now}, OriginalCookie: i.InputCookie, ActiveCookie: string(cookies), BeforeCookie: string(before), CookieUpdate: receipt}
					accountID, e := res.CreateAccount(a)
					if e != nil {
						return e
					}
					i.AccountID = accountID
					i.Name = in.Name
					i.PlatformAccountID = in.PlatformAccountID
					i.Status = "succeeded"
					i.Step = "completed"
					i.Message = "接入成功"
					i.InputCookie = ""
					i.BeforeCookie = nil
					i.Fingerprint = ""

					if e = release(); e != nil {
						return e
					}
				} else if in.Applied && (in.LoginStatus == "not_logged_in" || in.LoginStatus == "expired" || in.LoginStatus == "verification_needed") {
					i.Status = "failed"
					i.Step = "manual"
					i.Message = "请在原窗口完成登录后检查并同步"
				} else if in.Status == "failed" && (!in.Applied || in.Verified) {
					i.Status = "failed"
					i.Message = in.Message
					if i.Message == "" {
						i.Message = "本次接入未完成，可重试"
					}
					if e := release(); e != nil {
						return e
					}
				} else {
					i.Status = "uncertain"
					i.Message = "接入结果待确认，请先核实"
				}
			case "uncertain":
				if i.Step != "applying" && i.Step != "manual" {
					return ErrImportConflict
				}
				i.Status = "uncertain"
				i.Message = "接入结果待确认，请先核实"
			default:
				return ErrInvalidInput
			}
			i.LastResultStep = in.Step
			i.LastResultDigest = digest
		default:
			return ErrInvalidInput
		}
		return nil
	})
	if e != nil {
		return ImportView{}, e
	}
	v := projectImport(b)
	v.Execution = execution
	v.AllowExecute = allow
	v.CanApply = apply
	v.CanInspect = inspect
	return v, nil
}
func PreviewImport(actor identityservice.PublicUser, in ImportInput) ([]ImportPreviewRow, error) {
	return defaultAccountService().PreviewImport(actor, in)
}
func CreateImport(actor identityservice.PublicUser, in ImportInput) (ImportView, error) {
	return defaultAccountService().CreateImport(actor, in)
}
func GetImport(actor identityservice.PublicUser, id string) (ImportView, error) {
	return defaultAccountService().GetImport(actor, id)
}
func ListImports(actor identityservice.PublicUser) ([]ImportView, error) {
	return defaultAccountService().ListImports(actor)
}
func MutateImportItem(actor identityservice.PublicUser, id string, row int, kind string, in ImportMutation) (ImportView, error) {
	return defaultAccountService().MutateImportItem(actor, id, row, kind, in)
}
