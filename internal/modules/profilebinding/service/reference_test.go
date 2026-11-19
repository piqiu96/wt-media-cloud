package service

import (
	"errors"
	"testing"
)

func TestProfileReferenceOnlyBlocksDeletion(t *testing.T) {
	store := newMemoryStore()
	bindMain(store, 2, "main-user-2")
	store.profiles["profile"] = BrowserProfile{ID: "profile", UserID: 2, BitMainUserID: "main-user-2", Enabled: true, LocalState: ProfileLocalMissing}
	calls := 0
	svc := newTestService(store)
	svc.referenceCheck = func(id string) (bool, error) { calls++; return true, nil }
	if err := svc.DeleteProfile(profileActor("user-2"), "profile"); !errors.Is(err, ErrProfileReferenced) {
		t.Fatalf("delete referenced: %v", err)
	}
	if _, exists := store.profiles["profile"]; !exists {
		t.Fatal("referenced Profile deleted")
	}
	enabled := false
	if _, err := svc.UpdateProfile(profileActor("user-2"), "profile", nil, &enabled); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("ordinary editing queried dependencies: %d", calls)
	}
	svc.referenceCheck = func(string) (bool, error) { return false, errors.New("dependency unavailable") }
	if err := svc.DeleteProfile(profileActor("user-2"), "profile"); err == nil {
		t.Fatal("dependency error allowed deletion")
	}
}
