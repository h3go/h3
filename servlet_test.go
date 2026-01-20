package h3

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// mockServlet 是一个模拟的 Servlet 实现，用于测试
type mockServlet struct {
	startCalled   bool
	stopCalled    bool
	startError    error
	stopError     error
	startDuration time.Duration
	stopDuration  time.Duration
	mu            sync.Mutex
}

func newMockServlet() *mockServlet {
	return &mockServlet{}
}

func (m *mockServlet) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.startDuration > 0 {
		select {
		case <-time.After(m.startDuration):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	m.startCalled = true
	return m.startError
}

func (m *mockServlet) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stopDuration > 0 {
		time.Sleep(m.stopDuration)
	}

	m.stopCalled = true
	return m.stopError
}

func (m *mockServlet) wasStartCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.startCalled
}

func (m *mockServlet) wasStopCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopCalled
}

func (m *mockServlet) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startCalled = false
	m.stopCalled = false
}

func TestServletInterface(t *testing.T) {
	// 验证 mockServlet 实现了 Servlet 接口
	var _ Servlet = (*mockServlet)(nil)
}

func TestServletStart(t *testing.T) {
	servlet := newMockServlet()
	ctx := context.Background()

	err := servlet.Start(ctx)
	if err != nil {
		t.Errorf("Start() error = %v, want nil", err)
	}

	if !servlet.wasStartCalled() {
		t.Error("Start() was not called")
	}
}

func TestServletStartWithError(t *testing.T) {
	servlet := newMockServlet()
	expectedErr := errors.New("start failed")
	servlet.startError = expectedErr

	ctx := context.Background()
	err := servlet.Start(ctx)

	if err != expectedErr {
		t.Errorf("Start() error = %v, want %v", err, expectedErr)
	}

	if !servlet.wasStartCalled() {
		t.Error("Start() was not called")
	}
}

func TestServletStartWithContext(t *testing.T) {
	servlet := newMockServlet()
	servlet.startDuration = 200 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := servlet.Start(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("Start() error = %v, want %v", err, context.DeadlineExceeded)
	}
}

func TestServletStartWithCancelledContext(t *testing.T) {
	servlet := newMockServlet()
	servlet.startDuration = 200 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	err := servlet.Start(ctx)
	if err != context.Canceled {
		t.Errorf("Start() error = %v, want %v", err, context.Canceled)
	}
}

func TestServletStop(t *testing.T) {
	servlet := newMockServlet()

	err := servlet.Stop()
	if err != nil {
		t.Errorf("Stop() error = %v, want nil", err)
	}

	if !servlet.wasStopCalled() {
		t.Error("Stop() was not called")
	}
}

func TestServletStopWithError(t *testing.T) {
	servlet := newMockServlet()
	expectedErr := errors.New("stop failed")
	servlet.stopError = expectedErr

	err := servlet.Stop()

	if err != expectedErr {
		t.Errorf("Stop() error = %v, want %v", err, expectedErr)
	}

	if !servlet.wasStopCalled() {
		t.Error("Stop() was not called")
	}
}

func TestServletStopIdempotency(t *testing.T) {
	servlet := newMockServlet()

	// 第一次调用 Stop
	if err := servlet.Stop(); err != nil {
		t.Fatalf("first Stop() error = %v", err)
	}

	if !servlet.wasStopCalled() {
		t.Fatal("first Stop() was not called")
	}

	// 重置标志以测试第二次调用
	servlet.reset()

	// 第二次调用 Stop（测试幂等性）
	if err := servlet.Stop(); err != nil {
		t.Errorf("second Stop() error = %v", err)
	}

	if !servlet.wasStopCalled() {
		t.Error("second Stop() was not called")
	}
}

func TestServletLifecycle(t *testing.T) {
	servlet := newMockServlet()
	ctx := context.Background()

	// 1. 启动
	if err := servlet.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if !servlet.wasStartCalled() {
		t.Error("Start() was not called")
	}

	// 2. 停止
	if err := servlet.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	if !servlet.wasStopCalled() {
		t.Error("Stop() was not called")
	}
}

func TestServletMultipleStarts(t *testing.T) {
	servlet := newMockServlet()
	ctx := context.Background()

	// 第一次启动
	if err := servlet.Start(ctx); err != nil {
		t.Fatalf("first Start() error = %v", err)
	}

	if !servlet.wasStartCalled() {
		t.Fatal("first Start() was not called")
	}

	// 重置并再次启动
	servlet.reset()

	if err := servlet.Start(ctx); err != nil {
		t.Errorf("second Start() error = %v", err)
	}

	if !servlet.wasStartCalled() {
		t.Error("second Start() was not called")
	}
}

func TestServletConcurrentAccess(t *testing.T) {
	servlet := newMockServlet()
	ctx := context.Background()

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// 并发调用 Start
	for range goroutines {
		go func() {
			defer wg.Done()
			_ = servlet.Start(ctx)
		}()
	}

	// 并发调用 Stop
	for range goroutines {
		go func() {
			defer wg.Done()
			_ = servlet.Stop()
		}()
	}

	wg.Wait()

	// 验证至少被调用过（因为并发调用）
	if !servlet.wasStartCalled() && !servlet.wasStopCalled() {
		t.Error("neither Start() nor Stop() was called")
	}
}

// databaseServlet 模拟数据库连接管理的 Servlet
type databaseServlet struct {
	connected bool
	mu        sync.Mutex
}

func (d *databaseServlet) Start(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 模拟数据库连接
	select {
	case <-time.After(10 * time.Millisecond):
		d.connected = true
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (d *databaseServlet) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 模拟关闭数据库连接
	if !d.connected {
		return errors.New("not connected")
	}
	d.connected = false
	return nil
}

func (d *databaseServlet) isConnected() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.connected
}

func TestDatabaseServletLifecycle(t *testing.T) {
	db := &databaseServlet{}
	ctx := context.Background()

	// 初始状态未连接
	if db.isConnected() {
		t.Error("database should not be connected initially")
	}

	// 启动后应该连接
	if err := db.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if !db.isConnected() {
		t.Error("database should be connected after Start()")
	}

	// 停止后应该断开
	if err := db.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	if db.isConnected() {
		t.Error("database should not be connected after Stop()")
	}
}

func TestDatabaseServletStopWithoutStart(t *testing.T) {
	db := &databaseServlet{}

	// 未启动直接停止应该返回错误
	err := db.Stop()
	if err == nil {
		t.Error("Stop() should return error when not connected")
	}

	if err.Error() != "not connected" {
		t.Errorf("Stop() error = %v, want 'not connected'", err)
	}
}

// messageQueueServlet 模拟消息队列连接管理的 Servlet
type messageQueueServlet struct {
	subscribers int
	mu          sync.Mutex
}

func (m *messageQueueServlet) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 模拟订阅消息
	m.subscribers = 5
	return nil
}

func (m *messageQueueServlet) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 模拟取消订阅
	m.subscribers = 0
	return nil
}

func (m *messageQueueServlet) getSubscribers() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.subscribers
}

func TestMessageQueueServletLifecycle(t *testing.T) {
	mq := &messageQueueServlet{}
	ctx := context.Background()

	// 初始状态无订阅者
	if mq.getSubscribers() != 0 {
		t.Errorf("subscribers = %d, want 0", mq.getSubscribers())
	}

	// 启动后应有订阅者
	if err := mq.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if mq.getSubscribers() != 5 {
		t.Errorf("subscribers = %d, want 5", mq.getSubscribers())
	}

	// 停止后应无订阅者
	if err := mq.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	if mq.getSubscribers() != 0 {
		t.Errorf("subscribers = %d, want 0", mq.getSubscribers())
	}
}

// backgroundTaskServlet 模拟后台任务管理的 Servlet
type backgroundTaskServlet struct {
	running bool
	done    chan struct{}
	mu      sync.Mutex
}

func (b *backgroundTaskServlet) Start(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.running {
		return errors.New("already running")
	}

	b.running = true
	b.done = make(chan struct{})

	// 启动后台任务
	go func() {
		defer close(b.done)
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// 模拟后台任务工作
			case <-ctx.Done():
				return
			}

			b.mu.Lock()
			if !b.running {
				b.mu.Unlock()
				return
			}
			b.mu.Unlock()
		}
	}()

	return nil
}

func (b *backgroundTaskServlet) Stop() error {
	b.mu.Lock()
	if !b.running {
		b.mu.Unlock()
		return errors.New("not running")
	}
	b.running = false
	done := b.done
	b.mu.Unlock()

	// 等待后台任务完成
	select {
	case <-done:
		return nil
	case <-time.After(500 * time.Millisecond):
		return errors.New("timeout waiting for background task")
	}
}

func (b *backgroundTaskServlet) isRunning() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.running
}

func TestBackgroundTaskServletLifecycle(t *testing.T) {
	task := &backgroundTaskServlet{}
	ctx := context.Background()

	// 初始状态未运行
	if task.isRunning() {
		t.Error("task should not be running initially")
	}

	// 启动后应该运行
	if err := task.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if !task.isRunning() {
		t.Error("task should be running after Start()")
	}

	// 等待一段时间确保任务在运行
	time.Sleep(50 * time.Millisecond)

	if !task.isRunning() {
		t.Error("task should still be running")
	}

	// 停止后应该不再运行
	if err := task.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	if task.isRunning() {
		t.Error("task should not be running after Stop()")
	}
}

func TestBackgroundTaskServletDoubleStart(t *testing.T) {
	task := &backgroundTaskServlet{}
	ctx := context.Background()

	// 第一次启动
	if err := task.Start(ctx); err != nil {
		t.Fatalf("first Start() error = %v", err)
	}

	// 第二次启动应该失败
	err := task.Start(ctx)
	if err == nil {
		t.Error("second Start() should return error")
	}

	if err.Error() != "already running" {
		t.Errorf("error = %v, want 'already running'", err)
	}

	// 清理
	err = task.Stop()
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestBackgroundTaskServletStopWithoutStart(t *testing.T) {
	task := &backgroundTaskServlet{}

	// 未启动直接停止应该返回错误
	err := task.Stop()
	if err == nil {
		t.Error("Stop() should return error when not running")
	}

	if err.Error() != "not running" {
		t.Errorf("error = %v, want 'not running'", err)
	}
}

// compositeServlet 组合多个 Servlet
type compositeServlet struct {
	servlets []Servlet
}

func (c *compositeServlet) Start(ctx context.Context) error {
	for i, servlet := range c.servlets {
		if err := servlet.Start(ctx); err != nil {
			// 如果启动失败，回滚已启动的 Servlet
			for j := i - 1; j >= 0; j-- {
				_ = c.servlets[j].Stop()
			}
			return err
		}
	}
	return nil
}

func (c *compositeServlet) Stop() error {
	var errs []error
	// 逆序停止
	for i := len(c.servlets) - 1; i >= 0; i-- {
		if err := c.servlets[i].Stop(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errs[0] // 返回第一个错误
	}
	return nil
}

func TestCompositeServlet(t *testing.T) {
	servlet1 := newMockServlet()
	servlet2 := newMockServlet()
	servlet3 := newMockServlet()

	composite := &compositeServlet{
		servlets: []Servlet{servlet1, servlet2, servlet3},
	}

	ctx := context.Background()

	// 启动组合 Servlet
	if err := composite.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// 验证所有子 Servlet 都被启动
	if !servlet1.wasStartCalled() {
		t.Error("servlet1 was not started")
	}
	if !servlet2.wasStartCalled() {
		t.Error("servlet2 was not started")
	}
	if !servlet3.wasStartCalled() {
		t.Error("servlet3 was not started")
	}

	// 停止组合 Servlet
	if err := composite.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	// 验证所有子 Servlet 都被停止
	if !servlet1.wasStopCalled() {
		t.Error("servlet1 was not stopped")
	}
	if !servlet2.wasStopCalled() {
		t.Error("servlet2 was not stopped")
	}
	if !servlet3.wasStopCalled() {
		t.Error("servlet3 was not stopped")
	}
}

func TestCompositeServletStartFailure(t *testing.T) {
	servlet1 := newMockServlet()
	servlet2 := newMockServlet()
	servlet2.startError = errors.New("servlet2 start failed")
	servlet3 := newMockServlet()

	composite := &compositeServlet{
		servlets: []Servlet{servlet1, servlet2, servlet3},
	}

	ctx := context.Background()

	// 启动应该失败
	err := composite.Start(ctx)
	if err == nil {
		t.Fatal("Start() should fail")
	}

	if err.Error() != "servlet2 start failed" {
		t.Errorf("error = %v, want 'servlet2 start failed'", err)
	}

	// servlet1 应该被启动然后回滚
	if !servlet1.wasStartCalled() {
		t.Error("servlet1 should be started before rollback")
	}

	if !servlet1.wasStopCalled() {
		t.Error("servlet1 should be stopped during rollback")
	}

	// servlet2 应该被启动但失败
	if !servlet2.wasStartCalled() {
		t.Error("servlet2 should be started")
	}

	// servlet3 不应该被启动
	if servlet3.wasStartCalled() {
		t.Error("servlet3 should not be started")
	}
}

func TestCompositeServletStopWithErrors(t *testing.T) {
	servlet1 := newMockServlet()
	servlet2 := newMockServlet()
	servlet2.stopError = errors.New("servlet2 stop failed")
	servlet3 := newMockServlet()

	composite := &compositeServlet{
		servlets: []Servlet{servlet1, servlet2, servlet3},
	}

	ctx := context.Background()

	// 先启动
	if err := composite.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// 停止应该返回错误
	err := composite.Stop()
	if err == nil {
		t.Error("Stop() should return error")
	}

	// 所有 Servlet 都应该尝试停止
	if !servlet1.wasStopCalled() {
		t.Error("servlet1 should be stopped")
	}
	if !servlet2.wasStopCalled() {
		t.Error("servlet2 should be stopped")
	}
	if !servlet3.wasStopCalled() {
		t.Error("servlet3 should be stopped")
	}
}
