package service

import (
	"errors"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/model"
	"strings"
	"testing"
)

type cookieMemory struct {
	*memoryStore
	lock string
}

func (m *cookieMemory) MutateCookie(id string, userID identityservice.UserID, fn func(*model.AccountRecord, *string) error) (model.AccountRecord, error) {
	r := m.records[id]
	if r.CookieUpdate != nil {
		u := *r.CookieUpdate
		r.CookieUpdate = &u
	}
	lock := m.lock
	if e := fn(&r, &lock); e != nil {
		return r, e
	}
	m.records[id] = r
	m.lock = lock
	return r, nil
}
func cookieFixture(t *testing.T) (*accountService, *cookieMemory, identityservice.PublicUser) {
	t.Helper()
	actor := mediaActor(1, 10, identityservice.RoleOperator)
	m := &cookieMemory{memoryStore: newMemoryStore()}
	m.records["1"] = AccountRecord{Account: Account{ID: "1", UserID: 1, IdentificationStatus: IdentificationIdentified, Platform: PlatformBaidu, PlatformAccountID: "uid", BrowserProfileID: "p", Enabled: true, LastCheckStatus: "completed", PlatformLoginStatus: PlatformLoginNormal}, ActiveCookie: "old-confirmed"}
	facts := &fakeProfileFacts{profiles: map[string]fakeProfileFact{"p": {id: "p", userID: 1, bitProfileID: "bit", active: true}}}
	s := newAccountService(m, withProfileFactResolver(facts))
	return s, m, actor
}
func TestCookieLifecycleProtectsInputAndSettlesOnce(t *testing.T) {
	s, m, a := cookieFixture(t)
	in := CookieStartInput{RequestKey: "key", InputCookie: "BDUSS=new"}
	start, e := s.StartCookieUpdate(a, "1", in)
	if e != nil || !start.AllowExecute {
		t.Fatal(start, e)
	}
	duplicate, e := s.StartCookieUpdate(a, "1", in)
	if e != nil || duplicate.AllowExecute || duplicate.OperationID != start.OperationID {
		t.Fatal(duplicate, e)
	}
	changed := in
	changed.InputCookie = "different"
	if _, e = s.StartCookieUpdate(a, "1", changed); !errors.Is(e, ErrCookieConflict) {
		t.Fatal(e)
	}
	before := []map[string]any{{"name": "BDUSS", "value": "old", "domain": ".baidu.com", "path": "/", "httpOnly": true}}
	snap := CookieSnapshotInput{BeforeCookie: before, Fingerprint: strings.Repeat("f", 64), LoginStatus: "normal", PlatformAccountID: "uid"}
	prepared, e := s.SnapshotCookieUpdate(a, "1", start.OperationID, snap)
	if e != nil || !prepared.CanApply || m.records["1"].BeforeCookie == "" {
		t.Fatal(prepared, e)
	}
	repeated, e := s.SnapshotCookieUpdate(a, "1", start.OperationID, snap)
	if e != nil || repeated.CanApply {
		t.Fatal(repeated, e)
	}
	uncertain, e := s.SettleCookieUpdate(a, "1", start.OperationID, CookieResultInput{Status: "uncertain", Applied: true})
	if e != nil || uncertain.Status != "uncertain" || m.lock == "" || m.records["1"].ActiveCookie != "old-confirmed" {
		t.Fatal(uncertain, e)
	}
	if _, e = s.StartCookieUpdate(a, "1", CookieStartInput{RequestKey: "next", ExpectedOperationID: start.OperationID, InputCookie: "new"}); !errors.Is(e, ErrCookieConflict) {
		t.Fatal(e)
	}
	result := CookieResultInput{Status: "applied", Applied: true, InputMatches: true, LoginStatus: "normal", PlatformAccountID: "uid", Name: "real", Cookies: []map[string]any{{"name": "BDUSS", "value": "new", "domain": ".baidu.com", "path": "/"}}}
	done, e := s.SettleCookieUpdate(a, "1", start.OperationID, result)
	if e != nil || done.Status != "succeeded" || m.lock != "" {
		t.Fatal(done, e)
	}
	r := m.records["1"]
	if r.OriginalCookie != in.InputCookie || r.ActiveCookie == "old-confirmed" || r.CookieUpdate.InputCookie != "" {
		t.Fatal("credentials not transferred")
	}
	result.Name = "late"
	_, e = s.SettleCookieUpdate(a, "1", start.OperationID, result)
	if e != nil || m.records["1"].PlatformName != "real" {
		t.Fatal("repeat changed result", e)
	}
	next, e := s.StartCookieUpdate(a, "1", CookieStartInput{RequestKey: "next", ExpectedOperationID: start.OperationID, InputCookie: "BDUSS=second"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.SettleCookieUpdate(a, "1", start.OperationID, result); !errors.Is(e, ErrCookieConflict) {
		t.Fatal(e)
	}
	_, e = s.SettleCookieUpdate(a, "1", next.OperationID, CookieResultInput{Status: "failed", Message: "read unavailable"})
	if e != nil || m.records["1"].OriginalCookie != in.InputCookie || m.records["1"].ActiveCookie != r.ActiveCookie {
		t.Fatal(e)
	}
}
func TestCookieSnapshotRefusesActualOccupantAndUnknown(t *testing.T) {
	for _, status := range []string{"normal", "environment_error"} {
		t.Run(status, func(t *testing.T) {
			s, m, a := cookieFixture(t)
			u, e := s.StartCookieUpdate(a, "1", CookieStartInput{RequestKey: "key", InputCookie: "BDUSS=new"})
			if e != nil {
				t.Fatal(e)
			}
			v, e := s.SnapshotCookieUpdate(a, "1", u.OperationID, CookieSnapshotInput{BeforeCookie: []map[string]any{}, Fingerprint: strings.Repeat("f", 64), LoginStatus: status, PlatformAccountID: "other"})
			if e != nil || v.CanApply || v.Status != "failed" || m.lock != "" || m.records["1"].BeforeCookie != "" {
				t.Fatal(v, e)
			}
		})
	}
}

func TestCookieSameUIDInOtherWindowCompletesWithoutTakingItOver(t *testing.T) {
	s, m, a := cookieFixture(t)
	r := m.records["1"]
	r.PlatformAccountID = ""
	m.records["1"] = r
	existing := r
	existing.ID = "2"
	existing.PlatformAccountID = "existing"
	existing.BrowserProfileID = "other"
	existing.ActiveCookie = "do-not-touch"
	m.records["2"] = existing
	u, e := s.StartCookieUpdate(a, "1", CookieStartInput{RequestKey: "key", InputCookie: "BDUSS=new"})
	if e != nil {
		t.Fatal(e)
	}
	_, e = s.SnapshotCookieUpdate(a, "1", u.OperationID, CookieSnapshotInput{BeforeCookie: []map[string]any{}, Fingerprint: strings.Repeat("f", 64), LoginStatus: "not_logged_in"})
	if e != nil {
		t.Fatal(e)
	}
	v, e := s.SettleCookieUpdate(a, "1", u.OperationID, CookieResultInput{Status: "applied", Applied: true, InputMatches: true, LoginStatus: "normal", PlatformAccountID: "existing", Cookies: []map[string]any{{"name": "BDUSS", "value": "new"}}})
	if e != nil || v.Status != "succeeded" || m.lock != "" || m.records["2"].ActiveCookie != "do-not-touch" || m.records["1"].PlatformAccountID != "existing" || m.records["1"].ActiveCookie == "" {
		t.Fatal(v, e)
	}
}

func TestCookieStartInvalidatesAnEarlierReadOnlyCheck(t *testing.T) {
	s, m, a := cookieFixture(t)
	r := m.records["1"]
	r.CheckOperationID = "old-check"
	r.CheckProfileID = "p"
	r.LastCheckStatus = "running"
	m.records["1"] = r
	if _, e := s.StartCookieUpdate(a, "1", CookieStartInput{RequestKey: "new", InputCookie: "BDUSS=placeholder"}); e != nil {
		t.Fatal(e)
	}
	if _, e := s.ApplyLocalAccountCheckResult(a, "1", AccountCheckResultInput{OperationID: "old-check", ExecutionStatus: "completed", PlatformAccountID: "uid", PlatformLoginStatus: PlatformLoginNormal}); !errors.Is(e, ErrStaleCheck) {
		t.Fatal("old check overwrote Cookie operation", e)
	}
}
