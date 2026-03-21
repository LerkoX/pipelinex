# Logger 日志模块

日志模块提供了 Pipeline 日志推送的核心接口和实现。

## 核心接口

### Pusher

日志推送器接口，所有日志推送实现都需要实现此接口。

```go
type Pusher interface {
    // Push 推送单条日志
    Push(ctx context.Context, entry Entry) error

    // PushBatch 批量推送
    PushBatch(ctx context.Context, entries []Entry) error

    // Close 关闭连接，刷新缓冲
    Close() error
}
```

### Entry

日志条目结构：

```go
type Entry struct {
    Pipeline  string    // 流水线 ID
    BuildID   string    // 构建ID
    Node      string    // 节点名称
    Step      string    // 步骤名称
    Timestamp time.Time // 时间戳
    Level     Level     // 日志级别
    Message   string    // 日志消息
    Output    string    // 命令标准输出/错误
}
```

### 日志级别

```go
const (
    LevelDebug Level = "debug"
    LevelInfo  Level = "info"
    LevelWarn  Level = "warn"
    LevelError Level = "error"
)
```

## 内置实现

### ConsolePusher

控制台日志推送器，将日志输出到标准输出。

```go
// 创建控制台日志推送器
pusher := logger.NewConsolePusher()

// 配置推送器
pusher.SetShowTime(true)   // 显示时间戳
pusher.SetShowNode(true)   // 显示节点信息

// 推送日志
pusher.Push(ctx, logger.Entry{
    Pipeline: "pipeline-id",
    Level:    logger.LevelInfo,
    Message:  "消息内容",
})
```

## 使用示例

### 在 main.go 中使用

```go
import (
    "context"
    "github.com/chenyingqiao/pipelinex"
    "github.com/chenyingqiao/pipelinex/logger"
)

func main() {
    ctx := context.Background()

    // 创建控制台日志推送器
    consolePusher := logger.NewConsolePusher()

    // 创建 Runtime 并设置日志推送器
    runtime := pipelinex.NewRuntime(ctx)
    runtime.SetPusher(consolePusher)

    // 执行流水线...
}
```

### 在事件监听器中使用

```go
type PipelineListener struct {
    pusher logger.Pusher
    ctx    context.Context
}

func (l *PipelineListener) Handle(p pipelinex.Pipeline, event pipelinex.Event) {
    if l.pusher != nil {
        l.pusher.Push(l.ctx, logger.Entry{
            Pipeline: p.Id(),
            Level:    logger.LevelInfo,
            Message:  "流水线开始执行",
        })
    }
}
```

## 扩展建议

可以基于 Pusher 接口实现其他日志推送方式：

1. **HTTP 推送器** - 将日志发送到 HTTP 端点
2. **文件推送器** - 将日志写入文件
3. **Kafka 推送器** - 将日志发送到 Kafka 消息队列
4. **Elasticsearch 推送器** - 将日志发送到 Elasticsearch
