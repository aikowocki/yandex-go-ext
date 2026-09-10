package logging

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/shared/messaging"
)

func TestMessagingLoggingHelpers(t *testing.T) {
	backend := &recordingLogger{}
	ctx := WithLogger(context.Background(), backend)
	LogMessagingPublish(ctx, "", "", "message-1", "success", time.Millisecond, nil)
	LogMessagingConsume(ctx, "kafka", "avatars", "", "error", time.Millisecond, errors.New("failed"))
	if backend.calls != 2 {
		t.Fatalf("logging calls = %d, want 2", backend.calls)
	}
	if got := messaging.SafeValue(""); got != "unknown" {
		t.Fatalf("safeMessagingValue empty = %q", got)
	}
	if got := messaging.SafeValue("kafka"); got != "kafka" {
		t.Fatalf("safeMessagingValue value = %q", got)
	}
	if got := messagingAttrs("", "", "", "", 0); len(got) != 4 {
		t.Fatalf("messagingAttrs without message ID len = %d", len(got))
	}
}
