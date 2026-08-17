package testsupport

import (
	"testing"

	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
)

func TestContextAndAssertions(t *testing.T) {
	ctx := Context(t)

	logging.Info(ctx, "user.created", "user_id", "user-1")
	logging.Warn(ctx, "user.warning")

	Assert(t).
		Contains("user.created").
		CountMessage("user.created", 1).
		HasField("user_id", "user-1").
		HasLevel(logging.InfoLevel).
		NotContains("panic").
		NoErrors()

	if !Contains(t, "user.created") {
		t.Fatal("package-level assertion did not find user.created")
	}
	if !HasField(t, "user_id", "user-1") {
		t.Fatal("package-level field assertion did not find user_id")
	}
}

func TestContextIsParallelSafe(t *testing.T) {
	t.Parallel()

	ctx := Context(t)
	logging.Info(ctx, "parallel.event")

	Assert(t).Contains("parallel.event")
}
