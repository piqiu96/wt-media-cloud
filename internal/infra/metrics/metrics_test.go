package metrics

import "testing"

func TestDefaultAndInitializedRecorderAreNoop(t *testing.T) {
	recorder := Get()
	recorder.Count("requests", 1, map[string]string{"route": "/health"})

	if err := Initialize(); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	t.Cleanup(func() {
		if err := Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	recorder = Get()
	if _, ok := recorder.(Noop); !ok {
		t.Fatalf("Get() = %#v, want Noop", recorder)
	}
	recorder.Count("requests", -1, map[string]string{"route": "/health"})
}

func TestCloseRestoresNoopDefault(t *testing.T) {
	if err := Initialize(); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if err := Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, ok := Get().(Noop); !ok {
		t.Fatalf("Get() after Close() = %#v, want Noop", Get())
	}
}
