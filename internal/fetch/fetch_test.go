package fetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGetRetriesServerError(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls++
		if calls == 1 {
			http.Error(writer, "temporary", http.StatusInternalServerError)
			return
		}
		_, _ = writer.Write([]byte("ok"))
	}))
	defer server.Close()

	client := &Client{
		HTTPClient: server.Client(),
		MaxBytes:   1024,
		Retries:    2,
		Backoff:    time.Millisecond,
	}
	data, err := client.Get(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Get returned an error: %v", err)
	}
	if string(data) != "ok" {
		t.Fatalf("got %q, want ok", data)
	}
	if calls != 2 {
		t.Fatalf("got %d calls, want 2", calls)
	}
}

func TestGetRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(strings.Repeat("x", 128)))
	}))
	defer server.Close()

	client := &Client{
		HTTPClient: server.Client(),
		MaxBytes:   64,
		Retries:    1,
		Backoff:    time.Millisecond,
	}
	if _, err := client.Get(context.Background(), server.URL); err == nil {
		t.Fatal("Get accepted an oversized response")
	}
}
