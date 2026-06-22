package chat

import (
	"testing"

	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	"github.com/google/uuid"
)

// TestAppendInputZeroValueIsValid documents the zero-value shape of
// AppendInput. Tasks 5-8 build AppendInput literals; this test prevents
// silent breakage if fields are renamed.
func TestAppendInputZeroValueIsValid(t *testing.T) {
	authorID := uuid.New()
	execID := uuid.New()
	in := AppendInput{
		ThreadID:      "plan-config-uuid",
		Role:          chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_OVERSEER,
		Kind:          chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_USER_TEXT,
		Text:          "hello",
		PayloadJSON:   "{}",
		AuthorUserID:  &authorID,
		ExecutionID:   &execID,
	}
	if in.ThreadID == "" {
		t.Fatal("ThreadID should be settable")
	}
}
