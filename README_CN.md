# H3

H3 是一个轻量级、高性能的 Go HTTP 框架，基于 Go 1.22+ 的增强路由功能构建。

简体中文 | [English](README.md)

## 特性

- 🚀 **基于标准库** - 使用 Go 1.22+ 的 `http.ServeMux` 增强路由
- 🧩 **组件化架构** - 通过 Component 模式实现模块化应用结构
- 🔄 **生命周期管理** - Servlet 接口支持组件的启动和停止生命周期
- 🔌 **中间件支持** - 洋葱模型中间件链，支持全局和路由级中间件
- 📊 **响应包装** - 自动捕获 HTTP 状态码、响应大小和写入状态
- ⚡ **优雅关闭** - 内置优雅关闭支持
- 🎯 **类型安全** - 完全的类型安全，无反射

## 安装

```bash
go get github.com/h3go/h3
```

**要求**: Go 1.25.5 或更高版本

## 快速开始

```go
package main

import (
    "net/http"
    "github.com/h3go/h3"
)

func main() {
    // 创建路由器
    mux := h3.NewMux()
    
    // 注册路由
    mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Hello, H3!"))
    })
    
    mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
        id := r.PathValue("id")
        w.Write([]byte("User ID: " + id))
    })
    
    // 启动应用
    app := h3.New(mux, h3.Options{Addr: ":8080"})
    app.Start()
}
```

## 核心概念

### 1. Mux (路由复用器)

Mux 是 H3 的核心路由器，包装了 Go 1.23+ 的 `http.ServeMux`：

```go
mux := h3.NewMux()

// 注册处理器
mux.Handle("GET /api/users", usersHandler)
mux.HandleFunc("POST /api/users", createUser)

// 挂载子路由
apiMux := h3.NewMux()
apiMux.HandleFunc("GET /status", getStatus)
mux.Mount("/api", apiMux)
```

### 2. Component (组件)

Component 是可独立注册的路由模块，用于组织大型应用：

```go
// 创建用户模块
usersComponent := h3.NewComponent("/users")
usersComponent.Mux().HandleFunc("GET /", listUsers)
usersComponent.Mux().HandleFunc("GET /{id}", getUser)
usersComponent.Mux().HandleFunc("POST /", createUser)

// 创建管理员模块
adminComponent := h3.NewComponent("/admin")
adminComponent.Mux().HandleFunc("GET /dashboard", dashboard)

// 注册到服务器
app := h3.New(h3.NewMux(), h3.Options{Addr: ":8080"})
app.Register(usersComponent)
app.Register(adminComponent)
app.Start()
```

### 3. Response (响应包装器)

Response 自动包装 `http.ResponseWriter`，捕获响应信息：

```go
mux.Use(func(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        rw := h3.NewResponse(w)
        next.ServeHTTP(rw, r)
        
        // 记录响应信息
        log.Printf("Status: %d, Size: %d bytes, Committed: %v",
            rw.Status(), rw.Size(), rw.Committed())
    })
})
```

Response 接口支持高级特性：

```go
// HTTP/2 服务器推送
rw.Push("/static/style.css", nil)

// 流式响应 (SSE)
fmt.Fprintf(rw, "data: %s\n\n", message)
rw.Flush()

// WebSocket 升级
conn, buf, err := rw.Hijack()
```

### 4. Servlet (服务组件)

Servlet 是一个可选接口，用于管理组件的生命周期。实现此接口的组件可以在服务器启动和关闭时自动初始化和清理资源：

```go
type DatabaseComponent struct {
    *h3.Component
    db *sql.DB
}

func (c *DatabaseComponent) Start(ctx context.Context) error {
    // 在服务器启动时连接数据库
    db, err := sql.Open("postgres", "connection-string")
    if err != nil {
        return err
    }
    c.db = db
    return db.PingContext(ctx)
}

func (c *DatabaseComponent) Stop() error {
    // 在服务器关闭时断开数据库连接
    if c.db != nil {
        return c.db.Close()
    }
    return nil
}

// 注册实现了 Servlet 的组件
app := h3.New(h3.NewMux(), h3.Options{Addr: ":8080"})
app.Register(dbComponent) // Start 会自动调用
// ... 服务器运行
app.Stop(ctx)             // Stop 会自动调用
```

**Servlet 特性**：
- ✅ 自动生命周期管理
- ✅ Start 在 HTTP 服务器启动之前调用
- ✅ Stop 按注册顺序的逆序调用（后进先出）
- ✅ Start 失败会阻止服务器启动
- ✅ Stop 是幂等的，可以安全地多次调用

**常见使用场景**：
- 数据库连接池的初始化和关闭
- 消息队列的连接管理
- 后台任务的启动和停止
- 定时任务的调度管理
- 缓存系统的初始化

### 5. Middleware (中间件)

中间件采用标准的 `func(http.Handler) http.Handler` 签名：

```go
// 自定义中间件
func Logger(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        next.ServeHTTP(w, r)
        log.Printf("%s %s - %v", r.Method, r.URL.Path, time.Since(start))
    })
}

// 使用中间件
mux.Use(Logger)
```

中间件执行顺序遵循洋葱模型：

```
Request → M1 → M2 → M3 → Handler → M3 → M2 → M1 → Response
```

## 完整示例

### 模块化应用

```go
package main

import (
    "encoding/json"
    "net/http"
    "github.com/h3go/h3"
)

// User 模块
func NewUsersComponent() h3.Component {
    c := h3.NewComponent("/users")
    mux := c.Mux()
    
    // 用户列表
    mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
        users := []map[string]string{
            {"id": "1", "name": "Alice"},
            {"id": "2", "name": "Bob"},
        }
        json.NewEncoder(w).Encode(users)
    })
    
    // 用户详情
    mux.HandleFunc("GET /{id}", func(w http.ResponseWriter, r *http.Request) {
        id := r.PathValue("id")
        user := map[string]string{"id": id, "name": "User " + id}
        json.NewEncoder(w).Encode(user)
    })
    
    return c
}

// Admin 模块
func NewAdminComponent() h3.Component {
    c := h3.NewComponent("/admin")
    mux := c.Mux()
    
    // 管理员专用中间件
    mux.Use(func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // 权限检查
            token := r.Header.Get("Authorization")
            if token == "" {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }
            next.ServeHTTP(w, r)
        })
    })
    
    mux.HandleFunc("GET /dashboard", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Admin Dashboard"))
    })
    
    return c
}

func main() {
    // 创建根路由器
    mux := h3.NewMux()
    
    // 根路由
    mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Welcome to H3!"))
    })
    
    // 创建应用并注册组件
    app := h3.New(mux, h3.Options{Addr: ":8080"})
    app.Register(NewUsersComponent())
    app.Register(NewAdminComponent())
    
    // 启动应用
    app.Start()
}
```

### 优雅关闭

```go
func main() {
    mux := h3.NewMux()
    mux.HandleFunc("GET /", handler)
    
    app := h3.New(mux, h3.Options{Addr: ":8080"})
    
    // 在 goroutine 中启动
    go app.Start()
    
    // 等待信号
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    <-sigChan
    
    // 优雅关闭
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    if err := app.Stop(ctx); err != nil {
        log.Printf("Server shutdown error: %v", err)
    }
}
```

## 路由模式

H3 使用 Go 1.23+ 的路由模式语法：

```go
mux.HandleFunc("GET /users", listUsers)              // 精确匹配
mux.HandleFunc("GET /users/{id}", getUser)           // 路径参数
mux.HandleFunc("GET /files/{path...}", serveFile)    // 通配符
mux.HandleFunc("POST /users", createUser)            // 方法匹配
mux.HandleFunc("/about", about)                      // 所有方法
```

访问路径参数：

```go
func handler(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    path := r.PathValue("path")
}
```

## 性能

H3 直接基于标准库的 `http.ServeMux`，性能接近原生 Go HTTP 服务器：

- 零反射
- 最小内存分配
- 高效的路由匹配
- 轻量级中间件链

## 测试

```bash
# 运行所有测试
go test ./...

# 运行测试并显示覆盖率
go test -cover ./...

# 详细测试输出
go test -v ./...
```

当前测试覆盖率：**95.4%**

## 与其他框架对比

| 特性 | H3 | Chi | Echo | Gin |
|-----|-----|-----|------|-----|
| 基于标准库 | ✅ | ✅ | ❌ | ❌ |
| Go 1.23+ 路由 | ✅ | ❌ | ❌ | ❌ |
| 零反射 | ✅ | ✅ | ❌ | ❌ |
| 组件化 | ✅ | ❌ | ❌ | ❌ |
| 优雅关闭 | ✅ | ✅ | ✅ | ✅ |
| 中间件 | ✅ | ✅ | ✅ | ✅ |

## 许可证

MIT License

## 贡献

欢迎提交 Issue 和 Pull Request！
