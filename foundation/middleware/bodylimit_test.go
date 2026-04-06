package middleware_test

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"aicompass/foundation/middleware"
	"strings"
	"testing"
)

func TestBodyLimit_WithinLimit(t *testing.T) {
	t.Parallel()

	body := strings.NewReader(`{"msg":"hello"}`)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("unexpected read error: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	})

	handler := middleware.BodyLimit(1024)(inner) // 1 KB limit

	req := httptest.NewRequest(http.MethodPost, "/", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != `{"msg":"hello"}` {
		t.Errorf("body = %q, want %q", rec.Body.String(), `{"msg":"hello"}`)
	}
}

func TestBodyLimit_ExceedsLimit(t *testing.T) {
	t.Parallel()

	// Create a body larger than 64 bytes
	bigBody := strings.NewReader(strings.Repeat("x", 128))

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			// MaxBytesReader returns an error when limit is exceeded.
			http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.BodyLimit(64)(inner) // 64 byte limit

	req := httptest.NewRequest(http.MethodPost, "/", bigBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestBodyLimit_DefaultLimit(t *testing.T) {
	t.Parallel()

	// Pass 0 — should use 1MB default. A small body should pass fine.
	body := strings.NewReader("small payload")

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.BodyLimit(0)(inner) // 0 means default (1MB)

	req := httptest.NewRequest(http.MethodPost, "/", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestBodyLimit_GETRequest(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.BodyLimit(64)(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestBodyLimit_MultipartFormData_SkipsLimit(t *testing.T) {
	t.Parallel()

	// Create a multipart body larger than 64 bytes (our limit).
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", "test.pdf")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	// Write 256 bytes of data — well above our 64-byte limit.
	part.Write(bytes.Repeat([]byte("x"), 256))
	writer.Close()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.BodyLimit(64)(inner) // 64-byte limit — should be skipped for multipart

	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; multipart request should not be limited", rec.Code, http.StatusOK)
	}
}
