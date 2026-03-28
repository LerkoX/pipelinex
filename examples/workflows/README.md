# Pipelinex 本地工作流示例

本目录包含使用 pipelinex 流水线库的本地工作流配置示例。

## 工作流列表

### 1. 文件处理工作流 (file_processing.yaml)

自动化日志管理流水线，包含以下功能：
- 创建日志目录和归档目录
- 压缩日志文件
- 生成归档文件清单
- 清理超过保留天数的旧文件

**特性**：
- 并行执行：创建目录、压缩日志、生成清单并行运行
- 数据传递：各节点结果通过 `codec-block` 提取并传递
- 参数化配置：通过 `Param` 自定义目录路径、保留天数等

**使用方法**：
```bash
# 修改 Param 中的参数
# projectDir: 你的项目目录
# retentionDays: 保留天数
```

---

### 2. 数据采集处理工作流 (data_etl.yaml)

ETL（Extract, Transform, Load）流水线，包含以下功能：
- 并行采集用户、订单、产品数据
- 数据格式标准化和转换
- 生成用户报告和销售报告
- 合并所有报告

**特性**：
- 并行数据采集：三个 API 调用同时执行
- 数据传递：使用 `codec-block` 提取和传递结构化数据
- 完整的数据处理流程

**使用方法**：
```bash
# 修改 Param 中的参数
# date: 数据采集日期
# baseDir: 数据存储基础目录
# apiBaseUrl: API 基础 URL
```

---

## 运行工作流

### 方法一：使用预编译的 runner 工具（推荐）

```bash
# 进入工作流目录
cd examples/workflows

# 一键构建（使用 build.sh 脚本）
./build.sh

# 运行 runner 工具，选择工作流
./runner
```

**或直接编译**：
```bash
go build -o runner main.go
```

runner 会列出当前目录下的所有 `.yaml` 配置文件，你可以通过数字选择要运行的工作流。

### 方法二：直接运行 Go 代码

```bash
# 进入工作流目录
cd examples/workflows

# 运行 main.go
go run main.go
```

### 方法三：使用 pipelinex Go 库加载和执行配置文件

```go
package main

import (
    "context"
    "fmt"
    "os"
    "path/filepath"

    "github.com/yourusername/pipelinex"
)

```go
package main

import (
    "context"
    "fmt"
    "os"
    "path/filepath"

    "github.com/yourusername/pipelinex"
)

func main() {
    ctx := context.Background()

    // 加载配置文件
    configPath := filepath.Join("examples", "workflows", "file_processing.yaml")
    config, err := pipelinex.LoadConfigFromFile(configPath)
    if err != nil {
        fmt.Printf("Failed to load config: %v\n", err)
        os.Exit(1)
    }

    // 创建流水线
    pipeline, err := pipelinex.NewPipeline(config)
    if err != nil {
        fmt.Printf("Failed to create pipeline: %v\n", err)
        os.Exit(1)
    }

    // 注册事件监听器
    pipeline.OnNodeStart(func(node pipelinex.Node) {
        fmt.Printf("Node %s started\n", node.Name())
    })

    pipeline.OnNodeComplete(func(node pipelinex.Node) {
        fmt.Printf("Node %s completed\n", node.Name())
    })

    // 运行流水线
    runtime := pipelinex.NewRuntime(pipeline)
    err = runtime.Run(ctx)
    if err != nil {
        fmt.Printf("Pipeline failed: %v\n", err)
        os.Exit(1)
    }

    fmt.Println("Pipeline completed successfully")
}
```

---

## 数据传递机制

### codec-block 方式

在步骤输出中使用 `pipelinex-json` 或 `pipelinex-yaml` 代码块：

```yaml
steps:
  - name: generate-data
    run: |
      echo '```pipelinex-json'
      echo '{"value": 42, "message": "hello"}'
      echo '```'
extract:
  type: codec-block
```

然后在其他节点引用：
```yaml
steps:
  - name: use-data
    run: "echo 'Value: {{ NodeName.value }}'"
```

---

## 参数配置

所有参数在 `Param` 部分定义，可以在模板中使用：

```yaml
Param:
  date: "2024-03-28"
  baseDir: "/data/analytics"
  derivedValue: "{{ Param.baseDir }}/subdir"
```

---

## 运行时恢复

如果流水线执行失败，可以记录节点状态以便下次恢复：

```yaml
Nodes:
  Step1:
    runtime:
      status: SUCCESS  # 已完成，会被跳过
  Step2:
    runtime:
      status: PENDING   # 会执行
```

---

## 关键特性说明

### 数据传递
- **codec-block**：使用 `pipelinex-json` 或 `pipelinex-yaml` 代码块传递结构化数据
- **跨节点引用**：通过 `{{ NodeName.fieldName }}` 引用前驱节点的数据

### 并行执行
在 Graph 中定义多个并发的边即可实现并行执行

### 参数化配置
使用 `{{ Param.key }}` 在模板中引用参数

---

## 参考文件

- `/workspace/github/pipelinex/config.example.yaml` - 完整配置示例
- `/workspace/github/pipelinex/test/fixtures/runtime/` - 测试用例配置
- `/workspace/github/pipelinex/README.md` - 项目文档
