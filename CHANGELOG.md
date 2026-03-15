# 更新日志

## [Unreleased] - 2026-03-15

本版本新增了参数模板渲染功能，支持在 Metadata 中引用 Param，并大幅更新了文档。

### 新增功能

#### 1. 参数模板渲染（Param Template Rendering）
- 支持在 `Param` 中使用 pongo2 模板语法进行动态渲染
- 支持自引用：一个 Param 可以引用另一个 Param 的值
- 支持嵌套结构：列表、字典等复杂数据结构的模板渲染
- 支持访问嵌套字段：`Param.config.replicas`
- 访问方式：使用 `{{ Param.xxx }}` 语法
- 渲染时机：配置解析阶段，在 pipeline 启动前完成
- 错误处理：渲染失败会导致 pipeline 启动失败
- 新增 `config.template-render.example.yaml` - 模板渲染完整示例
- 删除 `example_test_config.yaml` - 替换为更完善的示例

#### 2. Metadata 支持渲染 Param
- Metadata 的 data 字段现在可以引用 Param 中定义的变量
- 支持组合多个 Param 创建新的元数据值
- 渲染规则：
  - Metadata 只能引用 Param，不能自引用（避免循环依赖）
  - 渲染时机：配置解析阶段，在 pipeline 启动前完成
  - 错误处理：渲染失败会导致 pipeline 启动失败
- 新增 `pipeline_impl.go:SetParam()` - 设置渲染后的 Param 值
- 更新 `config.example.yaml` - 添加 Metadata 渲染示例

### 文档更新

#### 1. README 大幅更新
- **README.md** - 添加参数模板渲染功能说明和完整示例
- **README_ZH.md** - 同步更新中文文档
- 添加模板渲染的使用场景和最佳实践
- 更新架构图，更清晰地展示组件关系

#### 2. 配置文档完善
- **doc/config.md** - 大幅更新（+66行）
- 添加完整的 Param 模板渲染章节
- 添加 Metadata 模板渲染说明
- 提供详细的语法示例和使用指南

#### 3. 架构图修改
- 优化架构图布局，更清晰展示各组件关系
- 更新英文和中文文档中的架构图

### 测试覆盖

- **runtime_test.go** - 新增 300 行测试代码
- 全面测试模板渲染功能
- 测试 Param 自引用和嵌套结构
- 测试 Metadata 引用 Param 的各种场景
- 测试错误处理和边界情况

### 改进与优化

#### 1. 配置增强
- 优化配置验证逻辑
- 完善模板渲染的错误提示
- 增强配置文件的灵活性

#### 2. 代码质量
- **runtime_impl.go** - 完善运行时实现（+184行）
- **eval_context.go** - 添加模板渲染支持
- 优化错误处理机制
- 提升代码可读性和可维护性

### 文件变更统计

- 新增文件：1 个（config.template-render.example.yaml）
- 修改文件：9 个（pipeline_impl.go、runtime_impl.go、runtime_test.go、eval_context.go、config.example.yaml、doc/config.md、README.md、README_ZH.md、LICENSE）
- 删除文件：1 个（example_test_config.yaml）
- 总行数变化：+1,400 / -70

### 完整提交记录

```
4791f86 feat: 更新README
af6cfd5 feat: 架构图修改
ed34fda feat: 添加Metadata中支持渲染Param
d37753f Update LICENSE
```

---

## [2026-03-14] - 2026-03-14

本版本引入了输出提取功能，完善了流水线执行逻辑，并更新了文档。

### 新增功能

#### 1. 输出提取功能（Output Extraction）
- 新增 `extractor.go` - 支持从命令输出中提取结构化数据
- `CodecBlockExtractor` - 自动识别 ```pipelinex-json 和 ```pipelinex-yaml 代码块
- `RegexExtractor` - 支持通过正则表达式提取输出信息
- 提取的数据可保存到 Pipeline 元数据中供后续节点使用

#### 2. 流水线执行逻辑完善
- `pipeline_impl.go` - 实现完整的 Pipeline 执行逻辑
- 支持多个入度为0的起始节点并行执行
- 完善执行状态管理和事件触发机制
- 优化并发控制和资源管理

### 文档更新

- 大幅更新 `README.md` - 添加输出提取功能说明和完整示例
- 新增 `README_ZH.md` - 完整的中文文档
- 更新配置示例，添加输出提取配置说明
- 完善 API 文档和使用示例

### 改进与优化

#### 配置增强
- 添加描述配置（Description Configuration）
- 完善执行器配置说明
- 优化配置文件结构

#### 项目管理
- 添加 GitNexus 配置，支持代码知识图谱分析
- 移除 `.claude` 配置文件
- 更新项目文档和开发指南

### 依赖更新

- 更新项目依赖，优化依赖管理

### 文件变更统计

- 新增文件：11 个（extractor 相关、README_ZH.md、配置更新）
- 修改文件：52 个（主要执行逻辑、文档、配置）
- 删除文件：0 个
- 总行数变化：+2,105 / -595

### 完整提交记录

```
efebadf feat: 去除.claude
c7990e7 feat: 添加描述配置
0e6ad1c feat: 添加从输出提取返回信息的功能
393eac7 feat: 添加gitnexus的配置
6a77d38 feat: 更新文档
29d8dcb feat: 流水线执行器支持local docker k8s 并且pipeline_impl实现了执行逻辑
```

---

## [2026-02-26] - 2026-02-26

本版本主要实现了 Docker 和 Local 执行器，同时简化了 Kubernetes 执行器的架构。

### 新增功能

#### 1. Docker 执行器
- 完整的 Docker 执行器实现 (`executor/docker/`)
- 支持容器生命周期管理（创建、启动、停止、删除）
- 支持实时日志收集和输出
- 支持执行终止功能
- 支持工作目录挂载和环境变量配置
- 提供完整的架构文档 (`executor/docker/README.md`)
- 包含完整的单元测试覆盖

#### 2. Local 执行器
- 完整的 Local 执行器实现 (`executor/local/`)
- 支持本地命令执行
- 支持执行终止功能
- 支持工作目录设置和环境变量配置
- 包含完整的单元测试覆盖

### 架构变更

#### Kubernetes 执行器重构
- **放弃 CRD 方式**：移除自定义资源定义和相关控制器
- **简化架构**：改为直接创建 Pod 的方式执行 Pipeline 节点
- **删除代码**：移除大量自动生成的 K8s 客户端代码（clientset、informers、listers 等）
- **目录重命名**：`executor/kubenetes/` → `executor/kubernetes/`

### 改进与优化

#### 代码组织
- 测试文件从 `test/` 目录移动到根目录，符合 Go 项目惯例
- 优化不必要公开的方法，改为私有方法
- Docker 执行器代码封装优化

### 文件变更统计

- 新增文件：13 个
- 修改文件：8 个
- 删除文件：37 个（主要为 K8s 自动生成代码）
- 总行数变化：+2,514 / -2,131

### 依赖更新

- 新增 Docker 客户端依赖
- 简化 Kubernetes 相关依赖

### 完整提交记录

```
20e6f1e feat: 封装优化
b53708c feat: 添加本地执行器，docker / local 都支持终止
8e105cb feat: k8s放弃使用CRD方式，直接创建pod
4d461a5 feat: 修改单元测试位置以及修改不必要公开的方法
4ee9b49 feat: 添加Docker执行器
```

---

## [2026-02-25]

本版本是 Pipeline 执行引擎的重大更新，引入了模板引擎、条件边执行、元数据管理等核心功能，同时大幅增强了并发安全性和可观测性。

### 新增功能

#### 1. 模板引擎支持
- 新增模板引擎接口 (`templete.go`, `templete_impl.go`)
- 支持基于文本的模板渲染，可用于动态生成节点配置
- 提供 `TemplateEngine` 接口，便于扩展不同的模板实现

#### 2. 条件边执行（Conditional Edge）
- 新增条件边功能 (`edge.go`, `edge_impl.go`)
- 支持基于表达式评估的边条件判断
- 添加 `EvalContext` 评估上下文 (`eval_context.go`)
- 支持根据节点执行结果动态决定执行路径

#### 3. 元数据管理（Metadata）
- 新增元数据系统 (`metadata.go`, `metadata_impl.go`)
- 支持在 Pipeline 执行过程中存储和传递元数据
- 提供进程安全的元数据读写操作

#### 4. Pipeline Runtime 重构
- 全新实现的 Pipeline Runtime (`runtime.go`, `runtime_impl.go`)
- 支持进程安全的并发执行
- 添加日志推送设置方法
- 支持 Pipeline 执行状态的实时监控

#### 5. 配置系统增强
- 重构配置相关逻辑到独立的 `config.go`
- 更新配置文件结构，支持执行器和多步骤配置
- 添加配置示例文件 (`config.example.yaml`)
- 支持节点状态信息持久化到配置

#### 6. 可视化增强
- 新增 Graph 文本绘图支持
- 支持在控制台输出 Pipeline 执行过程的可视化展示

#### 7. 执行器接口更新
- 重构执行器接口 (`executor.go`)
- 为不同类型的执行器（K8s、Docker、SSH、Local）提供更清晰的接口定义
- 完善 Kubernetes 执行器文档

### 改进与优化

#### 并发安全
- Pipeline Runtime 添加进程安全支持
- 使用 `sync.WaitGroup` 替代 errgroup 进行并发控制
- 修复入度为0的节点有多个时的调度问题

#### 错误处理
- 完善错误类型定义 (`err.go`)
- 添加更详细的错误信息和上下文

#### 日志系统
- 新增日志接口 (`logger.go`)
- 支持自定义日志推送

### Bug 修复

1. **修复死锁问题** - 修复了在特定并发场景下可能发生的死锁
2. **修复多起始节点问题** - 修复入度为0的节点有多个时的执行逻辑
3. **修复并发安全问题** - 修复 Pipeline 执行中的竞态条件

### 文档更新

- 新增 `CLAUDE.md` - 项目开发指南
- 新增 `doc/config.md` - 配置文件详细说明
- 新增 `doc/templete.md` - 模板引擎使用文档
- 更新 `README.md` - 项目介绍和快速开始
- 完善 Kubernetes 执行器文档

### 测试覆盖

新增大量单元测试：
- `test/conditional_edge_test.go` - 条件边功能测试
- `test/edge_test.go` - 边功能基础测试
- `test/eval_context_test.go` - 评估上下文测试
- `test/graph_edge_test.go` - 图边功能测试
- `test/runtime_test.go` - Pipeline Runtime 测试
- `test/templete_test.go` - 模板引擎测试
- 更新 `test/pipeline_test.go` - 基础 Pipeline 测试

### 依赖更新

- 更新 Go 版本要求至 1.23.0
- 更新 Kubernetes 相关依赖
- 移除未使用的 `golang.org/x/sync` 依赖

### 文件变更统计

- 新增文件：18 个
- 修改文件：17 个
- 删除文件：0 个
- 总行数变化：+3,839 / -116

### 迁移指南

从旧版本迁移到新版本时，请注意以下变更：

1. **配置文件格式更新** - 请参考 `config.example.yaml` 更新配置文件
2. **执行器接口变更** - 自定义执行器需要适配新的接口定义
3. **Pipeline 创建方式** - 建议使用新的 Runtime API 创建 Pipeline

---

**完整提交记录**：
```
33e9201 feat: 不使用errgroup直接使用WaitGroup代替
eda1f77 feat: 添加设置模板引擎
97cc674 feat: 添加state图的标签表达式读取功能
4745b3f feat：元数据支持
281c94c feat: 添加metadata支持
ee0e6eb feat: config相关的逻辑移动到config.go中
a54d377 feat: 添加模板引擎支持/条件边支持
fd7cf42 fix: 修复入度为0的节点有多个的情况
7fe6892 feat: pipeline_runtime 添加进程安全支持
726d4b0 feat: 添加配置示例
4dfd4f4 feat: 添加graph中文本绘图支持
dcd64a8 feat: 添加日志推送设置方法到runtime中
173dc13 feat: 修改更新配置文件结构，添加执行器和多步骤的功能
e38d371 feat: 修改执行器interface
ba0f601 feat: 修复死锁问题
dece094 feat: 配置文件添加节点状态信息
57105f4 feat: 添加单元测试
27fbc77 feat: 修改文档添加模板引擎选型和k8s执行器的实现目标
f1094ab feat: 常量设置
b999420 feat: 修改文件中的注释
909aa04 feat: 提阿娘CLAUDE的配置
```
