package executor

import (
	"context"
	"time"
)

// ExecutorProvider Executor提供者接口，用于根据类型创建Executor
type ExecutorProvider interface {
	// GetExecutor 根据执行器名称返回对应的Executor实例
	GetExecutor(ctx context.Context, name string) (Executor, error)
}

// Executor 执行器接口
type Executor interface {
	// Prepare 准备环境
	Prepare(ctx context.Context) error
	// Destruction 销毁环境
	Destruction(ctx context.Context) error
	// Transfer 从 commandChan 接收命令执行，并将结果发送到 resultChan
	// inputChan 用于接收交互式输入数据，可为 nil（不需要输入时）
	Transfer(ctx context.Context, resultChan chan<- any, commandChan <-chan any, inputChan <-chan []byte)
}

// ExecutorInfoProvider 用于获取 executor 运行时信息
type ExecutorInfoProvider interface {
	// GetRuntimeInfo 获取executor特定的运行时信息
	GetRuntimeInfo() map[string]any
	// GetInstanceId 获取executor实例ID (容器ID/Pod名称等)
	GetInstanceId() string
	// GetType 获取executor类型
	GetType() string
}

// ExecutorStatus 定义executor状态常量
const (
	ExecutorStatusPrepared  = "PREPARED"
	ExecutorStatusRunning   = "RUNNING"
	ExecutorStatusDestroyed = "DESTROYED"
)

// Adapter 适配器接口
type Adapter interface {
	// Config 适配器配置
	Config(ctx context.Context, config map[string]any) error
}

// Bridge 桥接器接口
type Bridge interface {
	// Conn 连接到环境中
	Conn(ctx context.Context, adapter Adapter) (Executor, error)
}

// StepResult 步骤执行结果
type StepResult struct {
	StepName   string
	Command    string
	Output     string
	Error      error
	StartTime  time.Time
	FinishTime time.Time
}

// CommandWrapper 包装命令，携带步骤元信息用于精确映射
type CommandWrapper struct {
	StepName string            // 步骤名称
	Command  string            // 要执行的命令
	Env      map[string]string // 命令级环境变量（已渲染终值）：执行器尽量以真实进程
	                           // 环境变量注入；不支持时在命令前拼接单引号转义的 export 行
}

// InputRequest 输入请求信息
// 程序通过输出 {"flowx":"wait-input",...} 来请求用户输入
type InputRequest struct {
	Prompt  string `json:"prompt"`   // 显示给用户的提示信息
	Type    string `json:"type"`     // 输入类型: text/password/confirm
	Timeout int    `json:"timeout"`  // 等待超时（秒），0表示使用默认值
}

// InputRequestEvent 输入请求事件
// 执行器检测到程序等待输入时发送此事件
type InputRequestEvent struct {
	StepName string       // 步骤名称
	Request  *InputRequest // 输入请求详情
}

// InputReadyEvent 输入就绪事件
// 通知流水线 InputChan 已准备好，可以开始发送输入
type InputReadyEvent struct {
	StepName string // 步骤名称
}
