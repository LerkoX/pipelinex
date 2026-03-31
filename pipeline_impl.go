package pipelinex

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/LerkoX/pipelinex/executor"
	"github.com/google/uuid"
	"github.com/thoas/go-funk"
)


// 预检查PipelineImpl是否实现了Pipeline接口
var _ Pipeline = (*PipelineImpl)(nil)
var _ Graph = (*DGAGraph)(nil)

// 保存了流水线的图结构
type DGAGraph struct {
	mu       sync.RWMutex
	nodes    map[string]Node
	edges    map[string]Edge            // edgeID -> Edge
	graph    map[string][]string        // src -> [dest1, dest2, ...] (保持兼容性)
	edgeMap  map[string]map[string]Edge // src -> dest -> Edge (快速查找)
	sequence []string
	hasCycle bool
}

func NewDGAGraph() *DGAGraph {
	return &DGAGraph{
		nodes:    map[string]Node{},
		edges:    map[string]Edge{},
		graph:    map[string][]string{},
		edgeMap:  map[string]map[string]Edge{},
		sequence: []string{},
	}
}

// Nodes 返回所有的节点map
func (dga *DGAGraph) Nodes() map[string]Node {
	dga.mu.RLock()
	defer dga.mu.RUnlock()
	return funk.Map(dga.nodes, func(k string, v Node) (string, Node) {
		return k, v
	}).(map[string]Node)
}

// Edges 返回所有的边
func (dga *DGAGraph) Edges() []Edge {
	dga.mu.RLock()
	defer dga.mu.RUnlock()
	edges := make([]Edge, 0, len(dga.edges))
	for _, edge := range dga.edges {
		edges = append(edges, edge)
	}
	return edges
}

// AddVertex 向图中添加顶点（节点）
// 检查是否存在循环；如果存在循环，则返回 ErrHasCycle
// 否则返回 nil
func (dga *DGAGraph) AddVertex(node Node) {
	dga.mu.Lock()
	defer dga.mu.Unlock()
	dga.nodes[node.Id()] = node
	dga.graph[node.Id()] = []string{}
}

// AddEdge 向图中添加边
func (dga *DGAGraph) AddEdge(edge Edge) error {
	dga.mu.Lock()
	defer dga.mu.Unlock()

	src := edge.Source()
	dest := edge.Target()

	if _, ok := dga.nodes[src.Id()]; !ok {
		return fmt.Errorf("source vertex %s not found", src.Id())
	}
	if _, ok := dga.nodes[dest.Id()]; !ok {
		return fmt.Errorf("dest vertex %s not found", dest.Id())
	}

	// 添加到edges映射
	dga.edges[edge.ID()] = edge

	// 添加到graph映射（保持兼容性）
	dga.graph[src.Id()] = append(dga.graph[src.Id()], dest.Id())

	// 添加到edgeMap（快速查找）
	if dga.edgeMap[src.Id()] == nil {
		dga.edgeMap[src.Id()] = make(map[string]Edge)
	}
	dga.edgeMap[src.Id()][dest.Id()] = edge

	dga.hasCycle = dga.cycleCheck()
	if dga.hasCycle {
		return ErrHasCycle
	}
	return nil
}

// Traversal 对DAG执行广度优先遍历
// 为图中的每个节点执行提供的 TraversalFn 函数
// 支持多个起始节点并发执行
// 支持条件边：如果边有表达式，会评估表达式决定是否遍历该边
func (dga *DGAGraph) Traversal(ctx context.Context, evalCtx EvaluationContext, fn TraversalFn) error {
	dga.mu.RLock()
	defer dga.mu.RUnlock()

	// 如果没有节点，直接返回
	if len(dga.nodes) == 0 {
		return nil
	}

	// 计算所有节点的入度（基于原始图结构）
	indeg := dga.getIndegrees()

	// 收集所有入度为0的起始节点
	startNodes := make([]string, 0)
	for v, d := range indeg {
		if d == 0 {
			startNodes = append(startNodes, v)
		}
	}

	if len(startNodes) == 0 {
		return nil // 没有起始节点
	}

	visited := make(map[string]bool)
	queue := make([]string, 0)

	// 并发执行所有起始节点
	var wg sync.WaitGroup
	var errMu sync.Mutex
	var firstErr error

	for _, startNodeID := range startNodes {
		nodeID := startNodeID // 避免闭包捕获问题
		visited[nodeID] = true
		queue = append(queue, nodeID)

		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			if err := fn(ctx, dga.nodes[id]); err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				errMu.Unlock()
			}
		}(nodeID)
	}

	// 等待所有起始节点完成
	wg.Wait()
	if firstErr != nil {
		return firstErr
	}

	// BFS 遍历剩余节点
	for len(queue) > 0 {
		vertexFocus := queue[0]
		queue = queue[1:]

		// 为当前节点的所有邻居创建 WaitGroup
		var wg sync.WaitGroup
		var errMu sync.Mutex
		var layerErr error

		for _, neighbor := range dga.graph[vertexFocus] {
			if visited[neighbor] {
				continue
			}

			// 获取边并评估条件
			shouldTraverse := true
			if edge, ok := dga.edgeMap[vertexFocus][neighbor]; ok && edge.Expression() != "" {
				result, err := edge.Evaluate(evalCtx)
				if err != nil {
					return fmt.Errorf("failed to evaluate edge condition %s->%s: %w",
						vertexFocus, neighbor, err)
				}
				shouldTraverse = result
			}

			// 条件不满足，跳过此边（不减少入度）
			if !shouldTraverse {
				continue
			}

			// 减少邻居的入度
			indeg[neighbor]--

			// 只有当入度减为0时才访问节点
			if indeg[neighbor] == 0 {
				visited[neighbor] = true
				queue = append(queue, neighbor)

				// 并发执行邻居节点
				wg.Add(1)
				go func(n string) {
					defer wg.Done()
					if err := fn(ctx, dga.nodes[n]); err != nil {
						errMu.Lock()
						if layerErr == nil {
							layerErr = err
						}
						errMu.Unlock()
					}
				}(neighbor)
			}
		}

		// 等待当前层的所有 goroutine 完成
		wg.Wait()
		if layerErr != nil {
			return layerErr
		}
	}

	return nil
}

// cycleCheck 检查有向无环图（DAG）中是否存在循环
// 如果找到循环则返回 true，否则返回 false
func (dga *DGAGraph) cycleCheck() bool {
	indeg := dga.getIndegrees()
	q := make([]string, 0)
	for v, d := range indeg {
		if d == 0 {
			q = append(q, v)
		}
	}
	visited := 0
	for len(q) > 0 {
		v := q[0]
		q = q[1:]
		visited++
		for _, n := range dga.graph[v] {
			indeg[n]--
			if indeg[n] == 0 {
				q = append(q, n)
			}
		}
	}
	return visited != len(dga.nodes)
}

// getIndegrees 计算所有节点的入度
// 返回一个 map，key 是节点ID，value 是入度值
func (dga *DGAGraph) getIndegrees() map[string]int {
	indeg := make(map[string]int)
	for v := range dga.nodes {
		indeg[v] = 0
	}
	for _, adj := range dga.graph {
		for _, n := range adj {
			indeg[n]++
		}
	}
	return indeg
}

// HasCycle 检查图中是否存在循环
func (dga *DGAGraph) HasCycle() bool {
	dga.mu.RLock()
	defer dga.mu.RUnlock()
	return dga.hasCycle
}

type PipelineImpl struct {
	id               string
	graph            Graph
	status           string
	metadata         Metadata
	metadataStore    MetadataStore
	listening        ListeningFn
	listener         Listener
	doneChan         chan struct{}
	cancelFunc       context.CancelFunc
	mu               sync.RWMutex
	executorProvider ExecutorProvider
	executors        map[string]Executor // 缓存已创建的executor
	param            map[string]interface{} // 存储渲染后的Param值
	templateEngine    TemplateEngine      // 模板引擎
	cleanupOnce      sync.Once          // 保护清理操作只执行一次
}

func NewPipeline(ctx context.Context) Pipeline {
	return &PipelineImpl{
		id:        uuid.NewString(),
		executors: make(map[string]Executor),
		doneChan:  make(chan struct{}),
	}
}

// SetParam 设置 param 值
func (p *PipelineImpl) SetParam(param map[string]interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.param = param
}

// Id 返回流水线的ID
func (p *PipelineImpl) Id() string {
	return p.id
}

// GetGraph 返回流水线的图结构
func (p *PipelineImpl) GetGraph() Graph {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.graph
}

// SetGraph 设置流水线的图结构
func (p *PipelineImpl) SetGraph(graph Graph) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.graph = graph
}

// Status 返回流水线的整体状态
func (p *PipelineImpl) Status() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.status
}

// SetMetadata 设置流水线的元数据存储
func (p *PipelineImpl) SetMetadata(store MetadataStore) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.metadataStore = store
}

// Metadata 获取流水线的元数据
func (p *PipelineImpl) Metadata() Metadata {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.metadata == nil {
		p.metadata = make(Metadata)
	}

	// 如果有 metadataStore，从 store 加载数据
	if p.metadataStore != nil {
		// 从 InConfigMetadataStore 加载所有数据
		if inConfigStore, ok := p.metadataStore.(*InConfigMetadataStore); ok {
			for k, v := range inConfigStore.data {
				p.metadata[k] = v
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
func (p *PipelineImpl) Listening(fn Listener) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.listener = fn
}

// Done 返回一个通道，用于通知流水线何时完成
func (p *PipelineImpl) Done() <-chan struct{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.doneChan
}

// shouldSkipNode 检查节点是否应该跳过执行
func (p *PipelineImpl) shouldSkipNode(node Node) bool {
	runtimeStatus := node.GetRuntimeStatus()
	if runtimeStatus == nil {
		return false
	}
	// SUCCESS、FAILED、CANCELLED 状态的节点都跳过
	// RUNNING 状态的节点需要恢复（不跳过）
	switch runtimeStatus.Status {
	case StatusSuccess, StatusFailed, StatusCancelled:
		return true
	default:
		return false
	}
}

// Run 执行流水线
func (p *PipelineImpl) Run(ctx context.Context) error {
	p.mu.Lock()
	ctx, cancel := context.WithCancel(ctx)
	p.cancelFunc = cancel
	p.mu.Unlock()

	defer func() {
		p.cleanupOnce.Do(func() {
			close(p.doneChan)

			p.mu.Lock()
			if p.cancelFunc != nil {
				p.cancelFunc()
				p.cancelFunc = nil
			}
			p.mu.Unlock()

			// 清理所有executor
			p.cleanupExecutors(ctx)
		})
	}()

	// 通知流水线开始
	p.notifyEvent(PipelineStart)

	// 创建求值上下文
	evalCtx := NewEvaluationContext().WithPipeline(p)

	// 如果有元数据存储，加载数据到求值上下文
	if p.metadataStore != nil {
		params := make(map[string]any)
		// 这里可以根据需求加载特定的元数据键
		// 目前加载所有配置中的数据（仅in-config类型适用）
		if inConfigStore, ok := p.metadataStore.(*InConfigMetadataStore); ok {
			for key := range inConfigStore.data {
				if val, err := inConfigStore.Get(ctx, key); err == nil {
					params[key] = val
				}
			}
		}
		if len(params) > 0 {
			evalCtx = evalCtx.WithParams(params)
		}
	}

	err := p.graph.Traversal(ctx, evalCtx, func(ctx context.Context, node Node) error {
		// 检查context是否已取消
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 检查是否应该跳过此节点
		if p.shouldSkipNode(node) {
			fmt.Printf("Skipping node %s (status: %s)\n", node.Id(), node.GetRuntimeStatus().Status)
			p.notifyEvent(PipelineNodeFinish)
			return nil
		}

		// 通知节点开始
		p.notifyEvent(PipelineNodeStart)
		fmt.Printf("Executing node: %s\n", node.Id())

		// 获取节点的executor配置
		executorName := node.GetExecutor()
		if executorName == "" {
			fmt.Printf("Node %s has no executor configured, skipping\n", node.Id())
			p.notifyEvent(PipelineNodeFinish)
			return nil
		}

		// 获取或创建executor
		executor, err := p.getOrCreateExecutor(ctx, executorName)
		if err != nil {
			fmt.Printf("Failed to get executor for node %s: %v\n", node.Id(), err)
			return fmt.Errorf("failed to get executor for node %s: %w", node.Id(), err)
		}

		// 执行节点
		if err := p.executeNode(ctx, node, executor); err != nil {
			fmt.Printf("Node %s execution failed: %v\n", node.Id(), err)
			return err
		}

		// 通知节点完成
		p.notifyEvent(PipelineNodeFinish)
		return nil
	})

	// 通知流水线完成
	if err != nil {
		p.status = StatusFailed
	} else {
		p.status = StatusSuccess
	}
	p.notifyEvent(PipelineFinish)
	return err
}

// executeNode 执行单个节点
func (p *PipelineImpl) executeNode(ctx context.Context, node Node, exec Executor) error {
	steps := node.GetSteps()
	if len(steps) == 0 {
		fmt.Printf("Node %s has no steps to execute\n", node.Id())
		return nil
	}

	// 1. 初始化节点运行时状态
	runtimeStatus := p.initializeNodeRuntimeStatus(node, exec)
	node.SetRuntimeStatus(runtimeStatus)

	// 2. 设置通道和启动 executor
	commandChan, resultChan, inputChan := p.setupExecutorChannels(ctx, exec, steps)

	// 将 inputChan 保存到 RuntimeStatus，供外部交互使用
	runtimeStatus = node.GetRuntimeStatus()
	if runtimeStatus != nil {
		runtimeStatus.InputChan = inputChan
		node.SetRuntimeStatus(runtimeStatus)
	}

	// 3. 发送所有步骤命令
	go p.sendCommands(ctx, node, commandChan, steps)

	// 4. 等待并处理所有结果
	lastErr, fullOutput := p.waitForResults(ctx, node, exec, resultChan, steps)

	// 5. 从完整输出中提取元数据
	if lastErr == nil {
		// 创建一个虚拟 StepResult 用于提取
		stepResult := &StepResult{
			StepName:  node.Id(),
			Output:    fullOutput,
			StartTime: time.Now(),
		}
		if err := p.extractOutput(ctx, node, stepResult, fullOutput); err != nil {
			fmt.Printf("Failed to extract output from node %s: %v\n", node.Id(), err)
		}
	}

	// 6. 更新节点最终状态
	if runtimeStatus = node.GetRuntimeStatus(); runtimeStatus != nil {
		p.updateNodeFinalStatus(runtimeStatus, lastErr)
		node.SetRuntimeStatus(runtimeStatus)
	}

	// 7. 关闭 inputChan
	close(inputChan)

	return lastErr
}

// initializeNodeRuntimeStatus 初始化节点运行时状态
func (p *PipelineImpl) initializeNodeRuntimeStatus(node Node, exec Executor) *NodeRuntimeStatus {
	node.EnsureIds()
	runtimeStatus := node.GetRuntimeStatus()
	if runtimeStatus == nil {
		runtimeStatus = &NodeRuntimeStatus{
			Id:    NewUUID(),
			Steps: []StepRuntimeStatus{},
		}
	}

	// 更新节点状态
	runtimeStatus.Status = StatusRunning
	runtimeStatus.StartTime = time.Now().Format(time.RFC3339)

	// 收集 Executor 信息
	if infoProvider, ok := exec.(executor.ExecutorInfoProvider); ok {
		runtimeStatus.Executor = &ExecutorRuntimeInfo{
			Type:       infoProvider.GetType(),
			InstanceId: infoProvider.GetInstanceId(),
			Status:     executor.ExecutorStatusRunning,
			Info:       infoProvider.GetRuntimeInfo(),
		}
	}

	return runtimeStatus
}

// setupExecutorChannels 设置通道并启动 executor
func (p *PipelineImpl) setupExecutorChannels(ctx context.Context, exec Executor, steps []Step) (chan any, chan any, chan []byte) {
	commandChan := make(chan any, len(steps))
	resultChan := make(chan any, len(steps)*10)
	inputChan := make(chan []byte, 100) // 输入通道，缓冲100条消息

	// 启动 executor 的 Transfer goroutine
	go exec.Transfer(ctx, resultChan, commandChan, inputChan)

	return commandChan, resultChan, inputChan
}

// sendCommands 发送所有步骤命令
func (p *PipelineImpl) sendCommands(ctx context.Context, node Node, commandChan chan any, steps []Step) {
	defer close(commandChan)
	for _, step := range steps {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// 检查 step 是否已完成（运行时状态恢复）
		if p.shouldSkipStep(node, step.Name) {
			fmt.Printf("Skipping step %s (status: %s)\n", step.Name, getStepStatusString(node, step.Name))
			continue
		}

		// 运行时渲染 step.Run，可以引用前面节点 extract 的数据
		renderedRun, err := p.renderStringWithRuntimeContext(step.Run)
		if err != nil {
			fmt.Printf("Failed to render step %s: %v, using original command\n", step.Name, err)
			renderedRun = step.Run // 渲染失败时使用原始命令
		}

		commandChan <- executor.CommandWrapper{
			StepName: step.Name,
			Command:  renderedRun,
		}
	}
}

// shouldSkipStep 检查步骤是否应该跳过执行
func (p *PipelineImpl) shouldSkipStep(node Node, stepName string) bool {
	runtimeStatus := node.GetRuntimeStatus()
	if runtimeStatus == nil {
		return false
	}

	// 查找 step 的运行时状态
	for _, step := range runtimeStatus.Steps {
		if step.Name == stepName {
			// SUCCESS、FAILED、CANCELLED 状态的 step 都跳过
			switch step.Status {
			case StatusSuccess, StatusFailed, StatusCancelled:
				return true
			default:
				return false
			}
		}
	}
	return false
}

// getStepStatusString 辅助函数：获取 step 状态字符串（避免 nil panic）
func getStepStatusString(node Node, stepName string) string {
	runtimeStatus := node.GetRuntimeStatus()
	if runtimeStatus == nil {
		return "unknown"
	}

	for _, step := range runtimeStatus.Steps {
		if step.Name == stepName {
			return step.Status
		}
	}
	return "unknown"
}

// waitForResults 等待并处理所有结果
func (p *PipelineImpl) waitForResults(ctx context.Context, node Node, exec Executor, resultChan chan any, steps []Step) (error, string) {
	var lastErr error
	resultCount := 0
	// 计算实际需要执行的步骤数量（不包括已完成的步骤）
	expectedResults := 0
	for _, step := range steps {
		if !p.shouldSkipStep(node, step.Name) {
			expectedResults++
		}
	}
	// 如果没有需要执行的步骤，直接返回
	if expectedResults == 0 {
		return nil, ""
			}
	var allOutput strings.Builder

	for resultCount < expectedResults {
		select {
		case <-ctx.Done():
			p.handleCancellation(nodeContext{ctx: ctx, node: node})
			return ctx.Err(), allOutput.String()
		case result, ok := <-resultChan:
			if !ok {
				return lastErr, allOutput.String()
			}

			errCount := p.handleResult(ctx, node, exec, result, resultCount, steps)
			if errCount.err != nil {
				lastErr = errCount.err
			}
			resultCount = errCount.count
			allOutput.WriteString(errCount.output)
		}
	}

	return lastErr, allOutput.String()
}

// handleCancellation 处理节点取消
func (p *PipelineImpl) handleCancellation(ctx nodeContext) {
	runtimeStatus := ctx.node.GetRuntimeStatus()
	if runtimeStatus != nil {
		runtimeStatus.Status = StatusCancelled
		runtimeStatus.EndTime = time.Now().Format(time.RFC3339)
		ctx.node.SetRuntimeStatus(runtimeStatus)
	}
}

// handleResult 处理单个结果
func (p *PipelineImpl) handleResult(ctx context.Context, node Node, _ Executor, result any, resultCount int, steps []Step) resultHandler {
	handler := resultHandler{count: resultCount}

	switch v := result.(type) {
	case error:
		handler.err = v
		handler.count++
	case *StepResult:
		if v.Error != nil {
			handler.err = v.Error
		}

		// 通过步骤名称查找对应的步骤（修复索引映射错误）
		var targetStep *Step
		for i := range steps {
			if steps[i].Name == v.StepName {
				targetStep = &steps[i]
				break
			}
		}

		if targetStep != nil {
			p.updateStepRuntimeStatus(node, *targetStep, v)
		} else {
			fmt.Printf("Warning: StepResult for '%s' not found in steps array\n", v.StepName)
		}
		handler.count++
	case []byte:
		// 实时输出
	output := string(v)
		// fmt.Print(output) - removed to avoid concurrent output issues
		handler.output = output
	}

	return handler
}

// resultHandler 处理结果的辅助结构
type resultHandler struct {
	err    error
	count  int
	output string
}

// nodeContext 节点上下文辅助结构
type nodeContext struct {
	ctx  context.Context
	node Node
}

// updateStepRuntimeStatus 更新步骤运行时状态
func (p *PipelineImpl) updateStepRuntimeStatus(node Node, step Step, result *StepResult) {
	stepStatus := StepRuntimeStatus{
		Id:     step.Id,
		Name:   step.Name,
		Status: StatusSuccess,
	}

	if !result.StartTime.IsZero() {
		stepStatus.StartTime = result.StartTime.Format(time.RFC3339)
	}
	if !result.FinishTime.IsZero() {
		stepStatus.EndTime = result.FinishTime.Format(time.RFC3339)
	}
	if result.Error != nil {
		stepStatus.Status = StatusFailed
		stepStatus.Error = result.Error.Error()
	}

	node.SetStepRuntimeStatus(&stepStatus)
}

// updateNodeFinalStatus 更新节点最终状态
func (p *PipelineImpl) updateNodeFinalStatus(runtimeStatus *NodeRuntimeStatus, err error) {
	if err != nil {
		runtimeStatus.Status = StatusFailed
	} else {
		runtimeStatus.Status = StatusSuccess
	}
	runtimeStatus.EndTime = time.Now().Format(time.RFC3339)

	// 更新 Executor 状态为 DESTROYED
	if runtimeStatus.Executor != nil {
		runtimeStatus.Executor.Status = executor.ExecutorStatusDestroyed
	}
}

// 这个主要是在运行过程中节点状态或者流水线状态变化，就会触发这个函数
// 节点
// 我们就可以在这里做一些处理
// 执行ListeningFn函数
func (p *PipelineImpl) Notify() {
	p.mu.RLock()
	listening := p.listening
	listener := p.listener
	p.mu.RUnlock()

	// 如果设置了ListeningFn则调用它
	if listening != nil {
		listening(p)
	}

	// 如果设置了事件监听器则处理它
	if listener != nil {
		// 通知当前状态
		p.notifyCurrentStatus(listener)
	}
}

// 终止流水线
func (p *PipelineImpl) Cancel() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cancelFunc != nil {
		p.cancelFunc()
		p.status = StatusCancelled

		// 通知监听器关于取消事件
		if p.listener != nil {
			p.listener.Handle(p, EventPipelineCancelled)
		}

		if p.listening != nil {
			p.listening(p)
		}
	}
}

// notifyEvent 通知监听器特定事件
func (p *PipelineImpl) notifyEvent(event Event) {
	p.mu.RLock()
	listener := p.listener
	listening := p.listening
	p.mu.RUnlock()

	if listener != nil {
		listener.Handle(p, event)
	}

	if listening != nil {
		listening(p)
	}
}

// notifyCurrentStatus 通知监听器当前流水线状态
func (p *PipelineImpl) notifyCurrentStatus(listener Listener) {
	// 此方法可用于通知详细的状态变化
	// 目前，它仅用当前流水线调用监听器
	listener.Handle(p, EventPipelineStatusUpdate)
}

// SetExecutorProvider 设置Executor提供者
func (p *PipelineImpl) SetExecutorProvider(provider ExecutorProvider) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.executorProvider = provider
	p.executors = make(map[string]Executor)
}

// getOrCreateExecutor 获取或创建Executor
func (p *PipelineImpl) getOrCreateExecutor(ctx context.Context, name string) (Executor, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 检查缓存
	if executor, ok := p.executors[name]; ok {
		return executor, nil
	}

	// 使用provider创建executor
	if p.executorProvider == nil {
		return nil, fmt.Errorf("executor provider not set")
	}

	executor, err := p.executorProvider.GetExecutor(ctx, name)
	if err != nil {
		return nil, err
	}

	// 缓存executor
	p.executors[name] = executor
	return executor, nil
}

// cleanupExecutors 清理所有executor
func (p *PipelineImpl) cleanupExecutors(ctx context.Context) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for name, executor := range p.executors {
		if err := executor.Destruction(ctx); err != nil {
			fmt.Printf("failed to destroy executor %s: %v\n", name, err)
		}
	}
	p.executors = make(map[string]Executor)
}

// SetTemplateEngine 设置模板引擎
func (p *PipelineImpl) SetTemplateEngine(engine TemplateEngine) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.templateEngine = engine
}

// GetTemplateEngine 获取模板引擎
func (p *PipelineImpl) GetTemplateEngine() TemplateEngine {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.templateEngine
}

// buildRenderContext 构建渲染上下文，包含 Param 和动态 Metadata
func (p *PipelineImpl) buildRenderContext() map[string]any {
	ctx := make(map[string]any)

	// 添加 Param
	p.mu.RLock()
	if p.param != nil {
		ctx["Param"] = p.param
		// 同时展开 param 到顶层，支持直接引用
		for k, v := range p.param {
			ctx[k] = v
		}
	}
	p.mu.RUnlock()

	// 添加 Metadata 和其他动态数据（这里 Metadata 返回的是拷贝，安全）
	metadata := p.Metadata()
	if metadata != nil {
		ctx["Metadata"] = metadata
		// 将平铺的 metadata 转换为嵌套结构
		// 例如：Node1.value = "42", Node1.message = "hello"
		// 转换为：Node1 = {"value": "42", "message": "hello"}
		for k, v := range metadata {
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
func (p *PipelineImpl) renderStringWithRuntimeContext(template string) (string, error) {
	engine := p.GetTemplateEngine()
	if engine == nil {
		return template, nil // 没有模板引擎，返回原始值
	}
	ctx := p.buildRenderContext()
	return engine.EvaluateString(template, ctx)
}

// extractOutput 从节点输出中提取数据并保存到metadata
func (p *PipelineImpl) extractOutput(ctx context.Context, node Node, stepResult *StepResult, fullOutput string) error {
	// 获取节点配置
	nodeConfig := node.GetConfig()
	if nodeConfig == nil {
		return nil
	}

	// 检查是否有提取配置
	extractConfig, hasExtract := nodeConfig["extract"]
	if !hasExtract || extractConfig == nil {
		return nil
	}

	// 创建提取器
	extractor, err := p.createExtractor(extractConfig)
	if err != nil {
		return fmt.Errorf("failed to create extractor: %w", err)
	}

	// 执行提取

	extracted, err := extractor.Extract(fullOutput)
	if err != nil {
		return fmt.Errorf("failed to extract data from node %s: %w", node.Id(), err)
	}

	// 保存到 metadata（加锁防止并发写入）
	if len(extracted) > 0 {
		p.mu.Lock()
		defer p.mu.Unlock()

		// 确保 metadata 已初始化
		if p.metadata == nil {
			p.metadata = make(Metadata)
		}

		// 保存到内存 metadata
		for key, value := range extracted {
			metadataKey := fmt.Sprintf("%s.%s", node.Id(), key)
			p.metadata[metadataKey] = convertBoolToString(value)
		}

		// 如果有 metadata store，同步保存
		if p.metadataStore != nil {
			for key, value := range extracted {
				metadataKey := fmt.Sprintf("%s.%s", node.Id(), key)
				valueStr := fmt.Sprintf("%v", value)
				if err := p.metadataStore.Set(ctx, metadataKey, valueStr); err != nil {
					fmt.Printf("Warning: Failed to save extracted data to store: %v\n", err)
				}
			}
		}
	}

	return nil
}

// createExtractor 根据配置创建提取器
func (p *PipelineImpl) createExtractor(extractConfig interface{}) (OutputExtractor, error) {
	if extractConfig == nil {
		return nil, nil
	}

	var configMap map[string]interface{}

	// 尝试解析为 map[string]interface{}
	if m, ok := extractConfig.(map[string]interface{}); ok {
		configMap = m
	} else if extractPtr, ok := extractConfig.(*ExtractConfig); ok && extractPtr != nil {
		// 转换 *ExtractConfig 到 map[string]interface{}
		configMap = make(map[string]interface{})
		if extractPtr.Type != "" {
			configMap["type"] = extractPtr.Type
		}
		if extractPtr.Patterns != nil {
			// 将 Patterns 转换为 map[string]interface{}
			patternsMap := make(map[string]interface{})
			for k, v := range extractPtr.Patterns {
				patternsMap[k] = v
			}
			configMap["patterns"] = patternsMap
		}
		if extractPtr.MaxOutputSize > 0 {
			configMap["maxOutputSize"] = extractPtr.MaxOutputSize
		}
	} else {
		return nil, fmt.Errorf("invalid extract config format")
	}

	// 获取提取类型
	extractType := "codec-block" // 默认类型
	if typeVal, ok := configMap["type"]; ok {
		if typeStr, ok := typeVal.(string); ok {
			extractType = typeStr
		}
	}

	// 获取输出大小限制
	maxOutputSize := 1024 * 1024 // 默认 1MB
	if sizeVal, ok := configMap["maxOutputSize"]; ok {
		if size, ok := sizeVal.(int); ok {
			maxOutputSize = size
		}
	}

	switch extractType {
	case "codec-block":
		return NewCodecBlockExtractor(maxOutputSize), nil

	case "regex":
		patterns := make(map[string]string)
		if patternsVal, ok := configMap["patterns"]; ok {
			if patternsMap, ok := patternsVal.(map[string]interface{}); ok {
				for k, v := range patternsMap {
					if patternStr, ok := v.(string); ok {
						patterns[k] = patternStr
					}
				}
			}
		}
		if len(patterns) == 0 {
			return nil, fmt.Errorf("regex extractor requires patterns")
		}
		return NewRegexExtractor(patterns, maxOutputSize)

	default:
		return nil, fmt.Errorf("unsupported extract type: %s", extractType)
	}
}
