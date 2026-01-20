package h3

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewMux(t *testing.T) {
	mux := NewMux()
	if mux == nil {
		t.Fatal("NewMux returned nil")
	}
}

func TestMuxHandle(t *testing.T) {
	mux := NewMux()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	mux.Handle("GET /test", handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if rec.Body.String() != "ok" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "ok")
	}
}

func TestMuxHandleFunc(t *testing.T) {
	mux := NewMux()

	mux.HandleFunc("GET /test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if rec.Body.String() != "hello" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "hello")
	}
}

func TestMuxHandler(t *testing.T) {
	mux := NewMux()

	mux.HandleFunc("GET /test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	h, pattern := mux.Handler(req)

	if h == nil {
		t.Fatal("Handler returned nil handler")
	}

	if pattern != "GET /test" {
		t.Errorf("pattern = %q, want %q", pattern, "GET /test")
	}
}

func TestMuxUse(t *testing.T) {
	mux := NewMux()

	order := []string{}

	// First middleware
	mux.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "first-before")
			next.ServeHTTP(w, r)
			order = append(order, "first-after")
		})
	})

	// Second middleware
	mux.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "second-before")
			next.ServeHTTP(w, r)
			order = append(order, "second-after")
		})
	})

	mux.HandleFunc("GET /test", func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	expected := []string{"first-before", "second-before", "handler", "second-after", "first-after"}
	if len(order) != len(expected) {
		t.Fatalf("execution order length = %d, want %d", len(order), len(expected))
	}

	for i, got := range order {
		if got != expected[i] {
			t.Errorf("order[%d] = %q, want %q", i, got, expected[i])
		}
	}
}

func TestMuxMiddlewareWithHeader(t *testing.T) {
	mux := NewMux()

	mux.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Custom", "middleware")
			next.ServeHTTP(w, r)
		})
	})

	mux.HandleFunc("GET /test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Custom"); got != "middleware" {
		t.Errorf("X-Custom header = %q, want %q", got, "middleware")
	}
}

func TestMuxMount(t *testing.T) {
	// Create sub-mux
	apiMux := NewMux()
	apiMux.HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("users"))
	})
	apiMux.HandleFunc("GET /posts", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("posts"))
	})

	// Mount to main mux
	mux := NewMux()
	mux.Mount("/api", apiMux)

	tests := []struct {
		path string
		want string
	}{
		{"/api/users", "users"},
		{"/api/posts", "posts"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
			}

			if rec.Body.String() != tt.want {
				t.Errorf("body = %q, want %q", rec.Body.String(), tt.want)
			}
		})
	}
}

func TestMuxMountRoot(t *testing.T) {
	subMux := NewMux()
	subMux.HandleFunc("GET /test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("root mount"))
	})

	mux := NewMux()
	mux.Mount("/", subMux)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if rec.Body.String() != "root mount" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "root mount")
	}
}

func TestMuxMountWithTrailingSlash(t *testing.T) {
	subMux := NewMux()
	subMux.HandleFunc("GET /test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	mux := NewMux()
	mux.Mount("/api/", subMux) // trailing slash

	req := httptest.NewRequest("GET", "/api/test", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestMuxMountPanic(t *testing.T) {
	mux := NewMux()
	subMux := NewMux()

	defer func() {
		if r := recover(); r == nil {
			t.Error("Mount with empty pattern should panic")
		}
	}()

	mux.Mount("", subMux)
}

func TestMuxHandlePanic(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		handler http.Handler
	}{
		{"empty pattern", "", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})},
		{"nil handler", "GET /test", nil},
		{"nil HandlerFunc", "GET /test", http.HandlerFunc(nil)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := NewMux()

			defer func() {
				if r := recover(); r == nil {
					t.Errorf("Handle(%q, %v) should panic", tt.pattern, tt.handler)
				}
			}()

			mux.Handle(tt.pattern, tt.handler)
		})
	}
}

func TestMuxMethodMatching(t *testing.T) {
	mux := NewMux()

	mux.HandleFunc("GET /test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("GET"))
	})

	mux.HandleFunc("POST /test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("POST"))
	})

	tests := []struct {
		method string
		want   string
		status int
	}{
		{"GET", "GET", http.StatusOK},
		{"POST", "POST", http.StatusOK},
		{"PUT", "", http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/test", nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != tt.status {
				t.Errorf("status = %d, want %d", rec.Code, tt.status)
			}

			if tt.status == http.StatusOK && rec.Body.String() != tt.want {
				t.Errorf("body = %q, want %q", rec.Body.String(), tt.want)
			}
		})
	}
}

func TestMuxPathParameters(t *testing.T) {
	mux := NewMux()

	mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		w.Write([]byte("user-" + id))
	})

	req := httptest.NewRequest("GET", "/users/123", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if rec.Body.String() != "user-123" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "user-123")
	}
}

func TestMuxWildcard(t *testing.T) {
	mux := NewMux()

	mux.HandleFunc("GET /files/{path...}", func(w http.ResponseWriter, r *http.Request) {
		path := r.PathValue("path")
		w.Write([]byte("path:" + path))
	})

	req := httptest.NewRequest("GET", "/files/a/b/c.txt", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if !strings.Contains(rec.Body.String(), "a/b/c.txt") {
		t.Errorf("body = %q, should contain %q", rec.Body.String(), "a/b/c.txt")
	}
}

func TestMuxNestedMount(t *testing.T) {
	// Create nested muxes
	usersMux := NewMux()
	usersMux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("users"))
	})

	apiMux := NewMux()
	apiMux.Mount("/users", usersMux)

	mainMux := NewMux()
	mainMux.Mount("/api", apiMux)

	req := httptest.NewRequest("GET", "/api/users/", nil)
	rec := httptest.NewRecorder()

	mainMux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if rec.Body.String() != "users" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "users")
	}
}

func TestMuxWithoutMiddleware(t *testing.T) {
	mux := NewMux()

	mux.HandleFunc("GET /test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("no middleware"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if rec.Body.String() != "no middleware" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "no middleware")
	}
}

func TestMuxResponseWrapping(t *testing.T) {
	mux := NewMux()

	mux.HandleFunc("GET /test", func(w http.ResponseWriter, r *http.Request) {
		// Verify that w is wrapped in Response
		if _, ok := w.(*Response); !ok {
			t.Error("ResponseWriter should be wrapped in Response")
		}
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)
}
