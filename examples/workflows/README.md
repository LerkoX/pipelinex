# Pipelinex 本地工作流示例

本目录包含使用 pipelinex 流水线库的本地工作流配置示例。

## 工作流列表

### 1. 文件处理工作流 (file_processing.yaml)

自动化日志管理流水线，包含以下功能：
- 创建日志文件和归档目录
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

# 运行工作流
./runner -config file_processing.yaml
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

# 运行工作流
./runner -config data_etl.yaml
```

---

### 3. CI/CD 部署工作流 (ci_cd_deployment.yaml)

完整的 CI/CD 部署流水线，包含以下功能：
- 代码检出和构建
- 单元测试和质量检查
- 根据质量检查结果决定部署路径
- 支持多环境部署（dev、staging、production）
- 生产环境需要人工审核

**特性**：
- **条件边**：根据测试结果、代码覆盖率、环境类型决定执行路径
- **数据传递**：每个节点的检查结果通过 `codec-block` 提取供后续条件判断使用
- **质量门禁**：代码覆盖率不达标时自动跳过部署
- **人工审核**：生产环境部署需要人工批准

**使用方法**：
```bash
# 部署到开发环境（跳过测试）
# environment: dev
# skipTests: true

# 部署到预发布环境（需要通过质量检查）
# environment: staging
# skipTests: false
# codeCoverageThreshold: 80

# 部署到生产环境（需要通过质量检查 + 人工审核）
# environment: production
# skipTests: false
# codeCoverageThreshold: 80

# 运行工作流
./runner -config ci_cd_deployment.yaml
```

**条件边示例**：
```yaml
# 只有测试通过才部署到 Staging
QualityCheck --> DeployStaging: {{ QualityCheck.allTestsPassed == true and QualityCheck.codeCoverage >= Param.codeCoverageThreshold }}

# 质量检查失败则跳过部署
QualityCheck --> SkipDeploy: {{ QualityCheck.allTestsPassed == false or QualityCheck.codeCoverage < Param.codeCoverageThreshold }}
```

---

### 4. 天气飞书通知工作流 (weather_feishu_notify.yaml)

天气获取和飞书通知流水线，包含以下功能：
- 从天气 API 获取当前和未来天气信息
- 通过飞书 API 发送富文本消息通知
- 支持自定义城市和通知时间

**特性**：
- **参数化配置**：可配置城市、通知时间、天气 API
- **飞书集成**：通过飞书开放平台发送消息
- **数据传递**：天气数据通过 `codec-block` 提取并传递

**使用方法**：
```bash
# 修改 Param 中的参数
# city: 城市名称（如：深圳、北京）
# notifyTime: 通知时间
# feishuAppId: 飞书应用 ID
# feishuAppSecret: 飞书应用密钥
# feishuChatId: 飞书群/用户 ID
# scriptDir: 脚本目录的绝对路径

# 运行工作流
./runner -config weather_feishu_notify.yaml
```

**配置定时任务**：
```bash
# 添加到 crontab（每天早上 7:30 执行）
30 7 * * * /myapp/pipelinex/runner -config /myapp/pipelinex/weather_feishu_notify.yaml >> /tmp/weather_notify.log 2>&1
```

---

## 运行工作流

### 方法一：使用预编译的 runner 工具（推荐）

```bash
# 进入工作流目录
cd examples/workflows

# 一键构建（使用 build.sh 脚本）
./build.sh

# 运行 runner 工具，通过 -config 参数指定配置文件
./runner -config <配置文件名.yaml>
```

### 方法二：直接运行 Go 代码

```bash
# 进入工作流目录
cd examples/workflows

# 运行 main.go 并指定配置文件
go run main.go -config <配置文件名.yaml>
```

### 方法三：使用 pipelinex Go 库加载和执行配置文件

```go
package main

import (
    "context"
    "fmt"
    "os"
    "path/filepath"

    "github.com/LerkoX/pipelinex"
)

func main() {
    ctx := context.Background()

    // 加载配置文件
    configPath := filepath.Join("examples", "workflows", "file_processing.yaml")
    configData, err := os.ReadFile(configPath)
    if err != nil {
        fmt.Printf("读取配置文件失败: %v\n", err)
        os.Exit(1)
    }

    // 创建运行时
    runtime := pipelinex.NewRuntime(ctx)

    // 运行流水线
    pipelineID := "my-pipeline"
    pipeline, err := runtime.RunSync(ctx, pipelineID, string(configData), nil)
    if err != nil {
        fmt.Printf("流水线执行失败: %v\n", err)
        os.Exit(1)
    }

    fmt.Printf("流水线 %s 执行完成！状态: %s\n", pipeline.ID(), pipeline.Status())
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
    run: "echo 'Value: {{ .Metadata.Generate.value }}'"
```

### 正则表达式方式

使用正则表达式提取数据：

```yaml
extract:
  type: regex
  patterns:
    coverage: "coverage: (\\d+\\.\\d+)%"
    tests: "(\\d+) tests? passed"
steps:
  - name: test
    run: go test -cover
```

---

## 参数配置

所有参数在 `Param` 部分定义，可以在模板中使用：

```yaml
Param:
  date: "2024-03-28"
  baseDir: "/data/analytics"
  derivedValue: "{{ .Param.baseDir }}/subdir"
```

在步骤中引用参数：
```yaml
steps:
  - name: use-param
    run: "echo 'Date: {{ .Param.date }}'"
```

---

## 运行时恢复

如果流水线执行失败，可以记录节点状态以便下次恢复：

```yaml
Status:
  Step1: SUCCESS  # 已完成，会被跳过
  Step2: PENDING   # 会执行

Nodes:
  Step1:
    # ... 节点配置
  Step2:
    # ... 节点配置
```

---

## 关键特性说明

### 数据传递
- **codec-block**：使用 `pipelinex-json` 或 `pipelinex-yaml` 代码块传递结构化数据
- **regex**：使用正则表达式提取数据
- **跨节点引用**：通过 `{{ .Metadata.NodeName.fieldName }}` 引用前置节点的数据

### 并行执行
在 Graph 中定义多个并发的边即可实现并行执行：

```yaml
Graph: |
  stateDiagram-v2
    [*] --> Task1
    [*] --> Task2
    [*] --> Task3
    Task1 --> Merge
    Task2 --> Merge
    Task3 --> Merge
    Merge --> [*]
```

### 条件边
使用模板表达式实现条件分支：

```yaml
Graph: |
  stateDiagram-v2
    [*] --> Build
    Build --> Deploy: "{{ eq .Param.environment 'production' }}"
    Build --> Test: "{{ ne .Param.environment 'production' }}"
    Test --> [*]
    Deploy --> [*]
```

### 参数化配置
使用 `{{ .Param.key }}` 在模板中引用参数

---

## 参考文件

- `/workspace/github/pipelinex/config.example.yaml` - 完整配置示例
- `/workspace/github/pipelinex/test/fixtures/runtime/` - 测试用例配置
- `/workspace/github/pipelinex/doc/config.md` - 配置详细文档
- `/workspace/github/pipelinex/README.md` - 项目文档
