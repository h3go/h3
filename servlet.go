package h3

import "context"

// Servlet 服务组件接口，表示可以启动和停止的服务
//
// 实现此接口的组件在服务器启动时会自动调用 Start 方法，
// 在服务器关闭时会自动调用 Stop 方法。这对于需要独立生命周期
// 管理的组件（如数据库连接、消息队列、后台任务等）特别有用。
//
// 生命周期:
//   - Start: 在 HTTP 服务器启动之前被调用
//   - Stop: 在 HTTP 服务器关闭时被调用（逆序执行）
//
// 注意:
//   - 如果 Start 返回错误，服务器启动会失败
//   - Stop 方法应该实现幂等性，可以安全地多次调用
//   - 多个 Servlet 的 Stop 方法按注册顺序的逆序执行
//
// 使用场景:
//   - 数据库连接池的初始化和关闭
//   - 消息队列的连接管理
//   - 后台任务的启动和停止
//   - 定时任务的调度管理
//
// 示例:
//
//	type DatabaseComponent struct {
//		*h3.component
//		db *sql.DB
//	}
//
//	func (c *DatabaseComponent) Start(ctx context.Context) error {
//		db, err := sql.Open("postgres", "connection-string")
//		if err != nil {
//			return err
//		}
//		c.db = db
//		return db.PingContext(ctx)
//	}
//
//	func (c *DatabaseComponent) Stop() error {
//		if c.db != nil {
//			return c.db.Close()
//		}
//		return nil
//	}
type Servlet interface {
	// Start 启动服务组件
	//
	// 参数:
	//   - ctx: 上下文，用于超时控制和取消信号
	//
	// 返回:
	//   - error: 启动失败时返回错误，会导致整个服务器启动失败
	Start(ctx context.Context) error

	// Stop 停止服务组件
	//
	// 此方法在服务器关闭时被调用，应该清理所有资源。
	// 实现应该是幂等的，可以安全地多次调用。
	//
	// 返回:
	//   - error: 停止失败时返回错误（会被记录但不会阻止关闭流程）
	Stop() error
}
