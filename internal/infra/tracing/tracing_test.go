package tracing

import (
	"context"
	"testing"
)

func TestDefaultAndInitializedTracerAreNoop(t *testing.T) {
	tracer := Get()
	_, finish := tracer.Start(context.Background(), "operation")
	finish(nil)

	if err := Initialize(); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	t.Cleanup(func() {
		if err := Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	tracer = Get()
	if _, ok := tracer.(Noop); !ok {
		t.Fatalf("Get() = %#v, want Noop", tracer)
	}
	ctx, finish := tracer.Start(context.Background(), "operation")
	if ctx == nil {
		t.Fatal("Noop Start returned nil context")
	}
	finish(nil)
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
