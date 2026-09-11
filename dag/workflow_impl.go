package dag

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/LerkoX/flowx/core"
	"github.com/LerkoX/flowx/executor"
	"github.com/LerkoX/flowx/logger"
	"github.com/LerkoX/flowx/metadata"
	"github.com/LerkoX/flowx/template"
)

type WorkflowImpl struct {
	id               string
	graph            Graph
	status           string
	metadata         Metadata
	metadataStore    metadata.MetadataStore
	listening        ListeningFn
	listener         Listener
	doneChan         chan struct{}
	cancelFunc       context.CancelFunc
	mu               sync.RWMutex
	executorProvider ExecutorProvider
	executors        map[string]executor.Executor    // 缓存已创建的executor
	param            map[string]core.FieldItem // 存储渲染后的Param值
	templateEngine   template.TemplateEngine         // 模板引擎
	cleanupOnce      sync.Once              // 保护清理操作只执行一次
	pauseMu          sync.Mutex             // 保护暂停/恢复操作的序列化
	pauseCond        *sync.Cond             // 暂停/恢复条件变量
	currentLevel     int                    // 记录当前执行到的BFS层级（用于暂停恢复）
	maxLoopIter      int                    // 循环图最大迭代次数
	pusher           logger.Pusher           // 日志推送器
	currentNode      Node                   // 当前正在执行的节点
}

func NewWorkflow(ctx context.Context) Workflow {
	return NewWorkflowWithId(ctx, "")
}

// NewWorkflowWithId 使用指定 ID 创建流水线；id 为空时退回随机 UUID。
// 宿主（如 flowx-studio）可传入稳定的外部身份（执行实例 ID），使同一执行的
// 续跑/重入保持同一 FLOWX_WORKFLOW_ID，节点可据此做按执行隔离的状态持久化。
func NewWorkflowWithId(ctx context.Context, id string) Workflow {
	if id == "" {
		id = core.NewUUID()
	}
	p := &WorkflowImpl{
		id:          id,
		executors:   make(map[string]executor.Executor),
		doneChan:    make(chan struct{}),
		maxLoopIter: 100, // 默认最大迭代次数
	}
	p.pauseCond = sync.NewCond(&p.pauseMu)
	return p
}

// 预检查WorkflowImpl是否实现了Workflow接口
var _ Workflow = (*WorkflowImpl)(nil)
var _ Graph = (*DGAGraph)(nil)

// SetParam 设置 param 值
func (p *WorkflowImpl) SetParam(param map[string]interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// 将 map[string]interface{} 转换为 map[string]FieldItem
	p.param = make(map[string]core.FieldItem)
	for k, v := range param {
		p.param[k] = core.ConvertToFieldItem(v)
	}
}

// GetParam 获取渲染后的 param 值
func (p *WorkflowImpl) GetParam() Metadata {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make(Metadata, len(p.param))
	for k, v := range p.param {
		result[k] = v
	}
	return result
}

// SetMaxLoopIterations 设置循环图最大迭代次数
func (p *WorkflowImpl) SetMaxLoopIterations(max int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if max > 0 {
		p.maxLoopIter = max
	}
}

// Id 返回流水线的ID
func (p *WorkflowImpl) Id() string {
	return p.id
}

// GetGraph 返回流水线的图结构
func (p *WorkflowImpl) GetGraph() Graph {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.graph
}

// SetGraph 设置流水线的图结构
func (p *WorkflowImpl) SetGraph(graph Graph) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.graph = graph
}

// Status 返回流水线的整体状态
func (p *WorkflowImpl) Status() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.status
}

// SetMetadata 设置流水线的元数据存储
func (p *WorkflowImpl) SetMetadata(store metadata.MetadataStore) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.metadataStore = store
}

// Metadata 获取流水线的元数据
func (p *WorkflowImpl) Metadata() Metadata {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.metadata == nil {
		p.metadata = make(Metadata)
	}

	// 如果有 metadataStore，从 store 加载数据
	if p.metadataStore != nil {
		// 从 metadata.InConfigMetadataStore 加载所有数据
		if inConfigStore, ok := p.metadataStore.(*metadata.InConfigMetadataStore); ok {
			for k, v := range inConfigStore.GetAll() {
				// 将 string 值转换为 FieldItem
				p.metadata[k] = core.FieldItem{
					Value:       v,
					Description: "",
					SrcNode:     "",
				}
			}
		}
	}

	// 返回元数据的拷贝，避免并发修改问题
	result := make(Metadata)
	for k, v := range p.metadata {
		result[k] = v
	}
	return result
}

// Listening 设置流水线执行事件监听器
func (p *WorkflowImpl) Listening(fn Listener) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.listener = fn
}

// Done 返回一个通道，用于通知流水线何时完成
func (p *WorkflowImpl) Done() <-chan struct{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.doneChan
}

// shouldSkipNode 检查节点是否应该跳过执行
func (p *WorkflowImpl) shouldSkipNode(node Node) bool {
	runtimeStatus := node.GetRuntimeStatus()
	if runtimeStatus == nil {
		return false
	}
	// SUCCESS、FAILED、CANCELLED 状态的节点都跳过
	// RUNNING 状态的节点需要恢复（不跳过）
	switch runtimeStatus.Status {
	case core.StatusSuccess, core.StatusFailed, core.StatusCancelled:
		return true
	default:
		return false
	}
}

// DeriveStatusFromNodes 根据节点运行时状态推导流水线状态。
// 用于加载携带运行时状态的快照配置（LoadWorkflow）后恢复可修改状态：
// 有 FAILED 节点 → FAILED；有未终结节点 → STOPPED；全部成功 → SUCCESS。
func (p *WorkflowImpl) DeriveStatusFromNodes() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == core.StatusRunning || p.graph == nil {
		return
	}

	hasFailed, hasIncomplete, hasSuccess := false, false, false
	for _, node := range p.graph.Nodes() {
		rs := node.GetRuntimeStatus()
		switch {
		case rs == nil:
			hasIncomplete = true
		case rs.Status == core.StatusFailed || rs.Status == core.StatusCancelled:
			hasFailed = true
		case rs.Status == core.StatusSuccess:
			hasSuccess = true
		default:
			hasIncomplete = true
		}
	}

	switch {
	case hasFailed:
		p.status = core.StatusFailed
	case hasIncomplete:
		p.status = core.StatusStopped
	case hasSuccess:
		p.status = core.StatusSuccess
	default:
		p.status = core.StatusStopped
	}
}

// Pause 暂停流水线，等待当前层执行完成后暂停
func (p *WorkflowImpl) Pause() error {
	p.pauseMu.Lock()
	defer p.pauseMu.Unlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status != core.StatusRunning {
		return fmt.Errorf("%w: current status is %s, expected RUNNING", core.ErrInvalidState, p.status)
	}

	// 设置状态为 PAUSED，并通知等待的 goroutine
	p.status = core.StatusPaused
	p.pauseCond.Broadcast()
	return nil
}

// Resume 恢复暂停的流水线
func (p *WorkflowImpl) Resume(ctx context.Context) error {
	p.pauseMu.Lock()
	defer p.pauseMu.Unlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status != core.StatusPaused && p.status != core.StatusStopped {
		return fmt.Errorf("%w: current status is %s, expected PAUSED or STOPPED", core.ErrInvalidState, p.status)
	}

	// 设置状态为 RUNNING，通知等待的 goroutine 继续执行
	p.status = core.StatusRunning
	p.pauseCond.Broadcast()
	return nil
}

// CurrentNode 返回当前正在执行的节点（如有）
func (p *WorkflowImpl) CurrentNode() Node {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.currentNode
}

// IsModifiable 判断当前是否可修改图
func (p *WorkflowImpl) IsModifiable() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	switch p.status {
	case core.StatusPaused, core.StatusStopped, core.StatusFailed, core.StatusCancelled, core.StatusSuccess:
		return true
	default:
		return false
	}
}

// restoreTraversalState 恢复遍历状态并重置
func (p *WorkflowImpl) restoreTraversalState() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	level := p.currentLevel
	p.currentLevel = 0
	return level
}

// Cancel 终止流水线
func (p *WorkflowImpl) Cancel() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cancelFunc != nil {
		p.cancelFunc()
		p.status = core.StatusCancelled

		// 通知监听器关于取消事件
		if p.listener != nil {
			p.listener.Handle(p, core.EventWorkflowCancelled)
		}

		if p.listening != nil {
			p.listening(p)
		}
	}
}

// Notify 这个主要是在运行过程中节点状态或者流水线状态变化，就会触发这个函数
// 节点
// 我们就可以在这里做一些处理
// 执行ListeningFn函数
func (p *WorkflowImpl) Notify() {
	p.mu.RLock()
	listening := p.listening
	listener := p.listener
	snapshot := newWorkflowSnapshot(p)
	p.mu.RUnlock()

	// 如果设置了ListeningFn则调用它
	if listening != nil {
		listening(snapshot)
	}

	// 如果设置了事件监听器则处理它
	if listener != nil {
		// 通知当前状态
		p.notifyCurrentStatus(listener, snapshot)
	}
}

// NotifyEventForNode 通知与特定节点关联的事件。
// 并行执行时 p.currentNode 会被多个 goroutine 交替覆盖/清空，
// 直接在快照上指定节点，避免监听器经 CurrentNode() 拿到错乱的节点。
func (p *WorkflowImpl) NotifyEventForNode(event Event, node Node) {
	p.mu.RLock()
	listener := p.listener
	listening := p.listening
	snapshot := newWorkflowSnapshot(p)
	p.mu.RUnlock()

	snapshot.currentNode = node

	if listener != nil {
		listener.Handle(snapshot, event)
	}

	if listening != nil {
		listening(snapshot)
	}
}

// NotifyEvent 通知监听器特定事件
func (p *WorkflowImpl) NotifyEvent(event Event) {
	p.mu.RLock()
	listener := p.listener
	listening := p.listening
	snapshot := newWorkflowSnapshot(p)
	p.mu.RUnlock()

	if listener != nil {
		listener.Handle(snapshot, event)
	}

	if listening != nil {
		listening(snapshot)
	}
}

// notifyCurrentStatus 通知监听器当前流水线状态
func (p *WorkflowImpl) notifyCurrentStatus(listener Listener, snapshot Workflow) {
	// 此方法可用于通知详细的状态变化
	// 目前，它仅用当前流水线调用监听器
	listener.Handle(snapshot, core.EventWorkflowStatusUpdate)
}

// SetExecutorProvider 设置Executor提供者
func (p *WorkflowImpl) SetExecutorProvider(provider ExecutorProvider) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.executorProvider = provider
	p.executors = make(map[string]executor.Executor)
}

// GetExecutorProvider 返回当前的 Executor 提供者
func (p *WorkflowImpl) GetExecutorProvider() ExecutorProvider {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.executorProvider
}

// getOrCreateExecutor 获取或创建Executor
func (p *WorkflowImpl) getOrCreateExecutor(ctx context.Context, name string) (executor.Executor, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 检查缓存
	if exec, ok := p.executors[name]; ok {
		return exec, nil
	}

	// 使用provider创建executor
	if p.executorProvider == nil {
		return nil, fmt.Errorf("executor provider not set")
	}

	exec, err := p.executorProvider.GetExecutor(ctx, name)
	if err != nil {
		return nil, err
	}

	// 缓存executor
	p.executors[name] = exec
	return exec, nil
}

// cleanupExecutors 清理所有executor
func (p *WorkflowImpl) cleanupExecutors(ctx context.Context) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for name, exec := range p.executors {
		if err := exec.Destruction(ctx); err != nil {
			fmt.Printf("failed to destroy executor %s: %v\n", name, err)
		}
	}
	p.executors = make(map[string]executor.Executor)
}

// SetTemplateEngine 设置模板引擎
func (p *WorkflowImpl) SetTemplateEngine(engine template.TemplateEngine) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.templateEngine = engine
}

// GetTemplateEngine 获取模板引擎
func (p *WorkflowImpl) GetTemplateEngine() template.TemplateEngine {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.templateEngine
}

// SetPusher 设置日志推送器
func (p *WorkflowImpl) SetPusher(pusher logger.Pusher) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pusher = pusher
}

// tryParseJSON 尝试将字符串解析为 JSON 对象或数组
// 性能优化：通过首字符快速判断，避免不必要的反序列化
func tryParseJSON(v interface{}) interface{} {
	str, ok := v.(string)
	if !ok {
		return v
	}

	trimmed := strings.TrimSpace(str)
	if len(trimmed) < 2 {
		return v
	}

	first := trimmed[0]
	last := trimmed[len(trimmed)-1]

	if !((first == '{' && last == '}') || (first == '[' && last == ']')) {
		return v
	}

	var result interface{}
	if err := json.Unmarshal([]byte(str), &result); err != nil {
		return v
	}
	return result
}

// buildRenderContext 构建渲染上下文，包含 Param 和动态 Metadata
func (p *WorkflowImpl) buildRenderContext() map[string]any {
	ctx := make(map[string]any)

	// 添加 Param（提取 FieldItem.Value 用于模板渲染）
	p.mu.RLock()
	if p.param != nil {
		paramValues := make(map[string]any)
		for k, v := range p.param {
			paramValues[k] = core.GetValue(v.Value)
		}
		ctx["Param"] = paramValues
		// 同时展开 param 到顶层，支持直接引用
		for k, v := range paramValues {
			ctx[k] = v
		}
	}
	p.mu.RUnlock()

	// 添加 Metadata 和其他动态数据（这里 Metadata 返回的是拷贝，安全）
	metadata := p.Metadata()
	if metadata != nil {
		metadataValues := make(map[string]any)
		for k, v := range metadata {
			metadataValues[k] = tryParseJSON(core.GetValue(v.Value))
		}
		ctx["Metadata"] = metadataValues
		// 将平铺的 metadata 转换为嵌套结构
		// 例如：Node1.value = "42", Node1.message = "hello"
		// 转换为：Node1 = {"value": "42", "message": "hello"}
		for k, v := range metadataValues {
			// 检查键名是否包含点（节点ID.键名）
			if dotIdx := strings.LastIndex(k, "."); dotIdx > 0 {
				nodeID := k[:dotIdx]
				keyName := k[dotIdx+1:]
				// 如果节点ID 的嵌套对象不存在，创建它
				if _, exists := ctx[nodeID]; !exists {
					ctx[nodeID] = make(map[string]any)
				}
				// 将键值添加到节点的嵌套对象中
				if nodeObj, ok := ctx[nodeID].(map[string]any); ok {
					nodeObj[keyName] = v
				}
			} else {
				// 不包含点的键，直接添加
				ctx[k] = v
			}
		}
	}

	return ctx
}

// renderStringWithRuntimeContext 使用运行时上下文渲染字符串
func (p *WorkflowImpl) renderStringWithRuntimeContext(templateStr string) (string, error) {
	engine := p.GetTemplateEngine()
	if engine == nil {
		return templateStr, nil // 没有模板引擎，返回原始值
	}
	ctx := p.buildRenderContext()
	return engine.EvaluateString(templateStr, ctx)
}

// nodeStringMap 从节点配置包读取 map[string]string 类型的保留键（params/env）。
// 兼容 YAML 反序列化出的 map[string]interface{}。
func nodeStringMap(node Node, key string) map[string]string {
	cfg := node.GetConfig()
	if cfg == nil {
		return nil
	}
	raw, ok := cfg[key]
	if !ok || raw == nil {
		return nil
	}
	switch m := raw.(type) {
	case map[string]string:
		return m
	case map[string]interface{}:
		out := make(map[string]string, len(m))
		for k, v := range m {
			out[k] = fmt.Sprintf("%v", v)
		}
		return out
	case map[interface{}]interface{}: // yaml.v2 反序列化的默认 map 类型
		out := make(map[string]string, len(m))
		for k, v := range m {
			out[fmt.Sprintf("%v", k)] = fmt.Sprintf("%v", v)
		}
		return out
	}
	return nil
}

// buildNodeRenderContext 在 workflow 渲染上下文基础上叠加节点级参数绑定（params）：
// 渲染本节点模板时 {{ Param.<name> }} 优先解析到节点绑定，未绑定的回退 workflow 级 Param。
// 绑定值本身是模板（如 {{ Param.project_dir }}/backend、{{ Upstream.city }}）时，
// 先在 workflow 上下文求值再代入——绑定模板里的 Param 引用一律指向 workflow 级 Param，
// 不存在绑定间互相引用，因此无循环引用风险。
func (p *WorkflowImpl) buildNodeRenderContext(node Node) map[string]any {
	ctx := p.buildRenderContext()
	params := nodeStringMap(node, "params")
	if len(params) == 0 {
		return ctx
	}
	engine := p.GetTemplateEngine()
	baseParam, _ := ctx["Param"].(map[string]any)
	merged := make(map[string]any, len(baseParam)+len(params))
	for k, v := range baseParam {
		merged[k] = v
	}
	for k, v := range params {
		if engine != nil && strings.Contains(v, "{{") {
			rendered, err := engine.EvaluateString(v, ctx)
			if err != nil {
				fmt.Printf("Warning: node %s param %q render failed: %v, using raw value\n", node.Id(), k, err)
				merged[k] = v
				continue
			}
			merged[k] = rendered
		} else {
			merged[k] = v
		}
	}
	ctx["Param"] = merged
	// 与 buildRenderContext 的顶层展开语义一致：绑定值也展开到顶层，支持直接引用
	for k := range params {
		ctx[k] = merged[k]
	}
	return ctx
}

// renderNodeStringWithContext 使用节点级渲染上下文渲染字符串（step.Run / env 值）
func (p *WorkflowImpl) renderNodeStringWithContext(node Node, ctx map[string]any, templateStr string) (string, error) {
	engine := p.GetTemplateEngine()
	if engine == nil {
		return templateStr, nil
	}
	return engine.EvaluateString(templateStr, ctx)
}