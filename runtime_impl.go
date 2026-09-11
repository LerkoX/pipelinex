package flowx

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/LerkoX/flowx/core"
	"github.com/LerkoX/flowx/dag"
	"github.com/LerkoX/flowx/executor/provider"
	"github.com/LerkoX/flowx/logger"
	"github.com/LerkoX/flowx/template"
)

// 预检查RuntimeImpl是否实现了Runtime接口
var _ Runtime = (*RuntimeImpl)(nil)

// RuntimeImpl Runtime接口的实现
type RuntimeImpl struct {
	workflows       map[string]dag.Workflow      // 存储所有流水线
	workflowIds     map[string]bool          // 跟踪所有使用过的流水线ID
	workflowConfigs map[string]*core.WorkflowConfig // 存储原始配置用于导出
	mu              sync.RWMutex             // 读写锁
	ctx             context.Context          // 上下文
	cancel          context.CancelFunc       // 取消函数
	doneChan        chan struct{}            // 完成通道
	background      chan struct{}            // 后台处理完成通道
	pusher          logger.Pusher            // 日志推送器
	templateEngine  template.TemplateEngine           // 模板引擎
}

// renderParam 渲染Param中的模板表达式，支持自引用
// 使用迭代方式处理参数间的相互引用，最大迭代次数防止无限循环
func (r *RuntimeImpl) renderParam(param map[string]core.FieldItem) (map[string]core.FieldItem, error) {
	if len(param) == 0 {
		return param, nil
	}

	// 创建结果副本，避免修改原始数据
	result := make(map[string]core.FieldItem)
	for k, v := range param {
		result[k] = v
	}

	// 最大迭代次数，防止无限循环
	maxIterations := 10
	changed := true
	iteration := 0

	for changed && iteration < maxIterations {
		changed = false
		iteration++

		// 遍历所有参数，尝试渲染
		for key, fieldItem := range result {
			// 创建上下文，Param的值可以直接访问，也可以通过Param.xxx访问
			// 从 FieldItem 中提取值用于上下文
			ctx := make(map[string]any)
			for k, v := range result {
				ctx[k] = core.GetValue(v.Value)
			}
			// 同时保留Param.xxx的访问方式
			paramValues := make(map[string]any)
			for k, v := range result {
				paramValues[k] = core.GetValue(v.Value)
			}
			ctx["Param"] = paramValues

			// 渲染 FieldItem.Value
			renderedValue, err := r.renderValue(core.GetValue(fieldItem.Value), ctx, 0)
			if err != nil {
				return nil, fmt.Errorf("failed to render param '%s': %w", key, err)
			}

			// 如果值发生变化，标记为需要继续迭代
			if !r.deepEqual(fieldItem.Value, renderedValue) {
				fieldItem.Value = renderedValue
				result[key] = fieldItem
				changed = true
			}
		}
	}

	// 如果达到最大迭代次数仍未稳定，说明可能存在循环引用
	if iteration >= maxIterations && changed {
		return result, fmt.Errorf("param rendering reached maximum iterations, possible circular reference detected")
	}

	return result, nil
}

// renderValue 递归渲染值中的模板表达式
// depth 参数控制递归深度，防止无限递归
func (r *RuntimeImpl) renderValue(value interface{}, ctx map[string]any, depth int) (interface{}, error) {
	// 限制递归深度
	if depth > 10 {
		return value, nil
	}

	switch v := value.(type) {
	case string:
		// 字符串类型，尝试渲染模板
		rendered, err := r.templateEngine.EvaluateString(v, ctx)
		if err != nil {
			// 渲染失败，返回原始值
			return v, nil
		}
		return rendered, nil

	case map[string]interface{}:
		// map类型，递归渲染每个值
		result := make(map[string]interface{})
		for k, val := range v {
			renderedVal, err := r.renderValue(val, ctx, depth+1)
			if err != nil {
				return nil, err
			}
			result[k] = renderedVal
		}
		return result, nil

	case []interface{}:
		// slice类型，递归渲染每个元素
		result := make([]interface{}, len(v))
		for i, val := range v {
			renderedVal, err := r.renderValue(val, ctx, depth+1)
			if err != nil {
				return nil, err
			}
			result[i] = renderedVal
		}
		return result, nil

	default:
		// 其他类型（数字、布尔值等），直接返回
		return value, nil
	}
}

// deepEqual 深度比较两个值是否相等
func (r *RuntimeImpl) deepEqual(a, b interface{}) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// renderMetadata 渲染Metadata中的模板表达式，可以引用Param
func (r *RuntimeImpl) renderMetadata(metadataData map[string]core.FieldItem, param map[string]core.FieldItem) (map[string]core.FieldItem, error) {
	if len(metadataData) == 0 {
		return metadataData, nil
	}

	// 构建上下文，Param可以通过{{ Param.xxx }}访问
	// 从 FieldItem 中提取值用于上下文
	paramValues := make(map[string]any)
	for k, v := range param {
		paramValues[k] = core.GetValue(v.Value)
	}
	ctx := map[string]any{
		"Param": paramValues,
	}

	// 渲染metadata数据
	result := make(map[string]core.FieldItem)
	for key, fieldItem := range metadataData {
		renderedValue, err := r.renderValue(core.GetValue(fieldItem.Value), ctx, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to render metadata '%s': %w", key, err)
		}
		fieldItem.Value = renderedValue
		result[key] = fieldItem
	}

	return result, nil
}

// renderConfig 渲染配置中所有引用 Param 的地方（配置阶段）
func (r *RuntimeImpl) renderConfig(config *core.WorkflowConfig) error {
	// 将 Param 从 map[string]interface{} 转换为 map[string]FieldItem
	paramFieldItem := make(map[string]core.FieldItem)
	for k, v := range config.Param {
		paramFieldItem[k] = core.ConvertToFieldItem(v)
	}

	// 构建 Param 上下文（用于渲染）
	ctx := map[string]any{}
	for k, v := range paramFieldItem {
		ctx[k] = core.GetValue(v.Value)
	}
	ctx["Param"] = ctx

	// 1. 渲染 Param 本身（支持自引用）
	if len(paramFieldItem) > 0 {
		renderedParam, err := r.renderParam(paramFieldItem)
		if err != nil {
			return fmt.Errorf("failed to render param: %w", err)
		}
		paramFieldItem = renderedParam
	}

	// 更新 config.Param 为渲染后的值
	config.Param = make(map[string]interface{})
	for k, v := range paramFieldItem {
		config.Param[k] = v.Value
	}

	// 将 Metadata.Data 从 map[string]interface{} 转换为 map[string]FieldItem
	metadataFieldItem := make(map[string]core.FieldItem)
	for k, v := range config.Metadate.Data {
		metadataFieldItem[k] = core.ConvertToFieldItem(v)
	}

	// 2. 渲染 Metadata
	if config.Metadate.Type != "" && len(metadataFieldItem) > 0 {
		renderedMetadata, err := r.renderMetadata(metadataFieldItem, paramFieldItem)
		if err != nil {
			return fmt.Errorf("failed to render metadata: %w", err)
		}
		metadataFieldItem = renderedMetadata
		// 将渲染后的 Metadata 更新到 config.Metadate.Data
		config.Metadate.Data = make(map[string]interface{})
		for k, v := range metadataFieldItem {
			config.Metadate.Data[k] = v.Value
		}
	}

	return nil
}

// NewRuntime 创建新的Runtime实例
func NewRuntime(ctx context.Context) Runtime {
	ctx, cancel := context.WithCancel(ctx)
	return &RuntimeImpl{
		workflows:       make(map[string]dag.Workflow),
		workflowIds:     make(map[string]bool),
		workflowConfigs: make(map[string]*core.WorkflowConfig),
		ctx:             ctx,
		cancel:          cancel,
		doneChan:        make(chan struct{}),
		background:      make(chan struct{}),
		templateEngine:  template.NewPongo2TemplateEngine(), // 默认引擎
	}
}

// Get 获取流水线状态
func (r *RuntimeImpl) Get(id string) (dag.Workflow, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	workflow, exists := r.workflows[id]
	if !exists {
		return nil, fmt.Errorf("workflow with id %s not found", id)
	}
	return workflow, nil
}

// Cancel 取消运行中的流水线
func (r *RuntimeImpl) Cancel(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	workflow, exists := r.workflows[id]
	if !exists {
		return fmt.Errorf("workflow with id %s not found", id)
	}

	// 调用流水线的Cancel方法
	if p, ok := workflow.(*dag.WorkflowImpl); ok {
		p.Cancel()
	}

	return nil
}

// RunAsync 执行异步流水线（完成后实例即从 Runtime 删除）
func (r *RuntimeImpl) RunAsync(ctx context.Context, id string, config string, listener dag.Listener) (dag.Workflow, error) {
	workflow, err := r.prepareWorkflow(ctx, id, config, listener)
	if err != nil {
		return nil, err
	}

	// 异步执行流水线
	go func() {
		defer func() {
			r.mu.Lock()
			delete(r.workflows, id)
			r.mu.Unlock()
		}()

		if err := workflow.Run(ctx); err != nil {
			fmt.Printf("dag.Workflow %s execution failed: %v\n", id, err)
		}
	}()

	return workflow, nil
}

// LoadWorkflow 加载流水线配置但不运行。
// 配置中携带的节点运行时状态（ExportConfig 导出的快照 YAML）会被恢复，
// 并据节点状态推导流水线状态（FAILED > STOPPED > SUCCESS），使实例处于可修改状态；
// 之后可通过 ModifyGraph/UpdateConfig 修改图，用 Rerun 继续运行（已终结节点跳过）。
// 不再使用时调用 Rm(id) 释放。
func (r *RuntimeImpl) LoadWorkflow(ctx context.Context, id string, config string, listener dag.Listener) (dag.Workflow, error) {
	workflow, err := r.prepareWorkflow(ctx, id, config, listener)
	if err != nil {
		return nil, err
	}
	if impl, ok := workflow.(*dag.WorkflowImpl); ok {
		impl.DeriveStatusFromNodes()
	}
	return workflow, nil
}

// prepareWorkflow 解析配置、构建图（含运行时状态恢复）、注册实例，但不启动执行
func (r *RuntimeImpl) prepareWorkflow(ctx context.Context, id string, config string, listener dag.Listener) (dag.Workflow, error) {
	// 提前获取 templateEngine，避免在持有写锁时调用 GetTemplateEngine 导致死锁
	templateEngine := r.GetTemplateEngine()

	r.mu.Lock()
	defer r.mu.Unlock()

	// 检查是否已存在相同ID的流水线
	if _, exists := r.workflowIds[id]; exists {
		return nil, fmt.Errorf("workflow with id %s already exists", id)
	}

	// 解析配置
	workflowConfig, err := r.parseConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// 统一渲染配置中所有引用 Param 的地方
	if err := r.renderConfig(workflowConfig); err != nil {
		return nil, fmt.Errorf("failed to render config: %w", err)
	}

	// 创建流水线：使用注册 ID（宿主传入的稳定身份，如 exec-<执行ID>）作为
	// 流水线实例 ID，保证同一执行的 RunAsync/LoadWorkflow/Rerun 保持一致
	workflow := dag.NewWorkflowWithId(ctx, id)
	workflow.SetTemplateEngine(templateEngine)
	workflow.SetPusher(r.pusher)
	workflow.SetPusher(r.pusher)

	// 设置监听器
	if listener != nil {
		workflow.Listening(listener)
	}

	// 构建图结构
	graph := r.buildGraph(workflowConfig)
	workflow.SetGraph(graph)

	// 设置渲染后的 param 值
	if len(workflowConfig.Param) > 0 {
		workflow.(*dag.WorkflowImpl).SetParam(workflowConfig.Param)
	}

	// 设置循环图最大迭代次数
	if workflowConfig.MaxLoopIterations > 0 {
		workflow.(*dag.WorkflowImpl).SetMaxLoopIterations(workflowConfig.MaxLoopIterations)
	}

	// 设置metadata
	if err := r.setupMetadata(ctx, workflow, workflowConfig); err != nil {
		return nil, fmt.Errorf("failed to setup metadata: %w", err)
	}

	// 创建并配置执行器提供者
	execProvider := provider.NewProvider()
	for name, execConfig := range workflowConfig.Executors {
		execProvider.RegisterExecutor(name, provider.ExecutorConfig{
			Type:   execConfig.Type,
			Config: execConfig.Config,
		})
	}
	workflow.SetExecutorProvider(execProvider)

	// 存储流水线并标记ID为已使用
	r.workflows[id] = workflow
	r.workflowIds[id] = true
	r.workflowConfigs[id] = workflowConfig

	return workflow, nil
}

// Rerun 重新运行已完成且被保留的流水线（配合 RunAsyncRetained 使用）。
// 已终结状态（SUCCESS/FAILED/CANCELLED）的节点按运行时状态跳过，
// 仅执行新增或尚未运行的节点；通常先通过 ModifyGraph/UpdateConfig 修改图。
func (r *RuntimeImpl) Rerun(ctx context.Context, id string) error {
	r.mu.RLock()
	workflow, exists := r.workflows[id]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("workflow with id %s not found (not retained or already removed)", id)
	}
	if !workflow.IsModifiable() {
		return core.ErrWorkflowRunning
	}

	go func() {
		if err := workflow.Run(ctx); err != nil {
			fmt.Printf("dag.Workflow %s re-run failed: %v\n", id, err)
		}
	}()
	return nil
}

// RunSync 执行同步流水线
func (r *RuntimeImpl) RunSync(ctx context.Context, id string, config string, listener dag.Listener) (dag.Workflow, error) {
	// 检查是否已存在相同ID的流水线
	r.mu.Lock()
	if _, exists := r.workflowIds[id]; exists {
		r.mu.Unlock()
		return nil, fmt.Errorf("workflow with id %s already exists", id)
	}
	r.workflowIds[id] = true
	r.mu.Unlock()

	// 解析配置
	workflowConfig, err := r.parseConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// 统一渲染配置中所有引用 Param 的地方
	if err := r.renderConfig(workflowConfig); err != nil {
		return nil, fmt.Errorf("failed to render config: %w", err)
	}

	// 创建流水线：使用注册 ID（宿主传入的稳定身份，如 exec-<执行ID>）作为
	// 流水线实例 ID，保证同一执行的 RunAsync/LoadWorkflow/Rerun 保持一致
	workflow := dag.NewWorkflowWithId(ctx, id)
	workflow.SetTemplateEngine(r.GetTemplateEngine())
	workflow.SetPusher(r.pusher)
	workflow.SetPusher(r.pusher)

	// 设置监听器
	if listener != nil {
		workflow.Listening(listener)
	}

	// 构建图结构
	graph := r.buildGraph(workflowConfig)
	workflow.SetGraph(graph)

	// 设置渲染后的 param 值
	if len(workflowConfig.Param) > 0 {
		workflow.(*dag.WorkflowImpl).SetParam(workflowConfig.Param)
	}

	// 设置循环图最大迭代次数
	if workflowConfig.MaxLoopIterations > 0 {
		workflow.(*dag.WorkflowImpl).SetMaxLoopIterations(workflowConfig.MaxLoopIterations)
	}

	// 设置metadata
	if err := r.setupMetadata(ctx, workflow, workflowConfig); err != nil {
		return nil, fmt.Errorf("failed to setup metadata: %w", err)
	}

	// 创建并配置执行器提供者
	execProvider := provider.NewProvider()
	for name, execConfig := range workflowConfig.Executors {
		execProvider.RegisterExecutor(name, provider.ExecutorConfig{
			Type:   execConfig.Type,
			Config: execConfig.Config,
		})
	}
	workflow.SetExecutorProvider(execProvider)

	// 存储流水线
	r.mu.Lock()
	r.workflows[id] = workflow
	r.workflowConfigs[id] = workflowConfig
	r.mu.Unlock()

	err = workflow.Run(ctx)
	if err != nil {
		return nil, fmt.Errorf("workflow execution failed: %w", err)
	}

	// 清理已完成的流水线，但保留ID记录
	r.mu.Lock()
	delete(r.workflows, id)
	r.mu.Unlock()

	return workflow, nil
}

// Rm 移除流水线记录
func (r *RuntimeImpl) Rm(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.workflows, id)
	delete(r.workflowConfigs, id)
	delete(r.workflowIds, id)
}

// Done runtime已经执行完成
func (r *RuntimeImpl) Done() chan struct{} {
	return r.doneChan
}

// Notify 通知runtime
func (r *RuntimeImpl) Notify(data interface{}) error {
	// 这里可以根据data的内容进行不同的处理
	// 例如：更新流水线状态、触发事件等
	switch v := data.(type) {
	case string:
		fmt.Printf("Runtime notification: %s\n", v)
	case map[string]interface{}:
		if msg, ok := v["message"].(string); ok {
			fmt.Printf("Runtime notification: %s\n", msg)
		}
	default:
		fmt.Printf("Runtime notification: %+v\n", v)
	}
	return nil
}

// Ctx 返回runtime公共上下文
func (r *RuntimeImpl) Ctx() context.Context {
	return r.ctx
}

// StopBackground 停止后台处理
func (r *RuntimeImpl) StopBackground() {
	r.cancel()
	select {
	case <-r.doneChan:
		// Channel already closed
	default:
		close(r.doneChan)
	}
}

// StartBackground 启动后台处理
func (r *RuntimeImpl) StartBackground() {
	go func() {
		defer close(r.background)

		ticker := time.NewTicker(30 * time.Second) // 每30秒检查一次
		defer ticker.Stop()

		for {
			select {
			case <-r.ctx.Done():
				return
			case <-ticker.C:
				// 定期清理已完成的流水线
				r.cleanupCompletedWorkflows()
			}
		}
	}()
}

// cleanupCompletedWorkflows 清理已完成的流水线
func (r *RuntimeImpl) cleanupCompletedWorkflows() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, workflow := range r.workflows {
		select {
		case <-workflow.Done():
			// 流水线已完成，可以清理
			delete(r.workflows, id)
		default:
			// 流水线仍在运行
		}
	}
}

// SetPusher 设置日志推送器
func (r *RuntimeImpl) SetPusher(pusher logger.Pusher) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pusher = pusher
}

// SetTemplateEngine 设置模板引擎
func (r *RuntimeImpl) SetTemplateEngine(engine template.TemplateEngine) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.templateEngine = engine
}

// getTemplateEngine 获取当前使用的模板引擎（内部使用）
func (r *RuntimeImpl) getTemplateEngine() template.TemplateEngine {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.templateEngine == nil {
		return template.NewPongo2TemplateEngine()
	}
	return r.templateEngine
}

// GetTemplateEngine 获取当前使用的模板引擎
func (r *RuntimeImpl) GetTemplateEngine() template.TemplateEngine {
	return r.getTemplateEngine()
}

// ExportConfig 导出流水线的运行时配置
// 返回包含当前运行时状态的 YAML 格式配置字符串
func (r *RuntimeImpl) ExportConfig(id string) (string, error) {
	r.mu.RLock()
	workflow, exists := r.workflows[id]
	config, configExists := r.workflowConfigs[id]
	r.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("workflow with id %s not found", id)
	}

	if !configExists {
		return "", fmt.Errorf("config for workflow %s not found", id)
	}

	// 使用 Snapshotter 生成带状态的配置
	snapshotter := dag.NewWorkflowSnapshotter()
	snapshotConfig, err := snapshotter.TakeSnapshot(workflow, config)
	if err != nil {
		return "", fmt.Errorf("failed to take snapshot: %w", err)
	}

	// 转换为 YAML
	yamlStr, err := snapshotter.ToYAML(snapshotConfig)
	if err != nil {
		return "", fmt.Errorf("failed to convert to YAML: %w", err)
	}

	return yamlStr, nil
}

// ListWorkflows 列出所有活跃的流水线ID
func (r *RuntimeImpl) ListWorkflows() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]string, 0, len(r.workflows))
	for id := range r.workflows {
		result = append(result, id)
	}
	return result
}

// Pause 暂停运行中的流水线
func (r *RuntimeImpl) Pause(ctx context.Context, id string) error {
	r.mu.RLock()
	workflow, exists := r.workflows[id]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("workflow with id %s not found", id)
	}

	return workflow.Pause()
}

// Resume 恢复暂停或停止的流水线
func (r *RuntimeImpl) Resume(ctx context.Context, id string) error {
	r.mu.RLock()
	workflow, exists := r.workflows[id]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("workflow with id %s not found", id)
	}

	return workflow.Resume(ctx)
}