package storage

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

const testDigest = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

func TestSourceKeyIsContentAddressed(t *testing.T) {
	key, err := SourceKey(42, testDigest, "mp4")
	if err != nil {
		t.Fatalf("SourceKey() error = %v", err)
	}
	if want := "materials/42/" + testDigest + ".mp4"; key != want {
		t.Fatalf("SourceKey() = %q, want %q", key, want)
	}
}

// Two runs that produced the same bytes have to land on the same key: that is
// what makes re-preparing a material idempotent instead of a second object that
// an earlier task's recorded key no longer points at. The control is the third
// arm — a different digest must produce a different key, or the equality above
// would be satisfied by a function that ignores its arguments.
func TestSourceKeyDependsOnTheDigestAndNothingElse(t *testing.T) {
	first, err := SourceKey(42, testDigest, "mp4")
	if err != nil {
		t.Fatal(err)
	}
	same, err := SourceKey(42, testDigest, "mp4")
	if err != nil {
		t.Fatal(err)
	}
	other, err := SourceKey(42, strings.Repeat("a", 64), "mp4")
	if err != nil {
		t.Fatal(err)
	}
	if first != same {
		t.Fatalf("the same digest produced %q and %q", first, same)
	}
	if first == other {
		t.Fatalf("a different digest produced the same key %q", first)
	}
}

func TestStagingKeySeparatesAttempts(t *testing.T) {
	first, err := StagingKey(42, "task-1", "mp4")
	if err != nil {
		t.Fatalf("StagingKey() error = %v", err)
	}
	second, err := StagingKey(42, "task-2", "mp4")
	if err != nil {
		t.Fatal(err)
	}
	if want := "materials/42/.staging/task-1.mp4"; first != want {
		t.Fatalf("StagingKey() = %q, want %q", first, want)
	}
	if first == second {
		t.Fatal("two tasks staged to the same key: they would interleave their bytes")
	}
	// The staging key has to sit under the material's own prefix, because the
	// promotion from staging to source is a server-side copy and a copy cannot
	// cross a bucket or a prefix the caller was not granted.
	if !strings.HasPrefix(first, "materials/42/") {
		t.Fatalf("staging key %q is outside the material's prefix", first)
	}
	if first == strings.TrimSuffix(first, "/.staging/task-1.mp4") {
		t.Fatalf("staging key %q is not in a staging segment", first)
	}
}

// The extension reaches a key, so a value that carries a separator would move the
// object somewhere the caller did not ask for. Each arm is a shape a caller could
// plausibly pass: an empty extension, one with a slash, one with an inner dot, one
// with a trailing space, and one long enough to be a sentence rather than a media
// type. They are refused by two rules between them — the character check for the
// separators and the space, the length check for the last — and not by one rule
// per arm; see the note on `validateExtension` for the guard that was deleted
// after a mutation showed it overlapped the character check exactly.
func TestKeyBuildersRefuseAnExtensionThatCouldMoveTheObject(t *testing.T) {
	for _, extension := range []string{"", "mp/4", "m.p4", "mp4 ", "verylongextension"} {
		if _, err := SourceKey(42, testDigest, extension); err == nil {
			t.Errorf("SourceKey accepted extension %q", extension)
		}
		if _, err := StagingKey(42, "task-1", extension); err == nil {
			t.Errorf("StagingKey accepted extension %q", extension)
		}
	}
	// A leading dot is a spelling of the same extension, not a different one, so
	// it is normalised rather than refused.
	key, err := SourceKey(42, testDigest, ".MP4")
	if err != nil {
		t.Fatalf("SourceKey(.MP4) error = %v", err)
	}
	if want := "materials/42/" + testDigest + ".mp4"; key != want {
		t.Fatalf("SourceKey(.MP4) = %q, want %q", key, want)
	}
}

func TestKeyBuildersRefuseAMalformedDigestOrIdentity(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		digest   string
		material int64
	}{
		{"short digest", strings.Repeat("a", 63), 42},
		{"long digest", strings.Repeat("a", 65), 42},
		{"non-hex digest", strings.Repeat("z", 64), 42},
		{"empty digest", "", 42},
		{"zero material", testDigest, 0},
		{"negative material", testDigest, -1},
	} {
		if _, err := SourceKey(testCase.material, testCase.digest, "mp4"); err == nil {
			t.Errorf("%s: SourceKey accepted it", testCase.name)
		}
	}
	// The digest is case-insensitive because the same bytes can be rendered
	// either way; normalising here keeps two spellings from naming two objects.
	upper, err := SourceKey(42, strings.ToUpper(testDigest), "mp4")
	if err != nil {
		t.Fatalf("SourceKey(uppercase) error = %v", err)
	}
	lower, err := SourceKey(42, testDigest, "mp4")
	if err != nil {
		t.Fatal(err)
	}
	if upper != lower {
		t.Fatalf("case changed the key: %q vs %q", upper, lower)
	}
}

// A task id reaches a key too, and `..` is the one value that survives the
// character check while still naming a different directory.
func TestStagingKeyRefusesATaskIDThatCouldEscapeThePrefix(t *testing.T) {
	for _, taskID := range []string{"", "..", "a/b", `a\b`, "task 1", strings.Repeat("t", 65)} {
		if _, err := StagingKey(42, taskID, "mp4"); err == nil {
			t.Errorf("StagingKey accepted task id %q", taskID)
		}
	}
}

func TestValidateKeyRefusesWhatCouldLeaveThePrefix(t *testing.T) {
	for _, key := range []string{"", "/absolute", "..", "a/../b", "a//b", `a\b`} {
		if err := validateKey(key); err == nil {
			t.Errorf("validateKey accepted %q", key)
		}
	}
	for _, key := range []string{"materials/42/x.mp4", "materials/42/.staging/t.mp4"} {
		if err := validateKey(key); err != nil {
			t.Errorf("validateKey rejected %q: %v", key, err)
		}
	}
}

func TestNormalisePrefixAlwaysEndsAtASegmentBoundary(t *testing.T) {
	for input, want := range map[string]string{
		"":          "",
		"   ":       "",
		"/":         "",
		"dev":       "dev/",
		"dev/":      "dev/",
		"/dev/":     "dev/",
		" dev/v2/ ": "dev/v2/",
	} {
		if got := normalisePrefix(input); got != want {
			t.Errorf("normalisePrefix(%q) = %q, want %q", input, got, want)
		}
	}
}

// Every method of the unconfigured store has to fail, and with the same error:
// a store that answered one call by doing nothing would turn a missing secret
// into a silent no-op somewhere much further from the cause.
func TestTheUnconfiguredStoreRefusesEveryCall(t *testing.T) {
	var store Store = notConfiguredStore{}
	ctx := context.Background()
	calls := map[string]func() error{
		"PresignGet": func() error { _, err := store.PresignGet(ctx, "materials/42/x.mp4", time.Minute); return err },
		"Stat":       func() error { _, err := store.Stat(ctx, "materials/42/x.mp4"); return err },
		"Put":        func() error { return store.Put(ctx, "materials/42/x.mp4", strings.NewReader(""), 0, "video/mp4") },
		"Copy":       func() error { return store.Copy(ctx, "a", "b") },
		"Remove":     func() error { return store.Remove(ctx, "materials/42/x.mp4") },
	}
	if len(calls) != 5 {
		t.Fatalf("the interface grew to %d methods; this check covers 5", len(calls))
	}
	for name, call := range calls {
		if err := call(); !errors.Is(err, ErrNotConfigured) {
			t.Errorf("%s error = %v, want ErrNotConfigured", name, err)
		}
	}
}
