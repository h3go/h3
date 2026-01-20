package h3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestNewServer(t *testing.T) {
	mux := NewMux()
	srv := NewServer(":8080", mux)

	if srv == nil {
		t.Fatal("NewServer returned nil")
	}

	if srv.opts.Addr != ":8080" {
		t.Errorf("addr = %q, want %q", srv.opts.Addr, ":8080")
	}

	if srv.mux == nil {
		t.Error("mux should not be nil")
	}
}

func TestServerUse(t *testing.T) {
	mux := NewMux()
	srv := NewServer(":8080", mux)

	called := false
	srv.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			next.ServeHTTP(w, r)
		})
	})

	// Add a handler to test middleware
	srv.mux.HandleFunc("GET /test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	// Verify middleware was added by making a test request
	ctx := context.Background()
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = srv.Stop(ctx) }()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://localhost:8080/test")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	if !called {
		t.Error("middleware was not called")
	}
}

func TestServerRegister(t *testing.T) {
	mux := NewMux()
	srv := NewServer(":8081", mux)

	// Create a component
	c := NewComponent("/api")
	c.Mux().HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	// Register component
	srv.Register(c)

	// Start server
	ctx := context.Background()
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = srv.Stop(ctx) }()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Test component route
	resp, err := http.Get("http://localhost:8081/api/status")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if string(body) != "ok" {
		t.Errorf("body = %q, want %q", string(body), "ok")
	}
}

func TestServerStartStop(t *testing.T) {
	mux := NewMux()
	mux.HandleFunc("GET /test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("running"))
	})

	srv := NewServer(":8082", mux)
	ctx := context.Background()

	// Start server
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Verify server is running
	resp, err := http.Get("http://localhost:8082/test")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// Stop server
	if err := srv.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Give server time to stop
	time.Sleep(100 * time.Millisecond)

	// Verify server is stopped
	_, err = http.Get("http://localhost:8082/test")
	if err == nil {
		t.Error("expected error when connecting to stopped server")
	}
}

func TestServerInvalidAddress(t *testing.T) {
	tests := []struct {
		name string
		addr string
	}{
		{"no port", "localhost"},
		{"invalid format", ":::"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := NewMux()
			srv := NewServer(tt.addr, mux)
			ctx := context.Background()

			err := srv.Start(ctx)
			if err == nil {
				_ = srv.Stop(ctx)
				t.Error("Start should fail with invalid address")
			}
		})
	}
}

func TestServerValidAddress(t *testing.T) {
	tests := []struct {
		name string
		addr string
		port int
	}{
		{"localhost with port", ":8083", 8083},
		{"explicit localhost", "localhost:8084", 8084},
		{"ipv4", "127.0.0.1:8085", 8085},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := NewMux()
			srv := NewServer(tt.addr, mux)
			ctx := context.Background()

			if err := srv.Start(ctx); err != nil {
				t.Fatalf("Start failed: %v", err)
			}
			defer func() { _ = srv.Stop(ctx) }()

			time.Sleep(100 * time.Millisecond)

			// Try to connect
			url := fmt.Sprintf("http://localhost:%d/", tt.port)
			resp, err := http.Get(url)
			if err != nil {
				t.Fatalf("GET failed: %v", err)
			}
			defer resp.Body.Close()
		})
	}
}

func TestServerMultipleComponents(t *testing.T) {
	mux := NewMux()
	srv := NewServer(":8086", mux)

	// Register multiple components
	apiComponent := NewComponent("/api")
	apiComponent.Mux().HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("api"))
	})

	adminComponent := NewComponent("/admin")
	adminComponent.Mux().HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("admin"))
	})

	srv.Register(apiComponent)
	srv.Register(adminComponent)

	ctx := context.Background()
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = srv.Stop(ctx) }()

	time.Sleep(100 * time.Millisecond)

	// Test both components
	tests := []struct {
		path string
		want string
	}{
		{"/api/status", "api"},
		{"/admin/status", "admin"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			resp, err := http.Get("http://localhost:8086" + tt.path)
			if err != nil {
				t.Fatalf("GET failed: %v", err)
			}
			defer func() { resp.Body.Close() }()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("ReadAll failed: %v", err)
			}
			if string(body) != tt.want {
				t.Errorf("body = %q, want %q", string(body), tt.want)
			}
		})
	}
}

func TestServerGracefulShutdown(t *testing.T) {
	mux := NewMux()

	// Add a slow handler
	mux.HandleFunc("GET /slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Write([]byte("done"))
	})

	srv := NewServer(":8087", mux)
	ctx := context.Background()

	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Start a slow request
	done := make(chan bool)
	go func() {
		resp, err := http.Get("http://localhost:8087/slow")
		if err != nil {
			t.Errorf("slow request failed: %v", err)
		} else {
			resp.Body.Close()
		}
		done <- true
	}()

	// Give the request time to start
	time.Sleep(50 * time.Millisecond)

	// Stop server (should wait for slow request)
	if err := srv.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Verify the slow request completed
	select {
	case <-done:
		// Request completed successfully
	case <-time.After(1 * time.Second):
		t.Error("slow request did not complete")
	}
}

func TestServerContextPropagation(t *testing.T) {
	mux := NewMux()

	contextReceived := false
	mux.HandleFunc("GET /test", func(w http.ResponseWriter, r *http.Request) {
		if r.Context() != nil {
			contextReceived = true
		}
		w.Write([]byte("ok"))
	})

	srv := NewServer(":8088", mux)
	ctx := context.Background()

	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = srv.Stop(ctx) }()

	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://localhost:8088/test")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	resp.Body.Close()

	if !contextReceived {
		t.Error("handler did not receive context")
	}
}

func TestServerWithMiddlewareAndComponents(t *testing.T) {
	mux := NewMux()
	srv := NewServer(":8089", mux)

	// Add global middleware
	srv.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Global", "true")
			next.ServeHTTP(w, r)
		})
	})

	// Create component with its own middleware
	c := NewComponent("/api")
	c.Mux().Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Component", "true")
			next.ServeHTTP(w, r)
		})
	})

	c.Mux().HandleFunc("GET /test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	srv.Register(c)

	ctx := context.Background()
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = srv.Stop(ctx) }()

	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://localhost:8089/api/test")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("X-Global") != "true" {
		t.Error("global middleware not executed")
	}

	if resp.Header.Get("X-Component") != "true" {
		t.Error("component middleware not executed")
	}
}

func TestServerStartMultipleTimes(t *testing.T) {
	mux := NewMux()
	mux.HandleFunc("GET /test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	srv := NewServer(":8090", mux)
	ctx := context.Background()

	// Start server
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Verify server is running
	resp, err := http.Get("http://localhost:8090/test")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	resp.Body.Close()

	// Stop server
	if err := srv.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Verify server is stopped
	_, err = http.Get("http://localhost:8090/test")
	if err == nil {
		t.Error("expected error when connecting to stopped server")
	}
}

// mockServletComponent 实现了 Component 和 Servlet 接口的测试组件
type mockServletComponent struct {
	*component
	startCalled bool
	stopCalled  bool
	startError  error
	stopError   error
	mu          sync.Mutex
}

func newMockServletComponent(prefix string) *mockServletComponent {
	return &mockServletComponent{
		component: NewComponent(prefix).(*component),
	}
}

func (c *mockServletComponent) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.startCalled = true
	return c.startError
}

func (c *mockServletComponent) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopCalled = true
	return c.stopError
}

func (c *mockServletComponent) wasStartCalled() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.startCalled
}

func (c *mockServletComponent) wasStopCalled() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stopCalled
}

func TestServerServletLifecycle(t *testing.T) {
	mux := NewMux()
	srv := NewServer(":8091", mux)

	// 创建实现了 Servlet 接口的组件
	servlet := newMockServletComponent("/servlet")
	servlet.Mux().HandleFunc("GET /test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	srv.Register(servlet)

	ctx := context.Background()

	// 启动服务器应该调用 Servlet.Start
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if !servlet.wasStartCalled() {
		t.Error("Servlet.Start was not called")
	}

	// 停止服务器应该调用 Servlet.Stop
	if err := srv.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if !servlet.wasStopCalled() {
		t.Error("Servlet.Stop was not called")
	}
}

func TestServerServletStartError(t *testing.T) {
	mux := NewMux()
	srv := NewServer(":8092", mux)

	// 创建会在 Start 时返回错误的组件
	servlet := newMockServletComponent("/servlet")
	servlet.startError = errors.New("start failed")

	srv.Register(servlet)

	ctx := context.Background()

	// 启动服务器应该失败
	err := srv.Start(ctx)
	if err == nil {
		_ = srv.Stop(ctx)
		t.Fatal("Start should fail when Servlet.Start returns error")
	}

	if err.Error() != "start failed" {
		t.Errorf("error = %q, want %q", err.Error(), "start failed")
	}
}

func TestServerMultipleServlets(t *testing.T) {
	mux := NewMux()
	srv := NewServer(":8093", mux)

	// 创建多个 Servlet 组件
	servlet1 := newMockServletComponent("/servlet1")
	servlet2 := newMockServletComponent("/servlet2")
	servlet3 := newMockServletComponent("/servlet3")

	srv.Register(servlet1)
	srv.Register(servlet2)
	srv.Register(servlet3)

	ctx := context.Background()

	// 启动服务器
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 验证所有 Servlet 都被启动
	if !servlet1.wasStartCalled() {
		t.Error("servlet1.Start was not called")
	}
	if !servlet2.wasStartCalled() {
		t.Error("servlet2.Start was not called")
	}
	if !servlet3.wasStartCalled() {
		t.Error("servlet3.Start was not called")
	}

	// 停止服务器
	if err := srv.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// 验证所有 Servlet 都被停止（应该按逆序）
	if !servlet1.wasStopCalled() {
		t.Error("servlet1.Stop was not called")
	}
	if !servlet2.wasStopCalled() {
		t.Error("servlet2.Stop was not called")
	}
	if !servlet3.wasStopCalled() {
		t.Error("servlet3.Stop was not called")
	}
}

// servletWithOrder 用于测试 Stop 调用顺序
type servletWithOrder struct {
	*mockServletComponent
	id        int
	stopOrder *[]int
	mu        *sync.Mutex
}

func (s *servletWithOrder) Stop() error {
	s.mu.Lock()
	*s.stopOrder = append(*s.stopOrder, s.id)
	s.mu.Unlock()
	return s.mockServletComponent.Stop()
}

func TestServerServletStopOrder(t *testing.T) {
	mux := NewMux()
	srv := NewServer(":8094", mux)

	// 记录 Stop 调用顺序
	var stopOrder []int
	var mu sync.Mutex

	createServlet := func(id int, prefix string) *servletWithOrder {
		return &servletWithOrder{
			mockServletComponent: newMockServletComponent(prefix),
			id:                   id,
			stopOrder:            &stopOrder,
			mu:                   &mu,
		}
	}

	servlet1 := createServlet(1, "/s1")
	servlet2 := createServlet(2, "/s2")
	servlet3 := createServlet(3, "/s3")

	srv.Register(servlet1)
	srv.Register(servlet2)
	srv.Register(servlet3)

	ctx := context.Background()

	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if err := srv.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// 验证 Stop 按逆序调用：3, 2, 1
	mu.Lock()
	defer mu.Unlock()

	if len(stopOrder) != 3 {
		t.Fatalf("stopOrder length = %d, want 3", len(stopOrder))
	}

	expectedOrder := []int{3, 2, 1}
	for i, id := range stopOrder {
		if id != expectedOrder[i] {
			t.Errorf("stopOrder[%d] = %d, want %d", i, id, expectedOrder[i])
		}
	}
}

func TestServerMixedComponents(t *testing.T) {
	mux := NewMux()
	srv := NewServer(":8095", mux)

	// 注册普通组件（不实现 Servlet）
	normalComponent := NewComponent("/normal")
	normalComponent.Mux().HandleFunc("GET /test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("normal"))
	})

	// 注册 Servlet 组件
	servlet := newMockServletComponent("/servlet")
	servlet.Mux().HandleFunc("GET /test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("servlet"))
	})

	srv.Register(normalComponent)
	srv.Register(servlet)

	ctx := context.Background()

	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 只有 Servlet 组件应该被启动
	if !servlet.wasStartCalled() {
		t.Error("servlet.Start was not called")
	}

	// 测试两个组件的路由都正常工作
	resp, err := http.Get("http://localhost:8095/normal/test")
	if err != nil {
		t.Fatalf("GET /normal/test failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "normal" {
		t.Errorf("normal component body = %q, want %q", string(body), "normal")
	}

	resp, err = http.Get("http://localhost:8095/servlet/test")
	if err != nil {
		t.Fatalf("GET /servlet/test failed: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "servlet" {
		t.Errorf("servlet component body = %q, want %q", string(body), "servlet")
	}

	if err := srv.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if !servlet.wasStopCalled() {
		t.Error("servlet.Stop was not called")
	}
}

// servletWithContextCapture 用于捕获传入的 context
type servletWithContextCapture struct {
	*mockServletComponent
	receivedCtx *context.Context
}

func (s *servletWithContextCapture) Start(ctx context.Context) error {
	*s.receivedCtx = ctx
	return s.mockServletComponent.Start(ctx)
}

func TestServerServletWithContext(t *testing.T) {
	mux := NewMux()
	srv := NewServer(":8096", mux)

	var receivedCtx context.Context
	servlet := &servletWithContextCapture{
		mockServletComponent: newMockServletComponent("/servlet"),
		receivedCtx:          &receivedCtx,
	}

	srv.Register(servlet)

	// 创建带值的 context
	ctx := context.WithValue(context.Background(), "test", "value") //nolint:staticcheck // SA1029: test code

	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 验证 Servlet.Start 收到了正确的 context
	if receivedCtx == nil {
		t.Fatal("Servlet.Start did not receive context")
	}

	if receivedCtx.Value("test") != "value" {
		t.Error("Servlet.Start received wrong context")
	}

	err := srv.Stop(ctx)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}
