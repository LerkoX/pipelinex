# Runtime 测试配置文件说明

本目录包含 `runtime_test.go` 中测试用例使用的配置文件，每个文件对应特定的测试功能。

## 基础执行测试

| 配置文件 | 测试用例 | 测试功能 |
|---------|---------|---------|
| `sync_pipeline.yaml` | `TestRuntimeImpl_RunSync` | 测试同步执行流水线，执行完成后自动清理 |
| `async_pipeline.yaml` | `TestRuntimeImpl_RunAsync` | 测试异步执行流水线，流水线保持在 Runtime 中 |
| `invalid_config.yaml` | `TestRuntimeImpl_RunSync_InvalidConfig` | 测试无效 YAML 配置的错误处理 |
| `single_node.yaml` | `TestRuntimeImpl_RunSync_DuplicateID` | 测试重复 ID 检测，相同 ID 的流水线不能重复执行 |
| `long_running.yaml` | `TestRuntimeImpl_Cancel` | 测试流水线取消功能，取消长时间运行的流水线 |
| `concurrent_template.yaml` | `TestRuntimeImpl_ConcurrentAccess` | 测试并发访问安全，同时运行多个流水线 |

## Param 模板渲染测试

| 配置文件 | 测试用例 | 测试功能 |
|---------|---------|---------|
| `param_self_reference.yaml` | `TestRuntimeImpl_RenderParam_SelfReference` | 测试 Param 自引用，参数可以引用之前定义的参数 |
| `param_circular_reference.yaml` | `TestRuntimeImpl_RenderParam_CircularReference` | 测试 Param 循环引用检测，防止无限循环 |
| `metadata_ref_param.yaml` | `TestRuntimeImpl_RenderMetadata_ReferenceParam` | 测试 Metadata 引用 Param，元数据可以引用参数值 |
| `param_nested.yaml` | `TestRuntimeImpl_RenderParam_NestedStructures` | 测试 Param 嵌套结构（map、slice）的模板渲染 |
| `param_undefined.yaml` | `TestRuntimeImpl_RenderParam_WithUndefinedVariable` | 测试 Param 使用未定义变量，未定义变量保持模板字符串原样 |

## 数据传递与并行执行测试

| 配置文件 | 测试用例 | 测试功能 |
|---------|---------|---------|
| `node_data_passing.yaml` | `TestRuntimeImpl_NodeDataPassing` | 测试节点间数据传递通过 `extract` 字段提取数据并在后续节点使用 |
| `parallel_nodes.yaml` | `TestRuntimeImpl_ParallelNodes` | 测试并行节点执行，独立任务并行运行以提升性能 |

## 运行时恢复测试

| 配置文件 | 测试用例 | 测试功能 |
|---------|---------|---------|
| `runtime_recovery.yaml` | `TestRuntimeImpl_RuntimeRecovery` | 测试运行时状态恢复，已完成的节点会被跳过 |
| `parallel_with_step_recovery.yaml` | `TestRuntimeImpl_ParallelStepRecovery` | 测试并行节点中的步骤级别恢复，精细控制步骤执行 |

## 综合功能测试

| 配置文件 | 测试用例 | 测试功能 |
|---------|---------|---------|
| `comprehensive_sync.yaml` | `TestComprehensivePipelineExecution/同步执行流水线` | 综合测试同步流水线执行和事件监听 |
| `comprehensive_async.yaml` | `TestComprehensivePipelineExecution/异步执行流水线` | 综合测试异步流水线执行和流水线存储 |
| `comprehensive_param_render.yaml` | `TestComprehensivePipelineExecution/Param模板渲染` | 综合测试 Param 模板渲染功能 |
| `comprehensive_metadata.yaml` | `TestComprehensivePipelineExecution/Metadata创建和渲染` | 综合测试 Metadata 创建、参数引用和渲染 |
| `comprehensive_dag.yaml` | `TestComprehensivePipelineExecution/多节点DAG执行` | 综合测试多节点 DAG 执行和验证事件触发 |

## 图解析测试

以下配置文件用于图解析测试，这些测试直接在代码中构造配置对象：

| 测试用例 | 测试功能 |
|---------|---------|
| `TestParseGraphEdges_BasicStateDiagram` | 测试基本状态图解析，验证节点创建 |
| `TestParseGraphEdges_ComplexDiagram` | 测试复杂状态图（并行路径）解析 |
| `TestParseGraphEdges_EmptyGraph` | 测试空图处理，验证节点仍然被创建 |
| `TestParseGraphEdges_InvalidSyntax` | 测试无效语法处理，容错性验证 |
| `TestParseGraphEdges_MissingNode` | 测试配置中缺失节点的情况 |
| `TestParseGraphEdges_ConditionalEdges` | 测试条件边解析和表达式提取 |
| `TestParseGraphEdges_UnconditionalEdges` | 测试无条件边解析 |
| `TestParseGraphEdges_WithNotes` | 测试带注释的图解析 |

## 其他模板测试

| 配置文件 | 测试用例 | 测试功能 |
|---------|---------|---------|
| `parallel_template.yaml` | `TestComprehensivePipelineExecution/并行执行` | 模板文件，用于格式化生成多个并发流水线配置 |

## 配置文件说明

### sync_pipeline.yaml
简单的双节点串行流水线，用于测试同步执行。

### async_pipeline.yaml
双节点串行流水线，用于测试异步执行。

### invalid_config.yaml
故意损坏的 YAML 内容，用于测试错误处理。

### single_node.yaml
单节点流水线，用于测试重复 ID 场景。

### long_running.yaml
包含 `sleep 2` 命令的流水线，用于测试取消功能。

### concurrent_template.yaml
包含格式化占位符的模板，用于测试并发场景。

### param_self_reference.yaml
包含参数自引用：`imageName: "myapp-{{ Param.buildId }}"`，`fullImage: "{{ Param.imageName }}:latest"`

### param_circular_reference.yaml
包含循环引用：`a -> b -> c -> a`，用于测试循环检测。

### metadata_ref_param.yaml
Metadata 数据引用 Param：`K8sNamespace: "{{ Param.namespace }}-{{ Param.env }}"`

### param_nested.yaml
包含嵌套结构：map 嵌套、slice 嵌套等。

### param_undefined.yaml
引用未定义变量 `Param.version`，测试容错处理。

### node_data_passing.yaml
使用 `extract: type: codec-block` 提取数据，并通过 `{{ Generate.value }}` 在后续节点引用。

### parallel_nodes.yaml
并行执行 TaskA、TaskB、TaskC，最后在 Merge 节点汇聚。

### runtime_recovery.yaml
Step1 设置 `runtime: { status: SUCCESS }`，测试已完成的节点被跳过。

### parallel_with_step_recovery.yaml
Task2 所有步骤设置为 SUCCESS，Task3 部分步骤设置 SUCCESS，测试步骤级别恢复。

### comprehensive_*.yaml
一系列综合测试配置，覆盖同步/异步执行、参数渲染、Metadata 和多节点 DAG 等功能。