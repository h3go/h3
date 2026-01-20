package h3

import (
	"errors"
	"net/http"
)

// Mux 路由复用器接口，扩展了标准库的 http.ServeMux
//
// Mux 在 http.ServeMux 的基础上添加了中间件和子路由挂载功能。
// 它完全兼容 http.Handler 接口，并支持 Go 1.22+ ServeMux 的所有特性
// （方法匹配、通配符、路径参数等）。
//
// 示例用法：
//
//	mux := h3.NewMux()
//	mux.Use(h3.RequestLogger())
//	mux.HandleFunc("GET /users/{id}", handleUser)
//	mux.Mount("/api", apiMux)
type Mux interface {
	// Use 添加中间件到中间件链
	// 中间件按注册顺序执行：先注册的在外层，后注册的在内层
	Use(func(http.Handler) http.Handler)

	// Handler 返回匹配请求的处理器和模式
	// 这是对底层 http.ServeMux.Handler 的封装
	Handler(r *http.Request) (h http.Handler, pattern string)

	// Handle 注册处理器到指定路由模式
	// pattern 支持方法前缀、通配符等 Go 1.22+ ServeMux 特性
	Handle(pattern string, handler http.Handler)

	// HandleFunc 注册处理函数到指定路由模式
	// 这是 Handle 方法的便捷包装
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))

	// Mount 将子路由挂载到指定路径
	// 子路由的所有路径都会添加 pattern 作为前缀
	//
	// 示例：
	//   mux.Mount("/api", apiMux)
	//   // apiMux 中的 "GET /users" 会变成 "GET /api/users"
	Mount(pattern string, mux Mux)

	// ServeHTTP 实现 http.Handler 接口
	ServeHTTP(http.ResponseWriter, *http.Request)
}

// mux 路由复用器的内部实现
type mux struct {
	mux *http.ServeMux                  // 底层标准库路由器
	pre func(http.Handler) http.Handler // 已合并的中间件链
}

// NewMux 创建新的路由复用器
//
// 返回的 Mux 实例可以注册路由、添加中间件和挂载子路由。
func NewMux() Mux {
	return &mux{
		mux: http.NewServeMux(),
	}
}

// Use 添加中间件到中间件链
//
// 中间件按注册顺序执行，形成洋葱模型：
//   - 先注册的中间件在外层（先执行 before，后执行 after）
//   - 后注册的中间件在内层（后执行 before，先执行 after）
//
// 示例：
//
//	mux.Use(loggingMiddleware)  // 外层
//	mux.Use(authMiddleware)     // 内层
//	// 执行顺序：logging before -> auth before -> handler -> auth after -> logging after
func (m *mux) Use(middleware func(http.Handler) http.Handler) {
	pre := m.pre

	m.pre = func(next http.Handler) http.Handler {
		if pre != nil {
			// 将新中间件包装在已有中间件链的内部
			return pre(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				middleware(next).ServeHTTP(w, r)
			}))
		} else {
			// 第一个中间件
			return middleware(next)
		}
	}
}

// Handler 返回匹配给定请求的处理器和模式
//
// 这是对底层 http.ServeMux.Handler 方法的直接封装。
// 返回的处理器不包含中间件，只是原始注册的处理器。
func (m *mux) Handler(r *http.Request) (h http.Handler, pattern string) {
	return m.mux.Handler(r)
}

// Handle 注册处理器到指定路由模式
//
// pattern 支持 Go 1.22+ ServeMux 的所有特性：
//   - 方法匹配：GET /path, POST /path
//   - 路径参数：/users/{id}, /files/{path...}
//   - 主机匹配：example.com/path
//
// 如果 pattern 为空或 handler 为 nil，会触发 panic。
func (m *mux) Handle(pattern string, handler http.Handler) {
	m.register(pattern, handler)
}

// HandleFunc 注册处理函数到指定路由模式
//
// 这是 Handle 方法的便捷包装，自动将函数转换为 http.HandlerFunc。
func (m *mux) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	m.register(pattern, http.HandlerFunc(handler))
}

// Mount 将子路由挂载到指定路径
//
// 子路由中的所有模式都会自动添加 pattern 作为前缀。
// 例如，将 apiMux 挂载到 "/api"，apiMux 中的 "GET /users"
// 会被注册为 "GET /api/users"。
//
// 特殊情况：
//   - pattern == "/" : 直接挂载到根路径
//   - pattern 带尾部斜杠（如 "/api/"）: 自动规范化为 "/api"
//   - pattern == "" : 触发 panic
//
// 实现细节：
// 对于非根路径，Mount 会添加通配符 {path...} 来捕获所有子路径，
// 然后使用 http.StripPrefix 移除前缀后转发给子路由。
func (m *mux) Mount(pattern string, mux Mux) {
	// 拒绝空字符串
	if pattern == "" {
		panic(errors.New("h3: invalid pattern"))
	}

	// 根路径特殊处理
	if pattern == "/" {
		m.register("/", mux)
		return
	}

	// 规范化 pattern：去掉尾部斜杠
	if len(pattern) > 0 && pattern[len(pattern)-1] == '/' {
		pattern = pattern[:len(pattern)-1]
	}

	// 添加通配符以匹配所有子路径
	// 例如: /api -> /api/{path...}
	// StripPrefix 会移除 /api 前缀，然后交给子路由处理
	m.register(pattern+"/{path...}", http.StripPrefix(pattern, mux))
}

// register 注册路由，如果参数无效则 panic
func (mux *mux) register(pattern string, handler http.Handler) {
	if err := mux.registerErr(pattern, handler); err != nil {
		panic(err)
	}
}

// registerErr 注册路由并返回错误而不是 panic
//
// 参数验证与 http.ServeMux.Handle 保持一致：
//   - pattern 不能为空
//   - handler 不能为 nil
//   - http.HandlerFunc 类型的 handler 不能为 nil 函数
func (m *mux) registerErr(pattern string, handler http.Handler) error {
	if pattern == "" {
		return errors.New("h3: invalid pattern")
	}
	if handler == nil {
		return errors.New("h3: nil handler")
	}
	if f, ok := handler.(http.HandlerFunc); ok && f == nil {
		return errors.New("h3: nil handler")
	}

	m.mux.Handle(pattern, handler)
	return nil
}

// ServeHTTP 实现 http.Handler 接口
//
// 如果存在中间件，会先应用中间件链，然后调用底层路由器。
// 如果没有中间件，直接调用底层路由器。
func (m *mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if m.pre != nil {
		m.pre(m.mux).ServeHTTP(NewResponse(w), r)
	} else {
		m.mux.ServeHTTP(NewResponse(w), r)
	}
}
