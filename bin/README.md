# PipelineX 示例程序

这是一个基础的 PipelineX 使用示例，展示了如何创建和执行一个简单的流水线。

## 功能说明

此示例演示了 PipelineX 的以下核心功能：
- 使用 Local 执行器运行本地命令
- 定义流水线图结构（DAG）
- 多步骤节点执行
- 事件监听机制
- 模板变量使用（Param）

## 编译

```bash
go build -o pipelinex-demo main.go
```

## 运行

```bash
./pipelinex-demo
```

## 示例流水线说明

示例流水线包含 3 个步骤：

1. **Step1 - 创建工作目录**: 在 /tmp 目录下创建临时工作目录
2. **Step2 - 写入文件**: 在工作目录中创建测试文件并写入内容
3. **Step3 - 清理**: 删除临时目录

## 代码结构

```go
// 1. 创建 Runtime
runtime := pipelinex.NewRuntime(ctx)

// 2. 定义事件监听器
listener := &PipelineListener{}

// 3. 定义流水线配置 (YAML 格式)
configYAML := `...`

// 4. 执行流水线
pipeline, err := runtime.RunSync(ctx, pipelineID, configYAML, listener)
```

## 扩展建议

1. **使用其他执行器**：
   - 修改 `Executors` 配置，添加 `docker` 或 `k8s` 执行器
   - 确保对应的环境已准备（Docker 守护进程、K8s 集群等）

2. **添加元数据**：
   ```yaml
   Metadate:
     type: in-config
     data:
       buildTime: "2024-01-01"
   ```

3. **异步执行**：
   使用 `RunAsync` 替代 `RunSync`，流水线将在后台执行

4. **条件边**：
   在 Graph 中使用条件表达式控制执行流
   ```yaml
   Graph: |
     stateDiagram-v2
       [*] --> Step1
       Step1 --> Step2: {{ Param.skipStep2 != true }}
   ```
