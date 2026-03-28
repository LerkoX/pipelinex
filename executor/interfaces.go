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
	// 只支持 string 类型的命令
	Transfer(ctx context.Context, resultChan chan<- any, commandChan <-chan any)
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
	StepName string // 步骤名称
	Command  string // 要执行的命令
}
