# 交互式输入功能使用指南

## 功能说明

从 v1.0 开始，Pipeline 支持交互式命令执行。外部程序可以向正在运行的命令发送输入数据，实现用户交互。

## 获取交互式输入通道

`inputChan` 存储在节点的 `RuntimeStatus` 中，可以通过以下方式获取：

```go
// 1. 获取节点的运行时状态
runtimeStatus := node.GetRuntimeStatus()
if runtimeStatus != nil && runtimeStatus.InputChan != nil {
    inputChan := runtimeStatus.InputChan

    // 2. 向 inputChan 发送输入数据
    inputChan <- []byte("用户输入\n")
}
```

## 使用示例

### 示例 1：简单交互式命令

```yaml
Nodes:
  InteractiveNode:
    executor: local
    steps:
      - name: ask-name
        run: |
          echo "请输入你的名字："
          read name
          echo "你好，$name！"
```

外部程序交互代码：

```go
package main

import (
    "bufio"
    "context"
    "fmt"
    "os"
    "time"

    "github.com/LerkoX/pipelinex"
)

func main() {
    ctx := context.Background()
    runtime := pipelinex.NewRuntime(ctx)

    config := `
Version: "1.0"
Name: interactive-pipeline
Graph: |
  stateDiagram-v2
    [*] --> InteractiveNode
    InteractiveNode --> [*]

Nodes:
  InteractiveNode:
    executor: local
    steps:
      - name: ask-name
        run: |
          echo "请输入你的名字："
          read name
          echo "你好，$name！"
`

    // 异步运行 pipeline
    pipeline, err := runtime.RunAsync(ctx, "demo", config, nil)
    if err != nil {
        panic(err)
    }

    // 获取节点并启动交互处理
    go handleInteraction(ctx, pipeline, "InteractiveNode")

    // 等待 pipeline 完成
    <-pipeline.Done()
}

func handleInteraction(ctx context.Context, pipeline pipelinex.Pipeline, nodeName string) {
    // 获取节点
    graph := pipeline.GetGraph()
    nodes := graph.GetVertices()

    var targetNode pipelinex.Node
    for _, node := range nodes {
        if node.Id() == nodeName {
            targetNode = node
            break
        }
    }

    if targetNode == nil {
        return
    }

    // 持续检查运行时状态
    ticker := time.NewTicker(100 * time.Millisecond)
    defer ticker.Stop()

    var inputChan chan []byte

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            runtimeStatus := targetNode.GetRuntimeStatus()
            if runtimeStatus != nil && runtimeStatus.InputChan != nil {
                if inputChan != runtimeStatus.InputChan {
                    // 发现新的 inputChan，开始交互
                    inputChan = runtimeStatus.InputChan
                    go promptUser(inputChan)
                }
            }
        }
    }
}

func promptUser(inputChan chan []byte) {
    reader := bufio.NewReader(os.Stdin)
    fmt.Print("你的输入: ")

    for {
        line, err := reader.ReadString('\n')
        if err != nil {
            close(inputChan)
            return
        }

        // 发送用户输入到命令
        inputChan <- []byte(line)

        // 提示符
        fmt.Print("你的输入: ")
    }
}
```

### 示例 2：自动化输入（程序自动回答）

```go
func autoAnswer(inputChan chan []byte) {
    // 等待一段时间后自动发送输入
    time.Sleep(2 * time.Second.Second)

    answers := []string{
        "Alice\n",
        "Yes\n",
        "continue\n",
    }

    for _, answer := range answers {
        time.Sleep(1 * time.Second)
        select {
        case inputChan <- []byte(answer):
        case <-time.After(5 * time.Second):
            fmt.Println("输入超时")
            return
        }
    }
}
```

### 示例 3：Kubernetes Pod 中的交互

```yaml
Nodes:
  K8sNode:
    executor: k8s
    image: ubuntu:latest
    steps:
      - name: interactive-step
        run: |
          apt-get update && apt-get install -y nano
          echo "请编辑文件（Ctrl+X 退出）："
          nano /tmp/test.txt
          cat /tmp/test.txt
```

外部程序可以同样通过 `inputChan` 向 Pod 发送按键（如 Ctrl+X）：

```go
// 发送 Ctrl+X (nano 退出快捷键)
inputChan <- []byte{0x18}  // Ctrl+X
```

## 注意事项

1. **InputChan 生命周期**：
   - `inputChan` 在节点开始执行时创建
   - 在节点执行完成后自动关闭
   - 不要重复写入已关闭的 channel

2. **线程安全**：
   - `inputChan` 是并发安全的
   - 可以从多个 goroutine 发送数据

3. **错误处理**：
   - 写入已关闭的 channel 会 panic
   - 建议使用 `select` 语句配合 `default` 或 `context` 超时处理

4. **TTY 模式**：
   - Kubernetes 和 Docker executor 支持启用 TTY
   - 启用 TTY 后输入方式类似终端（支持 Ctrl+C 等控制字符）

## 配置 TTY 模式

```go
// 通过 Node 配置启用 TTY
Nodes:
  NodeName:
    executor: k8s
    config:
      tty: true
      ttyWidth: 80
      ttyHeight: 24
```

## API 参考

### NodeRuntimeStatus

```go
type NodeRuntimeStatus struct {
    Id         string
    Status     string
    StartTime  string
    EndTime    string
    Steps      []StepRuntimeStatus
    Executor   *ExecutorRuntimeInfo
    Custom     map[string]interface{}
    InputChan  chan []byte  // 交互式输入通道
}
```

### 获取方式

```go
// 方式 1：通过 Node 接口
runtimeStatus := node.GetRuntimeStatus()
inputChan := runtimeStatus.InputChan

// 方式 2：通过 Graph 遍历
graph := pipeline.GetGraph()
nodes := graph.GetVertices()
for _, node := range nodes {
    if node.Status() == pipelinex.StatusRunning {
        runtimeStatus := node.GetRuntimeStatus()
        if runtimeStatus != nil && runtimeStatus.InputChan != nil {
            // 处理交互
        }
    }
}
```
