package h3

import (
	"context"
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"time"
)

// Options 提供了对 HTTP 服务器行为的细粒度控制，包括超时、TLS 配置、
// 协议支持等。所有字段都是可选的，未设置的字段将使用 Go 标准库的默认值。
type Options struct {
	// Addr 可选地指定服务器监听的 TCP 地址，格式为 "host:port"。
	// 如果为空，使用 ":http"（端口 80）。
	// 服务名称在 RFC 6335 中定义并由 IANA 分配。
	// 地址格式的详细信息请参见 net.Dial。
	Addr string

	// DisableGeneralOptionsHandler 如果为 true，将 "OPTIONS *" 请求传递给 Handler，
	// 否则响应 200 OK 和 Content-Length: 0。
	DisableGeneralOptionsHandler bool

	// TLSConfig 可选地提供 TLS 配置供 ServeTLS 和 ListenAndServeTLS 使用。
	// 注意，此值会被 ServeTLS 和 ListenAndServeTLS 克隆，因此无法使用
	// tls.Config.SetSessionTicketKeys 等方法修改配置。
	// 要使用 SetSessionTicketKeys，请改用 Server.Serve 配合 TLS Listener。
	TLSConfig *tls.Config

	// ReadTimeout 是读取整个请求（包括请求体）的最大持续时间。
	// 零值或负值表示没有超时。
	//
	// 因为 ReadTimeout 不允许 Handler 对每个请求体的可接受截止时间或
	// 上传速率做出单独决策，大多数用户会更倾向于使用 ReadHeaderTimeout。
	// 同时使用两者也是有效的。
	ReadTimeout time.Duration

	// ReadHeaderTimeout 是允许读取请求头的时间量。
	// 读取请求头后，连接的读取截止时间会被重置，Handler 可以决定
	// 请求体的读取速度是否太慢。如果为零，使用 ReadTimeout 的值。
	// 如果为负值，或者为零且 ReadTimeout 为零或负值，则没有超时。
	ReadHeaderTimeout time.Duration

	// WriteTimeout 是响应写入超时前的最大持续时间。
	// 每当读取新请求的头部时，它会被重置。与 ReadTimeout 类似，
	// 它不允许 Handler 基于每个请求做出决策。
	// 零值或负值表示没有超时。
	WriteTimeout time.Duration

	// IdleTimeout 是启用 keep-alive 时等待下一个请求的最大时间量。
	// 如果为零，使用 ReadTimeout 的值。如果为负值，或者为零且
	// ReadTimeout 为零或负值，则没有超时。
	IdleTimeout time.Duration

	// MaxHeaderBytes 控制服务器在解析请求头的键和值时读取的最大字节数，
	// 包括请求行。它不限制请求体的大小。
	// 如果为零，使用 DefaultMaxHeaderBytes。
	MaxHeaderBytes int

	// TLSNextProto 可选地指定一个函数，当 ALPN 协议升级发生时接管
	// 提供的 TLS 连接的所有权。map 的键是协商的协议名称。
	// Handler 参数应该用于处理 HTTP 请求，如果尚未设置，
	// 它将初始化 Request 的 TLS 和 RemoteAddr。
	// 函数返回时连接会自动关闭。
	// 如果 TLSNextProto 不为 nil，HTTP/2 支持不会自动启用。
	TLSNextProto map[string]func(*http.Server, *tls.Conn, http.Handler)

	// ConnState 指定一个可选的回调函数，当客户端连接状态改变时调用。
	// 详情请参见 ConnState 类型和相关常量。
	ConnState func(net.Conn, http.ConnState)

	// ErrorLog 指定一个可选的日志记录器，用于记录接受连接时的错误、
	// Handler 的意外行为以及底层 FileSystem 的错误。
	// 如果为 nil，通过 log 包的标准日志记录器进行日志记录。
	ErrorLog *log.Logger

	// HTTP2 配置 HTTP/2 连接。
	//
	// 此字段目前尚未生效。
	// 详见 https://go.dev/issue/67813。
	HTTP2 *http.HTTP2Config

	// Protocols 是服务器接受的协议集。
	//
	// 如果 Protocols 包含 UnencryptedHTTP2，服务器将接受未加密的 HTTP/2 连接。
	// 服务器可以在同一地址和端口上同时提供 HTTP/1 和未加密的 HTTP/2。
	//
	// 如果 Protocols 为 nil，默认通常是 HTTP/1 和 HTTP/2。
	// 如果 TLSNextProto 不为 nil 且不包含 "h2" 条目，默认仅为 HTTP/1。
	Protocols *http.Protocols
}

// Server HTTP 服务器
type Server struct {
	opts  *Options        // 服务器参数
	mux   Mux             // 路由复用器
	servs []Servlet       // Servlet 服务组件列表
	exit  chan chan error // 优雅关闭通道
}

// New 创建 HTTP 服务器实例
//
// 参数:
//   - mux: 路由复用器
//   - options: 可选的服务器配置参数（可选）
//
// 返回:
//   - *Server: 服务器实例
//
// 示例:
//
//	// 使用默认配置
//	server := h3.New(mux)
//
//	// 使用自定义配置
//	server := h3.New(mux, h3.Options{
//		Addr:         ":8080",
//		ReadTimeout:  10 * time.Second,
//		WriteTimeout: 10 * time.Second,
//	})
func New(mux Mux, options ...Options) *Server {
	var opts Options
	if len(options) > 0 {
		opts = options[0]
	}

	return &Server{
		opts: &opts,
		mux:  mux,
		exit: make(chan chan error),
	}
}

// NewServer 创建 HTTP 服务器实例（向后兼容）
//
// 此函数保留用于向后兼容。推荐使用 New 函数。
//
// 参数:
//   - addr: 监听地址，格式为 "host:port"
//   - mux: 路由复用器
//
// 返回:
//   - *Server: 服务器实例
func NewServer(addr string, mux Mux) *Server {
	return New(mux, Options{Addr: addr})
}

// Use 添加全局中间件
func (s *Server) Use(middleware func(http.Handler) http.Handler) {
	s.mux.Use(middleware)
}

// Register 注册应用组件到服务器
//
// 此方法会将应用组件的路由挂载到服务器的主路由器上。
// 如果应用组件实现了 Servlet 接口，还会将其添加到服务组件列表中，
// 以便在服务器启动和关闭时自动调用其 Start 和 Stop 方法。
//
// 参数:
//   - c: 要注册的应用组件
func (s *Server) Register(c Component) {
	// 挂载组件路由
	s.mux.Mount(c.Prefix(), c.Mux())

	// 如果组件实现了 Servlet 接口，添加到服务组件列表
	if serv, ok := c.(Servlet); ok {
		s.servs = append(s.servs, serv)
	}
}

// Handler 根据请求查找匹配的处理器和模式
//
// 此方法委托给内部路由器，返回能够处理该请求的 Handler 和匹配的路由模式。
//
// 参数:
//   - r: HTTP 请求
//
// 返回:
//   - h: 匹配的处理器
//   - pattern: 匹配的路由模式
func (s *Server) Handler(r *http.Request) (h http.Handler, pattern string) {
	return s.mux.Handler(r)
}

// Handle 注册路由模式和对应的处理器
//
// 此方法委托给内部路由器，将指定的处理器绑定到路由模式。
//
// 参数:
//   - pattern: 路由模式（例如 "GET /users/{id}"）
//   - handler: 处理该路由的 http.Handler
func (s *Server) Handle(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, handler)
}

// HandleFunc 注册路由模式和对应的处理函数
//
// 此方法委托给内部路由器，将处理函数绑定到路由模式。
// 这是 Handle 方法的便捷版本，接受函数而不是 http.Handler。
//
// 参数:
//   - pattern: 路由模式（例如 "GET /users/{id}"）
//   - handler: 处理该路由的函数
func (s *Server) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	s.mux.HandleFunc(pattern, handler)
}

// ServeHTTP 实现 http.Handler 接口，将请求委托给内部的路由器处理
//
// 这使得 Server 本身可以作为一个 http.Handler 使用，
// 可以嵌套在其他 HTTP 服务器或中间件中。
//
// 参数:
//   - w: HTTP 响应写入器
//   - r: HTTP 请求
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// Start 启动 HTTP 服务器(非阻塞)
//
// 此方法会按顺序执行以下操作:
//  1. 验证监听地址格式
//  2. 启动所有注册的 Servlet 组件（调用 Start 方法）
//  3. 启动 HTTP 服务器（在后台 goroutine 中）
//  4. 设置优雅关闭处理（在后台 goroutine 中等待 Stop 信号）
//
// 如果任何 Servlet 的 Start 方法返回错误，整个启动过程会失败。
//
// 参数:
//   - ctx: 用于 Servlet 启动的上下文
//
// 返回:
//   - error: 地址无效或 Servlet 启动失败时返回错误
func (s *Server) Start(ctx context.Context) error {
	opts := s.opts

	// 验证监听地址格式
	if _, _, err := net.SplitHostPort(opts.Addr); err != nil {
		return err
	}

	// 启动所有 Servlet 组件
	for i, serv := range s.servs {
		if err := serv.Start(ctx); err != nil {
			// 如果启动失败，则逆序停止已启动的 Servlet 组件
			for j := i - 1; j >= 0; j-- {
				stopErr := s.servs[j].Stop()
				if stopErr != nil {
					log.Println(stopErr)
				}
			}
			return err
		}
	}

	lctx, cancel := context.WithCancel(context.Background())

	server := &http.Server{
		Addr:                         opts.Addr,
		Handler:                      s.mux,
		DisableGeneralOptionsHandler: opts.DisableGeneralOptionsHandler,
		TLSConfig:                    opts.TLSConfig,
		ReadTimeout:                  opts.ReadTimeout,
		ReadHeaderTimeout:            opts.ReadHeaderTimeout,
		WriteTimeout:                 opts.WriteTimeout,
		IdleTimeout:                  opts.IdleTimeout,
		MaxHeaderBytes:               opts.MaxHeaderBytes,
		TLSNextProto:                 opts.TLSNextProto,
		ConnState:                    opts.ConnState,
		ErrorLog:                     opts.ErrorLog,
		BaseContext:                  func(net.Listener) context.Context { return lctx },
		HTTP2:                        opts.HTTP2,
		Protocols:                    opts.Protocols,
	}

	// 优雅关闭处理
	go func() {
		defer cancel()
		exit := <-s.exit

		// 逆序停止所有 Servlet 组件
		for i := len(s.servs) - 1; i >= 0; i-- {
			err := s.servs[i].Stop()
			if err != nil {
				log.Println(err)
			}
		}

		// 关闭 HTTP 服务器并返回结果
		exit <- server.Shutdown(lctx)
	}()

	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Panicln(err)
		}
	}()

	return nil
}

// Stop 优雅停止 HTTP 服务器
//
// 此方法会按顺序执行以下操作:
//  1. 发送关闭信号
//  2. 逆序停止所有 Servlet 组件（调用 Stop 方法）
//  3. 优雅关闭 HTTP 服务器（等待现有连接完成）
//
// 参数:
//   - ctx: 用于控制关闭超时的上下文
//
// 返回:
//   - error: 关闭过程中的错误
func (s *Server) Stop(ctx context.Context) error {
	exit := make(chan error)
	s.exit <- exit
	return <-exit
}
