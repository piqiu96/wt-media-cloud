package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/model"
	"strings"
)

var ErrCookieConflict = model.ErrCookieConflict

type CookieStartInput struct {
	RequestKey          string `json:"request_key"`
	ExpectedOperationID string `json:"expected_operation_id"`
	InputCookie         string `json:"input_cookie"`
}
type CookieSnapshotInput struct {
	BeforeCookie      []map[string]any `json:"before_cookie"`
	Fingerprint       string           `json:"fingerprint"`
	LoginStatus       string           `json:"login_status"`
	PlatformAccountID string           `json:"platform_account_id"`
}
type CookieResultInput struct {
	Verified          bool             `json:"verified"`
	Status            string           `json:"status"`
	Applied           bool             `json:"applied"`
	Cookies           []map[string]any `json:"cookies"`
	PlatformAccountID string           `json:"platform_account_id"`
	Name              string           `json:"name"`
	AvatarURL         string           `json:"avatar_url"`
	LoginStatus       string           `json:"login_status"`
	InputMatches      bool             `json:"input_matches"`
	Message           string           `json:"message"`
}
type CookieUpdateView struct {
	OperationID               string   `json:"operation_id"`
	RequestKey                string   `json:"request_key"`
	BrowserProfileID          string   `json:"browser_profile_id"`
	BitProfileID              string   `json:"bit_profile_id"`
	Platform                  Platform `json:"platform"`
	ExpectedPlatformAccountID string   `json:"expected_platform_account_id"`
	InputCookie               string   `json:"input_cookie,omitempty"`
	Fingerprint               string   `json:"fingerprint,omitempty"`
	Status                    string   `json:"status"`
	Step                      string   `json:"step"`
	Message                   string   `json:"message"`
	AllowExecute              bool     `json:"allow_execute"`
	CanApply                  bool     `json:"can_apply"`
}

func cookieView(u *model.CookieUpdate) CookieUpdateView {
	if u == nil {
		return CookieUpdateView{}
	}
	return CookieUpdateView{OperationID: u.OperationID, RequestKey: u.RequestKey, BrowserProfileID: u.BrowserProfileID, BitProfileID: u.BitProfileID, Platform: u.Platform, ExpectedPlatformAccountID: u.ExpectedPlatformAccountID, InputCookie: u.InputCookie, Fingerprint: u.Fingerprint, Status: u.Status, Step: u.Step, Message: u.Message}
}

type cookieStore interface {
	MutateCookie(string, identityservice.UserID, func(*model.AccountRecord, *string) error) (model.AccountRecord, error)
}

func (s *accountService) cookieAccess(actor identityservice.PublicUser, id string) (AccountRecord, error) {
	if !CanStartLocalOperation(actor.Role) {
		return AccountRecord{}, ErrForbidden
	}
	return s.GetOwnedAccountRecord(actor, id)
}
func (s *accountService) cookieMutation(actor identityservice.PublicUser, id string, fn func(*model.AccountRecord, *string) error) (AccountRecord, error) {
	if _, e := s.cookieAccess(actor, id); e != nil {
		return AccountRecord{}, e
	}
	store, ok := s.store.(cookieStore)
	if !ok {
		return AccountRecord{}, ErrProfileUnavailable
	}
	return store.MutateCookie(id, actor.ID, fn)
}
func (s *accountService) StartCookieUpdate(actor identityservice.PublicUser, id string, input CookieStartInput) (CookieUpdateView, error) {
	if strings.TrimSpace(input.RequestKey) == "" || len(input.RequestKey) > 64 || strings.TrimSpace(input.InputCookie) == "" || len(input.InputCookie) > 1048576 {
		return CookieUpdateView{}, ErrInvalidInput
	}
	record, e := s.cookieAccess(actor, id)
	if e != nil {
		return CookieUpdateView{}, e
	}
	if !record.Enabled || s.profileFacts == nil {
		return CookieUpdateView{}, ErrProfileUnavailable
	}
	_, owner, bitID, inScope, active, found, e := s.profileFacts.ResolveProfileForAccountCheck(record.BrowserProfileID)
	if e != nil {
		return CookieUpdateView{}, e
	}
	if !found || !inScope || !active || owner != actor.ID || bitID == "" {
		return CookieUpdateView{}, ErrProfileUnavailable
	}
	sum := sha256.Sum256([]byte(input.InputCookie))
	digest := hex.EncodeToString(sum[:])
	fresh := false
	out, e := s.cookieMutation(actor, id, func(r *model.AccountRecord, lock *string) error {
		u := r.CookieUpdate
		if u != nil && u.RequestKey == input.RequestKey {
			if u.RequestDigest != digest {
				return ErrCookieConflict
			}
			return nil
		}
		if r.BrowserProfileID != record.BrowserProfileID || !r.Enabled || *lock != "" {
			return ErrCookieConflict
		}
		if u != nil && (u.OperationID != input.ExpectedOperationID || u.Status == "running" || u.Status == "uncertain") {
			return ErrCookieConflict
		}
		if u == nil && input.ExpectedOperationID != "" {
			return ErrCookieConflict
		}
		r.CookieUpdate = &model.CookieUpdate{RequestKey: input.RequestKey, RequestDigest: digest, OperationID: s.newID("cookie"), BrowserProfileID: r.BrowserProfileID, BitProfileID: bitID, Platform: r.Platform, ExpectedPlatformAccountID: r.PlatformAccountID, InputCookie: input.InputCookie, Status: "running", Step: "preparing", Message: "正在检查窗口"}
		r.CheckOperationID = ""
		if r.LastCheckStatus == "running" {
			r.LastCheckStatus = "failed"
			r.LastCheckMessage = "检查已由登录 Cookie 更新替代"
		}
		*lock = r.CookieUpdate.OperationID
		r.UpdatedAt = s.now()
		fresh = true
		return nil
	})
	if e != nil {
		return CookieUpdateView{}, e
	}
	v := cookieView(out.CookieUpdate)
	v.AllowExecute = fresh
	return v, nil
}
func (s *accountService) SnapshotCookieUpdate(actor identityservice.PublicUser, id, op string, input CookieSnapshotInput) (CookieUpdateView, error) {
	if input.BeforeCookie == nil || len(input.Fingerprint) != 64 {
		return CookieUpdateView{}, ErrInvalidInput
	}
	canApply := false
	out, e := s.cookieMutation(actor, id, func(r *model.AccountRecord, lock *string) error {
		u := r.CookieUpdate
		if u == nil || u.OperationID != op || u.BrowserProfileID != r.BrowserProfileID {
			return ErrCookieConflict
		}
		if u.Step != "preparing" {
			return nil
		}
		if *lock != op {
			return ErrCookieConflict
		}
		// Unknown or a different logged-in account is never treated as an empty window.
		known := input.LoginStatus == "normal" || input.LoginStatus == "not_logged_in" || input.LoginStatus == "expired"
		occupied := input.PlatformAccountID != "" && input.PlatformAccountID != r.PlatformAccountID
		if !known || occupied || (input.LoginStatus == "normal" && input.PlatformAccountID == "") {
			u.Status = "failed"
			u.Step = "completed"
			u.Message = "窗口当前账号不一致或无法确认，请检查后再更新"
			u.InputCookie = ""
			*lock = ""
			return nil
		}
		b, _ := json.Marshal(input.BeforeCookie)
		r.BeforeCookie = string(b)
		u.Fingerprint = input.Fingerprint
		u.Step = "prepared"
		u.Message = "正在更新 Cookie"
		canApply = true
		r.UpdatedAt = s.now()
		return nil
	})
	if e != nil {
		return CookieUpdateView{}, e
	}
	v := cookieView(out.CookieUpdate)
	v.CanApply = canApply
	return v, nil
}
func (s *accountService) SettleCookieUpdate(actor identityservice.PublicUser, id, op string, input CookieResultInput) (CookieUpdateView, error) {
	if len(input.Message) > 500 || len(input.PlatformAccountID) > 255 || len(input.Name) > 128 || len(input.AvatarURL) > 2048 {
		return CookieUpdateView{}, ErrInvalidInput
	}
	if input.Status != "applied" && input.Status != "failed" && input.Status != "uncertain" {
		return CookieUpdateView{}, ErrInvalidInput
	}
	out, e := s.cookieMutation(actor, id, func(r *model.AccountRecord, lock *string) error {
		u := r.CookieUpdate
		if u == nil || u.OperationID != op || u.BrowserProfileID != r.BrowserProfileID {
			return ErrCookieConflict
		}
		if u.Status == "succeeded" || u.Status == "failed" {
			return nil
		}
		if *lock != op {
			return ErrCookieConflict
		}
		if input.Status == "applied" {
			if u.Fingerprint == "" || r.BeforeCookie == "" || !input.Applied || !input.InputMatches || len(input.Cookies) == 0 || input.LoginStatus != "normal" || input.PlatformAccountID == "" || (r.PlatformAccountID != "" && r.PlatformAccountID != input.PlatformAccountID) {
				return ErrInvalidInput
			}

			raw, _ := json.Marshal(input.Cookies)
			r.ActiveCookie = string(raw)
			if r.OriginalCookie == "" {
				r.OriginalCookie = u.InputCookie
			}
			r.CookieStatus = "active"
			now := s.now()
			r.CookieUpdatedAt = &now
			r.PlatformAccountID = input.PlatformAccountID
			r.PlatformName = input.Name
			r.PlatformAvatarURL = input.AvatarURL
			r.IdentificationStatus = IdentificationIdentified
			r.PlatformLoginStatus = PlatformLoginNormal
			r.LastCheckStatus = "completed"
			r.LastCheckMessage = "Cookie 已更新并确认登录"
			r.LastCheckAt = &now
			r.LastCheckIdentity = &model.CheckIdentity{PlatformAccountID: input.PlatformAccountID, PlatformName: input.Name, PlatformAvatarURL: input.AvatarURL}
			r.CheckOperationID = ""
			u.Status = "succeeded"
			u.Step = "completed"
			u.Message = "Cookie 已更新并确认登录"
			u.InputCookie = ""
			*lock = ""
		} else if input.Status == "failed" && (!input.Applied || (input.Verified && ((input.LoginStatus == "normal" && input.PlatformAccountID != "") || input.LoginStatus == "not_logged_in" || input.LoginStatus == "expired"))) {
			u.Status = "failed"
			u.Step = "completed"
			u.Message = input.Message
			u.InputCookie = ""
			*lock = ""
		} else {
			u.Status = "uncertain"
			u.Step = "confirming"
			u.Message = input.Message
			if u.Message == "" {
				u.Message = "更新结果待确认，请核实当前登录状态"
			}
		}
		r.UpdatedAt = s.now()
		return nil
	})

	if e != nil {
		return CookieUpdateView{}, e
	}
	return cookieView(out.CookieUpdate), nil
}
func StartCookieUpdate(actor identityservice.PublicUser, id string, input CookieStartInput) (CookieUpdateView, error) {
	return defaultAccountService().StartCookieUpdate(actor, id, input)
}
func SnapshotCookieUpdate(actor identityservice.PublicUser, id, op string, input CookieSnapshotInput) (CookieUpdateView, error) {
	return defaultAccountService().SnapshotCookieUpdate(actor, id, op, input)
}
func SettleCookieUpdate(actor identityservice.PublicUser, id, op string, input CookieResultInput) (CookieUpdateView, error) {
	return defaultAccountService().SettleCookieUpdate(actor, id, op, input)
}
func GetCookieUpdate(actor identityservice.PublicUser, id, op string) (CookieUpdateView, error) {
	r, e := defaultAccountService().cookieAccess(actor, id)
	if e != nil {
		return CookieUpdateView{}, e
	}
	if r.CookieUpdate == nil || r.CookieUpdate.OperationID != op {
		return CookieUpdateView{}, ErrCookieConflict
	}
	return cookieView(r.CookieUpdate), nil
}
func CurrentCookieUpdate(r AccountRecord) CookieUpdateView {
	v := cookieView(r.CookieUpdate)
	v.InputCookie = ""
	v.Fingerprint = ""
	return v
}
