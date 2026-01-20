package h3

// Component 应用组件接口，代表一个可独立注册的路由模块
type Component interface {
	Mux() Mux       // 获取组件的路由器
	Prefix() string // 获取组件的路径前缀
}

// NewComponent 创建新的应用组件
func NewComponent(prefix string) Component {
	return &component{
		mux:    NewMux(),
		prefix: prefix,
	}
}

// component 应用组件的内部实现
type component struct {
	mux    Mux    // 组件路由器
	prefix string // 路径前缀
}

// Mux 返回应用组件的路由器
func (c *component) Mux() Mux {
	return c.mux
}

// Prefix 返回应用组件的路径前缀
func (c *component) Prefix() string {
	return c.prefix
}
