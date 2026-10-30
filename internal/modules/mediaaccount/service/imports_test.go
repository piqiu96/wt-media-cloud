package service

import (
	"encoding/json"
	"errors"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/model"
	"maps"
	"strconv"
	"strings"
	"testing"
)

type importMemory struct {
	*memoryStore
	batches    map[string]model.ImportBatch
	locks      map[string]string
	seq        int
	failCreate bool
}

func (m *importMemory) CreateImport(b model.ImportBatch) (model.ImportBatch, error) {
	for _, v := range m.batches {
		if v.UserID == b.UserID && v.RequestKey == b.RequestKey {
			return v, nil
		}
	}
	m.seq++
	b.ID = strconv.Itoa(m.seq)
	m.batches[b.ID] = b
	return b, nil
}
func (m *importMemory) GetImport(id string, u identityservice.UserID) (model.ImportBatch, error) {
	b, ok := m.batches[id]
	if !ok || b.UserID != u {
		return b, ErrNotFound
	}
	return b, nil
}
func (m *importMemory) ListImports(u identityservice.UserID) ([]model.ImportBatch, error) {
	out := []model.ImportBatch{}
	for _, b := range m.batches {
		if b.UserID == u {
			out = append(out, b)
		}
	}
	return out, nil
}
func (m *importMemory) MutateImport(id string, u identityservice.UserID, fn func(*model.ImportBatch, model.ImportResources) error) (model.ImportBatch, error) {
	b, e := m.GetImport(id, u)
	if e != nil {
		return b, e
	}
	raw, _ := json.Marshal(b)
	b = model.ImportBatch{}
	json.Unmarshal(raw, &b)
	locks := maps.Clone(m.locks)
	records := maps.Clone(m.records)
	if e = fn(&b, m); e != nil {
		m.locks = locks
		m.records = records
		return b, e
	}
	next, _ := json.Marshal(b)
	if string(raw) != string(next) {
		b.Revision++
	}
	m.batches[id] = b
	return b, nil
}
func (m *importMemory) ReserveProfile(id, op string, p Platform, marker string) (model.ImportTarget, error) {
	if m.locks[id] != "" && m.locks[id] != op {
		return model.ImportTarget{}, ErrImportConflict
	}
	m.locks[id] = op
	return model.ImportTarget{BitID: "bit-" + id, Name: "Test"}, nil
}
func (m *importMemory) ReleaseProfile(id, op string) error {
	if m.locks[id] == op {
		delete(m.locks, id)
	}
	return nil
}
func (m *importMemory) FindIdentity(p Platform, uid string) (string, error) {
	for _, r := range m.records {
		if r.Platform == p && r.PlatformAccountID == uid {
			return r.ID, nil
		}
	}
	return "", nil
}
func (m *importMemory) CreateAccount(a model.AccountRecord) (string, error) {
	if m.failCreate {
		return "", errors.New("write failed")
	}
	return m.memoryStore.Create(a)
}
func importFixture(t *testing.T) (*accountService, *importMemory, identityservice.PublicUser, ImportView) {
	t.Helper()
	m := &importMemory{memoryStore: newMemoryStore(), batches: map[string]model.ImportBatch{}, locks: map[string]string{}}
	s := newAccountService(m)
	a := mediaActor(1, 10, identityservice.RoleOperator)
	b, e := s.CreateImport(a, ImportInput{RequestKey: "key", Platform: PlatformBaidu, GroupID: "g", Items: []ImportRow{{1, "BDUSS=a"}, {3, "BDUSS=b"}}})
	if e != nil {
		t.Fatal(e)
	}
	return s, m, a, b
}
func prepImport(t *testing.T, s *accountService, a identityservice.PublicUser, b ImportView, row int) ImportView {
	t.Helper()
	v, e := s.MutateImportItem(a, b.ID, row, "start", ImportMutation{ExpectedRevision: b.Revision})
	if e != nil || !v.AllowExecute {
		t.Fatal(v, e)
	}
	attempt := v.Execution.Attempt
	v, e = s.MutateImportItem(a, b.ID, row, "result", ImportMutation{ExpectedRevision: v.Revision, Attempt: attempt, Step: "created", BrowserProfileID: strconv.Itoa(row)})
	if e != nil || !v.AllowExecute {
		t.Fatal(v, e)
	}
	v, e = s.MutateImportItem(a, b.ID, row, "result", ImportMutation{ExpectedRevision: v.Revision, Attempt: attempt, Step: "prepared", BeforeCookie: []map[string]any{}, Fingerprint: strings.Repeat("f", 64), CookieResultInput: CookieResultInput{LoginStatus: "not_logged_in"}})
	if e != nil || !v.CanApply {
		t.Fatal(v, e)
	}
	return v
}
func successImportInput(v ImportView) ImportMutation {
	return ImportMutation{ExpectedRevision: v.Revision, Attempt: v.Execution.Attempt, Step: "completed", CookieResultInput: CookieResultInput{Status: "applied", Applied: true, InputMatches: true, LoginStatus: "normal", PlatformAccountID: "real", Name: "Real", Cookies: []map[string]any{{"name": "BDUSS", "value": "readback", "domain": ".baidu.com"}}}}
}
func TestImportCookieRejectsEmptyHeaderPairs(t *testing.T) {
	if validateImportCookie(PlatformBaidu, "; ;") == nil {
		t.Fatal("empty header was accepted")
	}
}
func TestImportPreviewAndCreateIdempotency(t *testing.T) {
	s, m, a, b := importFixture(t)
	preview, e := s.PreviewImport(a, ImportInput{Platform: PlatformBaidu, Items: []ImportRow{{2, "bad"}, {4, "BDUSS=valid"}}})
	if e != nil || preview[0].Valid || !preview[1].Valid || len(m.batches) != 1 {
		t.Fatal(preview, e)
	}
	in := ImportInput{RequestKey: "key", Platform: PlatformBaidu, GroupID: "g", Items: []ImportRow{{1, "BDUSS=a"}, {3, "BDUSS=b"}}}
	again, e := s.CreateImport(a, in)
	if e != nil || again.ID != b.ID {
		t.Fatal(e)
	}
	in.Items[0].InputCookie = "BDUSS=changed"
	if _, e = s.CreateImport(a, in); !errors.Is(e, ErrImportConflict) {
		t.Fatal(e)
	}
	public, _ := json.Marshal(b)
	if strings.Contains(string(public), "BDUSS=") || strings.Contains(string(public), "request_digest") {
		t.Fatal("secret in projection")
	}
	if _, e = s.GetImport(mediaActor(2, 10, identityservice.RoleOperator), b.ID); !errors.Is(e, ErrNotFound) {
		t.Fatal(e)
	}
}
func TestImportStableRowsSerialGrantAndOldRevision(t *testing.T) {
	s, _, a, b := importFixture(t)
	v, e := s.MutateImportItem(a, b.ID, 3, "start", ImportMutation{ExpectedRevision: b.Revision})
	if e != nil || v.Execution.OperationID != "import:1:3:1" {
		t.Fatal(v, e)
	}
	repeat, e := s.MutateImportItem(a, b.ID, 3, "start", ImportMutation{ExpectedRevision: b.Revision})
	if e != nil || repeat.AllowExecute {
		t.Fatal(repeat, e)
	}
	if _, e = s.MutateImportItem(a, b.ID, 1, "start", ImportMutation{ExpectedRevision: v.Revision}); !errors.Is(e, ErrImportConflict) {
		t.Fatal("parallel grant", e)
	}
	if _, e = s.MutateImportItem(a, b.ID, 1, "actions", ImportMutation{ExpectedRevision: b.Revision, Action: "skip"}); !errors.Is(e, ErrImportConflict) {
		t.Fatal("stale json", e)
	}
	if _, e = s.MutateImportItem(a, b.ID, 2, "start", ImportMutation{ExpectedRevision: v.Revision}); !errors.Is(e, ErrNotFound) {
		t.Fatal("array index used as row", e)
	}
}
func TestImportSuccessTransferAndDuplicateDoesNotOverwrite(t *testing.T) {
	s, m, a, b := importFixture(t)
	v := prepImport(t, s, a, b, 1)
	in := successImportInput(v)
	done, e := s.MutateImportItem(a, b.ID, 1, "result", in)
	if e != nil || done.Counts["succeeded"] != 1 || m.locks["1"] != "" {
		t.Fatal(done, e)
	}
	r := m.records[done.Content.Items[0].AccountID]
	if r.OriginalCookie != "BDUSS=a" || !strings.Contains(r.ActiveCookie, "readback") || r.BeforeCookie != "[]" || r.CookieUpdate.Status != "succeeded" {
		t.Fatal("transfer failed")
	}
	if m.batches[b.ID].Content.Items[0].InputCookie != "" {
		t.Fatal("batch duplicate credentials")
	}
	again, e := s.MutateImportItem(a, b.ID, 1, "result", in)
	if e != nil || again.Revision != done.Revision || len(m.records) != 1 {
		t.Fatal(again, e)
	}
	v = prepImport(t, s, a, done, 3)
	second, e := s.MutateImportItem(a, b.ID, 3, "result", successImportInput(v))
	if e != nil || second.Counts["succeeded"] != 2 || second.Content.Items[1].SameIdentityAccountID != r.ID || second.Content.Items[1].PlatformAccountID != "real" || second.Content.Items[1].Name != "Real" || len(m.records) != 2 {
		t.Fatal(second, e)
	}
}
func TestImportUnknownInspectOnlyAndManualContinue(t *testing.T) {
	s, m, a, b := importFixture(t)
	v := prepImport(t, s, a, b, 1)
	unknown, e := s.MutateImportItem(a, b.ID, 1, "result", ImportMutation{ExpectedRevision: v.Revision, Attempt: 1, Step: "uncertain"})
	if e != nil || unknown.Counts["uncertain"] != 1 || m.locks["1"] == "" {
		t.Fatal(unknown, e)
	}
	for _, action := range []string{"skip", "retry"} {
		if _, e = s.MutateImportItem(a, b.ID, 1, "actions", ImportMutation{ExpectedRevision: unknown.Revision, Action: action}); !errors.Is(e, ErrImportConflict) {
			t.Fatal(action, e)
		}
	}
	inspected, e := s.MutateImportItem(a, b.ID, 1, "actions", ImportMutation{ExpectedRevision: unknown.Revision, Action: "reconcile"})
	if e != nil || !inspected.CanInspect || inspected.AllowExecute || inspected.CanApply || inspected.Execution.Attempt != 1 {
		t.Fatal(inspected, e)
	}
	manual, e := s.MutateImportItem(a, b.ID, 1, "result", ImportMutation{ExpectedRevision: inspected.Revision, Attempt: 1, Step: "completed", CookieResultInput: CookieResultInput{Status: "uncertain", Applied: true, LoginStatus: "verification_needed"}})
	if e != nil || manual.Content.Items[0].Step != "manual" {
		t.Fatal(manual, e)
	}
	resume, e := s.MutateImportItem(a, b.ID, 1, "actions", ImportMutation{ExpectedRevision: manual.Revision, Action: "reconcile"})
	if e != nil {
		t.Fatal(e)
	}
	in := successImportInput(resume)
	in.InputMatches = false
	in.Verified = true
	done, e := s.MutateImportItem(a, b.ID, 1, "result", in)
	if e != nil || done.Counts["succeeded"] != 1 {
		t.Fatal(done, e)
	}
}
func TestImportSkipRestoreOriginalRowAndNoWindowCreate(t *testing.T) {
	s, m, a, b := importFixture(t)
	skipped, e := s.MutateImportItem(a, b.ID, 3, "actions", ImportMutation{ExpectedRevision: b.Revision, Action: "skip"})
	if e != nil || skipped.Counts["skipped"] != 1 || len(m.locks) > 0 {
		t.Fatal(e)
	}
	restored, e := s.MutateImportItem(a, b.ID, 3, "actions", ImportMutation{ExpectedRevision: skipped.Revision, Action: "restore"})
	if e != nil || restored.Content.Items[1].RowNumber != 3 || restored.Content.Items[1].Status != "pending" {
		t.Fatal(restored, e)
	}
}
func TestImportOccupiedSnapshotAndRollback(t *testing.T) {
	s, m, a, b := importFixture(t)
	v := prepImport(t, s, a, b, 1)
	m.failCreate = true
	if _, e := s.MutateImportItem(a, b.ID, 1, "result", successImportInput(v)); e == nil {
		t.Fatal("expected tx failure")
	}
	raw := m.batches[b.ID]
	if raw.Content.Items[0].Status != "running" || raw.Content.Items[0].InputCookie == "" || m.locks["1"] == "" {
		t.Fatal("failed tx changed progress")
	}
}

func TestImportRestorePreservesFailureContext(t *testing.T) {
	for _, tc := range []struct{ name, message, existing string }{
		{"specific failure", "读取登录信息超时，请稍后重试", ""},
		{"existing account", "此账号已存在，请查看已有账号", "42"},
		{"legacy skipped duplicate", "已跳过", "42"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, m, a, b := importFixture(t)
			stored := m.batches[b.ID]
			stored.Content.Items[0].Status = "failed"
			stored.Content.Items[0].Message = tc.message
			stored.Content.Items[0].SameIdentityAccountID = tc.existing
			m.batches[b.ID] = stored
			skipped, err := s.MutateImportItem(a, b.ID, 1, "actions", ImportMutation{ExpectedRevision: b.Revision, Action: "skip"})
			if err != nil {
				t.Fatal(err)
			}
			restored, err := s.MutateImportItem(a, b.ID, 1, "actions", ImportMutation{ExpectedRevision: skipped.Revision, Action: "restore"})
			if err != nil {
				t.Fatal(err)
			}
			item := restored.Content.Items[0]
			if item.Status != "failed" || item.Message == "已恢复" || item.Message == "已跳过" {
				t.Fatalf("lost actionable failure: %+v", item)
			}
			if tc.message != "已跳过" && item.Message != tc.message {
				t.Fatalf("lost original failure: %q", item.Message)
			}
			if item.SameIdentityAccountID != tc.existing || len(m.records) != 0 || len(m.locks) != 0 {
				t.Fatal("restore must not create or reserve resources")
			}
		})
	}
}

func TestImportSameUIDInDifferentWindowsCompletesAndContinues(t *testing.T) {
	s, m, a, b := importFixture(t)
	first := prepImport(t, s, a, b, 1)
	first, e := s.MutateImportItem(a, b.ID, 1, "result", successImportInput(first))
	if e != nil {
		t.Fatal(e)
	}
	firstID := first.Content.Items[0].AccountID
	original := m.records[firstID]
	second := prepImport(t, s, a, first, 3)
	input := successImportInput(second)
	second, e = s.MutateImportItem(a, b.ID, 3, "result", input)
	if e != nil || second.Counts["succeeded"] != 2 || second.Counts["failed"] != 0 {
		t.Fatalf("same UID should complete in both windows: counts=%+v error=%v", second.Counts, e)
	}
	secondID := second.Content.Items[1].AccountID
	if firstID == secondID || secondID == "" || m.records[secondID].BrowserProfileID == original.BrowserProfileID || m.records[secondID].PlatformAccountID != original.PlatformAccountID {
		t.Fatal("window records were merged or not identified")
	}
	if m.records[firstID].ActiveCookie != original.ActiveCookie || m.records[firstID].BrowserProfileID != original.BrowserProfileID {
		t.Fatal("other window changed")
	}
	repeated, e := s.MutateImportItem(a, b.ID, 3, "result", input)
	if e != nil || repeated.Counts["succeeded"] != 2 || len(m.records) != 2 {
		t.Fatalf("replayed result created another record: %v", e)
	}
}

func TestLegacySkippedDuplicateRestoresToInspectionInsteadOfReplay(t *testing.T) {
	s, m, a, b := importFixture(t)
	stored := m.batches[b.ID]
	stored.Content.Items[0].Status = "skipped"
	stored.Content.Items[0].PreviousStatus = "uncertain"
	stored.Content.Items[0].Step = "applying"
	stored.Content.Items[0].SameIdentityAccountID = "old-other-window"
	m.batches[b.ID] = stored
	restored, e := s.MutateImportItem(a, b.ID, 1, "actions", ImportMutation{ExpectedRevision: b.Revision, Action: "restore"})
	if e != nil || restored.Content.Items[0].Status != "uncertain" || restored.AllowExecute {
		t.Fatalf("restore must inspect, not replay: %v", e)
	}
	receipt, e := s.MutateImportItem(a, b.ID, 1, "start", ImportMutation{ExpectedRevision: restored.Revision})
	if e != nil || receipt.AllowExecute || receipt.Execution != nil || receipt.Content.Items[0].Status != "uncertain" || receipt.Content.Items[0].Attempt != restored.Content.Items[0].Attempt {
		t.Fatal("legacy unknown result must return a receipt without a new execution grant")
	}
}
