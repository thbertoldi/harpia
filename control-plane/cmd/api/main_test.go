package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResponseWriterPreservesFlusher(t *testing.T) {
	recorder := httptest.NewRecorder()
	wrapped := &responseWriter{ResponseWriter: recorder, statusCode: http.StatusOK}

	flusher, ok := any(wrapped).(http.Flusher)
	if !ok {
		t.Fatal("responseWriter must implement http.Flusher for Connect server streams")
	}

	flusher.Flush()

	if !recorder.Flushed {
		t.Fatal("responseWriter.Flush did not delegate to the wrapped ResponseWriter")
	}
}
