package routes

import (
	"testing"

	"go.uber.org/zap"
)

func TestRouteSetup_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("RouteSetup panicked: %v", r)
		}
	}()

	logger := zap.NewNop()
	r := RouteSetup(nil, logger)
	if r == nil {
		t.Fatal("expected non-nil router")
	}
}
