package h3

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewResponse(t *testing.T) {
	w := httptest.NewRecorder()
	rw := NewResponse(w)

	if rw == nil {
		t.Fatal("NewResponse returned nil")
	}

	if rw.Status() != http.StatusOK {
		t.Errorf("initial status = %d, want %d", rw.Status(), http.StatusOK)
	}

	if rw.Size() != 0 {
		t.Errorf("initial size = %d, want 0", rw.Size())
	}

	if rw.Committed() {
		t.Error("initial committed should be false")
	}
}

func TestNewResponseIdempotent(t *testing.T) {
	w := httptest.NewRecorder()
	rw1 := NewResponse(w)
	rw2 := NewResponse(rw1)

	if rw1 != rw2 {
		t.Error("NewResponse should return same instance when wrapping *Response")
	}
}

func TestResponseStatus(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"OK", http.StatusOK},
		{"Created", http.StatusCreated},
		{"BadRequest", http.StatusBadRequest},
		{"NotFound", http.StatusNotFound},
		{"InternalServerError", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			rw := NewResponse(w)

			rw.WriteHeader(tt.status)

			if rw.Status() != tt.status {
				t.Errorf("Status() = %d, want %d", rw.Status(), tt.status)
			}

			if w.Code != tt.status {
				t.Errorf("underlying recorder code = %d, want %d", w.Code, tt.status)
			}
		})
	}
}

func TestResponseWrite(t *testing.T) {
	w := httptest.NewRecorder()
	rw := NewResponse(w)

	data := []byte("hello world")
	n, err := rw.Write(data)

	if err != nil {
		t.Fatalf("Write error: %v", err)
	}

	if n != len(data) {
		t.Errorf("Write returned %d, want %d", n, len(data))
	}

	if rw.Size() != int64(len(data)) {
		t.Errorf("Size() = %d, want %d", rw.Size(), len(data))
	}

	if !rw.Committed() {
		t.Error("Committed() should be true after Write")
	}

	if w.Body.String() != string(data) {
		t.Errorf("body = %q, want %q", w.Body.String(), string(data))
	}
}

func TestResponseWriteMultiple(t *testing.T) {
	w := httptest.NewRecorder()
	rw := NewResponse(w)

	data1 := []byte("hello")
	data2 := []byte(" ")
	data3 := []byte("world")

	rw.Write(data1)
	rw.Write(data2)
	rw.Write(data3)

	expectedSize := int64(len(data1) + len(data2) + len(data3))
	if rw.Size() != expectedSize {
		t.Errorf("Size() = %d, want %d", rw.Size(), expectedSize)
	}

	if w.Body.String() != "hello world" {
		t.Errorf("body = %q, want %q", w.Body.String(), "hello world")
	}
}

func TestResponseCommitted(t *testing.T) {
	t.Run("not committed initially", func(t *testing.T) {
		w := httptest.NewRecorder()
		rw := NewResponse(w)

		if rw.Committed() {
			t.Error("Committed() should be false initially")
		}
	})

	t.Run("committed after WriteHeader", func(t *testing.T) {
		w := httptest.NewRecorder()
		rw := NewResponse(w)

		rw.WriteHeader(http.StatusCreated)

		if !rw.Committed() {
			t.Error("Committed() should be true after WriteHeader")
		}
	})

	t.Run("committed after Write", func(t *testing.T) {
		w := httptest.NewRecorder()
		rw := NewResponse(w)

		rw.Write([]byte("test"))

		if !rw.Committed() {
			t.Error("Committed() should be true after Write")
		}
	})
}

func TestResponseWriteHeaderMultiple(t *testing.T) {
	// Capture log output to verify error logging
	w := httptest.NewRecorder()
	rw := NewResponse(w)

	// First call should succeed
	rw.WriteHeader(http.StatusOK)

	if rw.Status() != http.StatusOK {
		t.Errorf("first Status() = %d, want %d", rw.Status(), http.StatusOK)
	}

	// Second call should be ignored (and logged)
	rw.WriteHeader(http.StatusBadRequest)

	if rw.Status() != http.StatusOK {
		t.Errorf("Status() = %d, want %d (second WriteHeader should be ignored)", rw.Status(), http.StatusOK)
	}
}

func TestResponseUnwrap(t *testing.T) {
	w := httptest.NewRecorder()
	rw := NewResponse(w)

	unwrapped := rw.Unwrap()

	if unwrapped != w {
		t.Error("Unwrap() should return the original ResponseWriter")
	}
}

func TestResponseImplicitStatus(t *testing.T) {
	w := httptest.NewRecorder()
	rw := NewResponse(w)

	// Write without calling WriteHeader
	rw.Write([]byte("test"))

	// Should default to 200 OK
	if rw.Status() != http.StatusOK {
		t.Errorf("implicit status = %d, want %d", rw.Status(), http.StatusOK)
	}

	if w.Code != http.StatusOK {
		t.Errorf("underlying recorder code = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestResponseExplicitStatus(t *testing.T) {
	w := httptest.NewRecorder()
	rw := NewResponse(w)

	rw.WriteHeader(http.StatusCreated)
	rw.Write([]byte("test"))

	if rw.Status() != http.StatusCreated {
		t.Errorf("status = %d, want %d", rw.Status(), http.StatusCreated)
	}
}

func TestResponseHeaderOperations(t *testing.T) {
	w := httptest.NewRecorder()
	rw := NewResponse(w)

	// Set headers before committing
	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("X-Custom", "value")

	rw.WriteHeader(http.StatusOK)

	if got := w.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}

	if got := w.Header().Get("X-Custom"); got != "value" {
		t.Errorf("X-Custom = %q, want %q", got, "value")
	}
}

func TestResponseSize(t *testing.T) {
	tests := []struct {
		name  string
		data  [][]byte
		total int64
	}{
		{"empty", [][]byte{}, 0},
		{"single write", [][]byte{[]byte("hello")}, 5},
		{"multiple writes", [][]byte{[]byte("hello"), []byte(" "), []byte("world")}, 11},
		{"large write", [][]byte{bytes.Repeat([]byte("a"), 1000)}, 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			rw := NewResponse(w)

			for _, data := range tt.data {
				rw.Write(data)
			}

			if rw.Size() != tt.total {
				t.Errorf("Size() = %d, want %d", rw.Size(), tt.total)
			}
		})
	}
}

func TestResponseWithHandler(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := NewResponse(w)
		rw.WriteHeader(http.StatusCreated)
		rw.Write([]byte("created"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}

	if rec.Body.String() != "created" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "created")
	}
}

func TestResponseEmptyWrite(t *testing.T) {
	w := httptest.NewRecorder()
	rw := NewResponse(w)

	n, err := rw.Write([]byte{})

	if err != nil {
		t.Fatalf("Write error: %v", err)
	}

	if n != 0 {
		t.Errorf("Write returned %d, want 0", n)
	}

	if rw.Size() != 0 {
		t.Errorf("Size() = %d, want 0", rw.Size())
	}

	// Empty write should still commit the response
	if !rw.Committed() {
		t.Error("Committed() should be true after Write")
	}
}

func TestResponseStatusBeforeAndAfterWrite(t *testing.T) {
	w := httptest.NewRecorder()
	rw := NewResponse(w)

	// Default status before any operations
	if rw.Status() != http.StatusOK {
		t.Errorf("initial status = %d, want %d", rw.Status(), http.StatusOK)
	}

	// Set explicit status
	rw.WriteHeader(http.StatusNoContent)

	// Write data
	rw.Write([]byte("test"))

	// Status should remain the explicitly set value
	if rw.Status() != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rw.Status(), http.StatusNoContent)
	}
}

func BenchmarkResponseWrite(b *testing.B) {
	w := httptest.NewRecorder()
	rw := NewResponse(w)
	data := []byte("benchmark data")

	for b.Loop() {
		rw.Write(data)
	}
}

func BenchmarkResponseWriteHeader(b *testing.B) {
	for b.Loop() {
		w := httptest.NewRecorder()
		rw := NewResponse(w)
		rw.WriteHeader(http.StatusOK)
	}
}

func TestResponseFlush(t *testing.T) {
	t.Run("with flusher support", func(t *testing.T) {
		// httptest.ResponseRecorder implements Flusher
		w := httptest.NewRecorder()
		rw := NewResponse(w)

		// Should not panic
		rw.Flush()

		// Verify flush was called on the underlying recorder
		if !w.Flushed {
			t.Error("expected underlying Flush to be called")
		}
	})

	t.Run("without flusher support", func(t *testing.T) {
		// Create a ResponseWriter that doesn't implement Flusher
		w := &nonFlusherWriter{header: make(http.Header)}
		rw := NewResponse(w)

		// Should panic when Flusher is not supported
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic when Flush is not supported")
			}
		}()

		rw.Flush()
	})
}

func TestResponseHijack(t *testing.T) {
	t.Run("without hijacker support", func(t *testing.T) {
		// httptest.ResponseRecorder doesn't implement Hijacker
		w := httptest.NewRecorder()
		rw := NewResponse(w)

		conn, buf, err := rw.Hijack()

		if err == nil {
			t.Error("expected error when Hijack is not supported")
		}

		if conn != nil {
			t.Error("conn should be nil when Hijack is not supported")
		}

		if buf != nil {
			t.Error("buf should be nil when Hijack is not supported")
		}
	})
}

func TestResponsePush(t *testing.T) {
	t.Run("without pusher support", func(t *testing.T) {
		// httptest.ResponseRecorder doesn't implement Pusher
		w := httptest.NewRecorder()
		rw := NewResponse(w)

		err := rw.Push("/static/style.css", nil)

		if err == nil {
			t.Error("expected error when Push is not supported")
		}
	})

	t.Run("with pusher support", func(t *testing.T) {
		// Create a mock ResponseWriter that implements Pusher
		w := &mockPusherWriter{
			ResponseWriter: httptest.NewRecorder(),
			pushed:         make(map[string]*http.PushOptions),
		}
		rw := NewResponse(w)

		target := "/static/style.css"
		opts := &http.PushOptions{
			Method: "GET",
			Header: http.Header{"Accept": []string{"text/css"}},
		}

		err := rw.Push(target, opts)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if _, ok := w.pushed[target]; !ok {
			t.Errorf("expected Push to be called with target %q", target)
		}
	})
}

// nonFlusherWriter is a ResponseWriter that doesn't implement Flusher
type nonFlusherWriter struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (w *nonFlusherWriter) Header() http.Header {
	return w.header
}

func (w *nonFlusherWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(b)
}

func (w *nonFlusherWriter) WriteHeader(statusCode int) {
	if w.status == 0 {
		w.status = statusCode
	}
}

// mockPusherWriter is a ResponseWriter that implements Pusher
type mockPusherWriter struct {
	http.ResponseWriter
	pushed map[string]*http.PushOptions
}

func (w *mockPusherWriter) Push(target string, opts *http.PushOptions) error {
	w.pushed[target] = opts
	return nil
}
