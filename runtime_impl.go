package pipelinex

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/chenyingqiao/pipelinex/executor/provider"
	"github.com/chenyingqiao/pipelinex/logger"
	"github.com/tetrafolium/mermaid-check/ast"
	"github.com/tetrafolium/mermaid-check/parser"
	"gopkg.in/yaml.v2"
)

// 预检查RuntimeImpl是否实现了Runtime接口
var _ Runtime = (*RuntimeImpl)(nil)

// RuntimeImpl Runtime接口的实现
type RuntimeImpl struct {
	pipelines      map[string]Pipeline // 存储所有流水线
	pipelineIds    map[string]bool     // 跟踪所有使用过的流水线ID
	mu             sync.RWMutex        // 读写锁
	ctx            context.Context     // 上下文
	cancel         context.CancelFunc  // 取消函数
	doneChan       chan struct{}       // 完成通道
	background     chan struct{}       // 后台处理完成通道
	pusher         logger.Pusher        // 日志推送器
	templateEngine TemplateEngine      // 模板引擎
}

// renderParam 渲染Param中的模板表达式，支持自引用
// 使用迭代方式处理参数间的相互引用，最大迭代次数防止无限循环
func (r *RuntimeImpl) renderParam(param map[string]interface{}) (map[string]interface{}, error) {
	if len(param) == 0 {
		return param, nil
	}

	// 创建结果副本，避免修改原始数据
	result := make(map[string]interface{})
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
		for key, value := range result {
			// 创建上下文，Param的值可以直接访问，也可以通过Param.xxx访问
			ctx := make(map[string]any)
			// 将所有Param值直接放入上下文，使其可以直接访问
			for k, v := range result {
				ctx[k] = v
			}
			// 同时保留Param.xxx的访问方式
			ctx["Param"] = result

			renderedValue, err := r.renderValue(value, ctx, 0)
			if err != nil {
				return nil, fmt.Errorf("failed to render param '%s': %w", key, err)
			}

			// 如果值发生变化，标记为需要继续迭代
			if !r.deepEqual(value, renderedValue) {
				result[key] = renderedValue
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
func (r *RuntimeImpl) renderMetadata(metadataData map[string]interface{}, param map[string]interface{}) (map[string]interface{}, error) {
	if len(metadataData) == 0 {
		return metadataData, nil
	}

	// 构建上下文，Param可以通过{{ Param.xxx }}访问
	ctx := map[string]any{
		"Param": param,
	}

	// 渲染metadata数据
	result := make(map[string]interface{})
	for key, value := range metadataData {
		renderedValue, err := r.renderValue(value, ctx, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to render metadata '%s': %w", key, err)
		}
		result[key] = renderedValue
	}

	return result, nil
}

// NewRuntime 创建新的Runtime实例
func NewRuntime(ctx context.Context) Runtime {
	ctx, cancel := context.WithCancel(ctx)
	return &RuntimeImpl{
		pipelines:       make(map[string]Pipeline),
		pipelineIds:     make(map[string]bool),
		ctx:             ctx,
		cancel:          cancel,
		doneChan:        make(chan struct{}),
		background:      make(chan struct{}),
		templateEngine:  NewPongo2TemplateEngine(), // 默认引擎
	}
}

// Get 获取流水线状态
func (r *RuntimeImpl) Get(id string) (Pipeline, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	pipeline, exists := r.pipelines[id]
	if !exists {
		return nil, fmt.Errorf("pipeline with id %s not found", id)
	}
	return pipeline, nil
}

// Cancel 取消运行中的流水线
func (r *RuntimeImpl) Cancel(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	pipeline, exists := r.pipelines[id]
	if !exists {
		return fmt.Errorf("pipeline with id %s not found", id)
	}

	// 调用流水线的Cancel方法
	if p, ok := pipeline.(*PipelineImpl); ok {
		p.Cancel()
	}

	return nil
}

// RunAsync 执行异步流水线
func (r *RuntimeImpl) RunAsync(ctx context.Context, id string, config string, listener Listener) (Pipeline, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 检查是否已存在相同ID的流水线
	if _, exists := r.pipelineIds[id]; exists {
		return nil, fmt.Errorf("pipeline with id %s already exists", id)
	}

	// 解析配置
	pipelineConfig, err := r.parseConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// 渲染Param
	if len(pipelineConfig.Param) > 0 {
		renderedParam, err := r.renderParam(pipelineConfig.Param)
		if err != nil {
			return nil, fmt.Errorf("failed to render param: %w", err)
		}
		pipelineConfig.Param = renderedParam
	}

	// 渲染Metadata
	if pipelineConfig.Metadate.Type != "" && len(pipelineConfig.Metadate.Data) > 0 {
		renderedMetadata, err := r.renderMetadata(pipelineConfig.Metadate.Data, pipelineConfig.Param)
		if err != nil {
			return nil, fmt.Errorf("failed to render metadata: %w", err)
		}
		pipelineConfig.Metadate.Data = renderedMetadata
	}

	// 创建流水线
	pipeline := NewPipeline(ctx)

	// 设置监听器
	if listener != nil {
		pipeline.Listening(listener)
	}

	// 构建图结构
	graph := r.buildGraph(pipelineConfig)
	pipeline.SetGraph(graph)

	// 设置渲染后的 param 值
	if len(pipelineConfig.Param) > 0 {
		pipeline.(*PipelineImpl).SetParam(pipelineConfig.Param)
	}

	// 设置metadata
	if err := r.setupMetadata(ctx, pipeline, pipelineConfig); err != nil {
		return nil, fmt.Errorf("failed to setup metadata: %w", err)
	}

	// 创建并配置执行器提供者
	execProvider := provider.NewProvider()
	for name, execConfig := range pipelineConfig.Executors {
		execProvider.RegisterExecutor(name, provider.ExecutorConfig{
			Type:   execConfig.Type,
			Config: execConfig.Config,
		})
	}
	pipeline.SetExecutorProvider(execProvider)

	// 存储流水线并标记ID为已使用
	r.pipelines[id] = pipeline
	r.pipelineIds[id] = true

	// 异步执行流水线
	go func() {
		defer func() {
			r.mu.Lock()
			delete(r.pipelines, id)
			r.mu.Unlock()
		}()

		if err := pipeline.Run(ctx); err != nil {
			fmt.Printf("Pipeline %s execution failed: %v\n", id, err)
		}
	}()

	return pipeline, nil
}

// RunSync 执行同步流水线
func (r *RuntimeImpl) RunSync(ctx context.Context, id string, config string, listener Listener) (Pipeline, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 检查是否已存在相同ID的流水线
	if _, exists := r.pipelineIds[id]; exists {
		return nil, fmt.Errorf("pipeline with id %s already exists", id)
	}

	// 解析配置
	pipelineConfig, err := r.parseConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// 渲染Param
	if len(pipelineConfig.Param) > 0 {
		renderedParam, err := r.renderParam(pipelineConfig.Param)
		if err != nil {
			return nil, fmt.Errorf("failed to render param: %w", err)
		}
		pipelineConfig.Param = renderedParam
	}

	// 渲染Metadata
	if pipelineConfig.Metadate.Type != "" && len(pipelineConfig.Metadate.Data) > 0 {
		renderedMetadata, err := r.renderMetadata(pipelineConfig.Metadate.Data, pipelineConfig.Param)
		if err != nil {
			return nil, fmt.Errorf("failed to render metadata: %w", err)
		}
		pipelineConfig.Metadate.Data = renderedMetadata
	}

	// 创建流水线
	pipeline := NewPipeline(ctx)

	// 设置监听器
	if listener != nil {
		pipeline.Listening(listener)
	}

	// 构建图结构
	graph := r.buildGraph(pipelineConfig)
	pipeline.SetGraph(graph)

	// 设置渲染后的 param 值
	if len(pipelineConfig.Param) > 0 {
		pipeline.(*PipelineImpl).SetParam(pipelineConfig.Param)
	}

	// 设置metadata
	if err := r.setupMetadata(ctx, pipeline, pipelineConfig); err != nil {
		return nil, fmt.Errorf("failed to setup metadata: %w", err)
	}

	// 创建并配置执行器提供者
	execProvider := provider.NewProvider()
	for name, execConfig := range pipelineConfig.Executors {
		execProvider.RegisterExecutor(name, provider.ExecutorConfig{
			Type:   execConfig.Type,
			Config: execConfig.Config,
		})
	}
	pipeline.SetExecutorProvider(execProvider)

	// 存储流水线并标记ID为已使用
	r.pipelines[id] = pipeline
	r.pipelineIds[id] = true

	err = pipeline.Run(ctx)
	if err != nil {
		return nil, fmt.Errorf("pipeline execution failed: %w", err)
	}

	// 清理已完成的流水线，但保留ID记录
	delete(r.pipelines, id)

	return pipeline, nil
}

// Rm 移除流水线记录
func (r *RuntimeImpl) Rm(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.pipelines, id)
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

// setupMetadata 设置流水线的metadata
func (r *RuntimeImpl) setupMetadata(ctx context.Context, pipeline Pipeline, config *PipelineConfig) error {
	// 检查是否有metadata配置（注意配置中是Metadate）
	if config.Metadate.Type == "" {
		return nil
	}

	// 创建metadata store
	factory := NewMetadataStoreFactory()
	store, err := factory.Create(config.Metadate)
	if err != nil {
		return fmt.Errorf("failed to create metadata store: %w", err)
	}

	// 设置到pipeline
	pipeline.SetMetadata(store)
	return nil
}

// parseConfig 解析流水线配置
func (r *RuntimeImpl) parseConfig(config string) (*PipelineConfig, error) {
	var pipelineConfig PipelineConfig

	err := yaml.Unmarshal([]byte(config), &pipelineConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml config: %w", err)
	}

	return &pipelineConfig, nil
}

// buildGraph 构建图结构
func (r *RuntimeImpl) buildGraph(config *PipelineConfig) Graph {
	graph := NewDGAGraph()

	// 创建节点
	nodeMap := make(map[string]Node)
	for nodeName, nodeConfig := range config.Nodes {
		// 初始状态：如果有 runtime 则用 runtime 的 status，否则用 StatusUnknown
		initialStatus := StatusUnknown
		if nodeConfig.Runtime != nil && nodeConfig.Runtime.Status != "" {
			initialStatus = nodeConfig.Runtime.Status
		}

		// 确保步骤有ID
		for i := range nodeConfig.Steps {
			if nodeConfig.Steps[i].Id == "" {
				nodeConfig.Steps[i].Id = NewUUID()
			}
		}

		node := NewDGANodeWithConfig(
			nodeName,
			initialStatus,
			nodeConfig.Executor,
			nodeConfig.Image,
			nodeConfig.Steps,
			nodeConfig.Config,
		)

		// 恢复运行时状态
		if nodeConfig.Runtime != nil {
			node.SetRuntimeStatus(nodeConfig.Runtime)
		}

		// 确保节点有ID
		node.EnsureIds()

		nodeMap[nodeName] = node
		graph.AddVertex(node)
	}

	// 解析图关系并添加边
	if config.Graph != "" {
		r.parseGraphEdges(graph, nodeMap, config.Graph)
	}

	return graph
}

// SetPipelineParam 设置 pipeline 的 param 值（内部使用）
func SetPipelineParam(pipeline Pipeline, param map[string]interface{}) {
	if pipelineImpl, ok := pipeline.(*PipelineImpl); ok {
		pipelineImpl.SetParam(param)
	}
}

// parseGraphEdges 解析图边关系
// 使用 mermaid-check 库解析 stateDiagram-v2 语法
// 支持从边标签中解析条件表达式，例如：A --> B: label[{eq .Param}]
func (r *RuntimeImpl) parseGraphEdges(graph Graph, nodeMap map[string]Node, graphStr string) {
	stateParser := parser.NewStateParser()
	diagram, err := stateParser.Parse(graphStr)
	if err != nil {
		// 解析失败时静默返回，不建立边关系
		return
	}

	// 转换为状态图
	stateDiagram, ok := diagram.(*ast.StateDiagram)
	if !ok {
		return
	}

	// 遍历所有语句，提取转换关系
	for _, stmt := range stateDiagram.Statements {
		// 尝试转换为 Transition
		if transition, ok := stmt.(*ast.Transition); ok {
			// 跳过 [*] 开始/结束节点
			if transition.From == "[*]" || transition.To == "[*]" {
				continue
			}

			srcNode, srcExists := nodeMap[transition.From]
			destNode, destExists := nodeMap[transition.To]

			if !srcExists || !destExists {
				continue
			}

			// 从 Label 中提取条件表达式
			expression := r.extractExpression(transition.Label)

			// 添加边关系（有条件表达式则创建条件边）
			var edge Edge
			if expression != "" {
				edge = NewConditionalEdge(srcNode, destNode, expression)
			} else {
				edge = NewDGAEdge(srcNode, destNode)
			}
			_ = graph.AddEdge(edge)
		}
	}
}

// ExtractExpression 从边标签中提取条件表达式（公共函数供测试使用）
// 使用模板引擎的 Validate 方法验证表达式语法
// 先检查是否包含模板标记 {{ 或 {%，再使用模板引擎验证
func ExtractExpression(label string) string {
	if label == "" {
		return ""
	}

	// 检查是否包含模板表达式标记 {{ 或 {%
	if !strings.Contains(label, "{{") && !strings.Contains(label, "{%") {
		return ""
	}

	// 使用模板引擎验证表达式语法
	engine := NewPongo2TemplateEngine()
	if err := engine.Validate(label); err == nil {
		return label
	}
	return ""
}

// extractExpression 从边标签中提取条件表达式（内部使用）
// 使用模板引擎的Validate方法验证label是否是有效的模板表达式
func (r *RuntimeImpl) extractExpression(label string) string {
	if label == "" {
		return ""
	}

	// 检查是否包含模板表达式标记 {{ 或 {%
	if !strings.Contains(label, "{{") && !strings.Contains(label, "{%") {
		return ""
	}

	engine := r.getTemplateEngine()

	// 使用模板引擎验证label是否是有效的模板表达式
	if err := engine.Validate(label); err == nil {
		return label
	}
	return ""
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
				r.cleanupCompletedPipelines()
			}
		}
	}()
}

// cleanupCompletedPipelines 清理已完成的流水线
func (r *RuntimeImpl) cleanupCompletedPipelines() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, pipeline := range r.pipelines {
		select {
		case <-pipeline.Done():
			// 流水线已完成，可以清理
			delete(r.pipelines, id)
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
func (r *RuntimeImpl) SetTemplateEngine(engine TemplateEngine) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.templateEngine = engine
}

// getTemplateEngine 获取当前使用的模板引擎（内部使用）
func (r *RuntimeImpl) getTemplateEngine() TemplateEngine {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.templateEngine == nil {
		return NewPongo2TemplateEngine()
	}
	return r.templateEngine
}


