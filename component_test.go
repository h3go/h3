package h3

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewComponent(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
	}{
		{"empty prefix", ""},
		{"root prefix", "/"},
		{"simple prefix", "/api"},
		{"nested prefix", "/api/v1"},
		{"with trailing slash", "/api/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewComponent(tt.prefix)
			if c == nil {
				t.Fatal("NewComponent returned nil")
			}

			if c.Prefix() != tt.prefix {
				t.Errorf("Prefix() = %q, want %q", c.Prefix(), tt.prefix)
			}

			if c.Mux() == nil {
				t.Error("Mux() returned nil")
			}
		})
	}
}

func TestComponentMux(t *testing.T) {
	c := NewComponent("/api")
	mux := c.Mux()

	if mux == nil {
		t.Fatal("Mux() returned nil")
	}

	// Verify the mux is functional
	mux.HandleFunc("GET /test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ServeHTTP status = %d, want %d", rec.Code, http.StatusOK)
	}

	if rec.Body.String() != "test" {
		t.Errorf("ServeHTTP body = %q, want %q", rec.Body.String(), "test")
	}
}

func TestComponentPrefix(t *testing.T) {
	tests := []struct {
		prefix string
	}{
		{"/api"},
		{"/api/v1"},
		{"/admin"},
		{""},
		{"/"},
	}

	for _, tt := range tests {
		c := NewComponent(tt.prefix)
		if got := c.Prefix(); got != tt.prefix {
			t.Errorf("Prefix() = %q, want %q", got, tt.prefix)
		}
	}
}

func TestComponentIntegration(t *testing.T) {
	// Create component with routes
	c := NewComponent("/api")
	c.Mux().HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("users list"))
	})
	c.Mux().HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("user " + r.PathValue("id")))
	})

	// Create server and register component
	mux := NewMux()
	mux.Mount(c.Prefix(), c.Mux())

	tests := []struct {
		path string
		want string
	}{
		{"/api/users", "users list"},
		{"/api/users/123", "user 123"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
			}

			if got := rec.Body.String(); got != tt.want {
				t.Errorf("body = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestComponentWithMiddleware(t *testing.T) {
	c := NewComponent("/api")

	// Add middleware to component's mux
	called := false
	c.Mux().Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.Header().Set("X-Middleware", "executed")
			next.ServeHTTP(w, r)
		})
	})

	c.Mux().HandleFunc("GET /test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	c.Mux().ServeHTTP(rec, req)

	if !called {
		t.Error("middleware was not called")
	}

	if got := rec.Header().Get("X-Middleware"); got != "executed" {
		t.Errorf("X-Middleware header = %q, want %q", got, "executed")
	}
}

func TestMultipleComponents(t *testing.T) {
	// Create multiple components
	apiComponent := NewComponent("/api")
	apiComponent.Mux().HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("api ok"))
	})

	adminComponent := NewComponent("/admin")
	adminComponent.Mux().HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("admin ok"))
	})

	// Mount both components
	mux := NewMux()
	mux.Mount(apiComponent.Prefix(), apiComponent.Mux())
	mux.Mount(adminComponent.Prefix(), adminComponent.Mux())

	tests := []struct {
		path string
		want string
	}{
		{"/api/status", "api ok"},
		{"/admin/status", "admin ok"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
			}

			if got := rec.Body.String(); got != tt.want {
				t.Errorf("body = %q, want %q", got, tt.want)
			}
		})
	}
}
