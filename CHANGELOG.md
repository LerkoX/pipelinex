# 更新日志

## [Unreleased] - 2026-04-30

- 新增完整的示例代码：`examples/workflows/`（数据ETL、文件处理、天气飞书通知）
- 新增测试用例配置：`test/fixtures/runtime/`（18个配置文件，覆盖数据传递、并行执行、运行时恢复等场景）
- 增强节点间数据传递支持，通过 `extract` 字段提取数据并在后续节点使用
- 完善并行节点执行和步骤级别的运行时恢复功能
- 优化执行器信息追踪（Kubernetes、Local）
- 修复步骤级别恢复中的索引映射错误

---

## [Unreleased] - 2026-03-22

- 新增 Pipeline 运行时状态持久化功能
- 新增控制台日志推送
- 增强 Metadata 线程安全性和渲染功能
- 新增执行器信息追踪（Docker、Kubernetes、Local）
- 新增可执行程序支持（`bin/main.go`）
- 优化配置解析和错误处理
- 大幅提升测试覆盖率

---

## [Unreleased] - 2026-03-15

- 新增参数模板渲染功能，支持 Param 自引用和嵌套结构
- Metadata 支持引用 Param
- 更新 README 和配置文档

---

## [2026-03-14] - 2026-03-14

- 新增输出提取功能（CodecBlockExtractor、RegexExtractor）
- 完善 Pipeline 执行逻辑
- 新增 README_ZH.md 中文文档
- 添加描述配置和 GitNexus 支持

---

## [2026-02-26] - 2026-02-26

- 新增 Docker 执行器
- 新增 Local 执行器
- Kubernetes 执行器重构：放弃 CRD 方式，改为直接创建 Pod
- 优化代码组织

---

## [2026-02-25]

- 新增模板引擎支持
- 新增条件边执行功能
- 新增元数据管理系统
- Pipeline Runtime 重构，支持进程安全的并发执行
- 配置文件系统增强
- 新增 Graph 文本绘图支持
- 修复死锁和多起始节点问题
- 大幅提升测试覆盖率
