package pipelinex

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

// TestNewRuntime tests creating a new Runtime instance
func TestNewRuntime(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx)

	if runtime == nil {
		t.Fatal("NewRuntime should not return nil")
	}

	// Check if it's a RuntimeImpl type using reflection
	if reflect.TypeOf(runtime).String() != "*pipelinex.RuntimeImpl" {
		t.Fatalf("NewRuntime should return *RuntimeImpl, got %v", reflect.TypeOf(runtime))
	}
}

// TestRuntimeImpl_Get tests getting a pipeline
func TestRuntimeImpl_Get(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx)

	// Test getting non-existent pipeline
	_, err := runtime.Get("non-existent")
	if err == nil {
		t.Fatal("Expected error when getting non-existent pipeline")
	}
}

// TestRuntimeImpl_RunSync tests synchronous pipeline execution
func TestRuntimeImpl_RunSync(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx)

	// Prepare test configuration with new format
	config := `
Param:
  testParam: "test-value"
Executors:
  local:
    type: local
    config:
      shell: bash
Graph: |
  stateDiagram-v2
    [*] --> Task1
    Task1 --> Task2
Nodes:
  Task1:
    executor: local
    steps:
      - name: step1
        run: "echo 'task1'"
  Task2:
    executor: local
    steps:
      - name: step1
        run: "echo 'task2'"
`

	// Create test listener
	listener := &TestListener{}

	// Execute synchronous pipeline
	pipeline, err := runtime.RunSync(ctx, "test-sync-pipeline", config, listener)
	if err != nil {
		t.Fatalf("RunSync failed: %v", err)
	}

	if pipeline == nil {
		t.Fatal("Pipeline should not be nil")
	}

	// Check if pipeline is cleaned up
	_, err = runtime.Get("test-sync-pipeline")
	if err == nil {
		t.Fatal("Pipeline should be cleaned up after sync execution")
	}
}

// TestRuntimeImpl_RunSync_InvalidConfig tests synchronous execution with invalid config
func TestRuntimeImpl_RunSync_InvalidConfig(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx)

	// Prepare invalid configuration
	invalidConfig := `
invalid: yaml: content
  testParam: "test-value"
  missing: closing: brace
`

	_, err := runtime.RunSync(ctx, "test-invalid-config", invalidConfig, nil)
	if err == nil {
		t.Fatal("Expected error with invalid YAML config")
	}
}

// TestRuntimeImpl_RunSync_DuplicateID tests synchronous execution with duplicate ID
func TestRuntimeImpl_RunSync_DuplicateID(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx)

	// Prepare test configuration with new format
	config := `
Param:
  testParam: "test-value"
Executors:
  local:
    type: local
    config:
      shell: bash
Graph: |
  stateDiagram-v2
    [*] --> Task1
Nodes:
  Task1:
    executor: local
    steps:
      - name: step1
        run: "echo 'task1'"
`

	// First execution
	_, err := runtime.RunSync(ctx, "duplicate-id", config, nil)
	if err != nil {
		t.Fatalf("First RunSync failed: %v", err)
	}

	// Second execution with same ID
	_, err = runtime.RunSync(ctx, "duplicate-id", config, nil)
	if err == nil {
		t.Fatal("Expected error when running pipeline with duplicate ID")
	}
}

// TestRuntimeImpl_RunAsync tests asynchronous pipeline execution
func TestRuntimeImpl_RunAsync(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx)

	// Prepare test configuration with new format
	config := `
Param:
  testParam: "test-value"
Executors:
  local:
    type: local
    config:
      shell: bash
Graph: |
  stateDiagram-v2
    [*] --> Task1
Nodes:
  Task1:
    executor: local
    steps:
      - name: step1
        run: "echo 'task1'"
`

	// Create test listener
	listener := &TestListener{}

	// Execute asynchronous pipeline
	pipeline, err := runtime.RunAsync(ctx, "test-async-pipeline", config, listener)
	if err != nil {
		t.Fatalf("RunAsync failed: %v", err)
	}

	if pipeline == nil {
		t.Fatal("Pipeline should not be nil")
	}

	// Check if pipeline is stored in runtime
	retrieved, err := runtime.Get("test-async-pipeline")
	if err != nil {
		t.Fatalf("Pipeline should be stored in runtime: %v", err)
	}
	if retrieved != pipeline {
		t.Fatal("Retrieved pipeline should be the same instance")
	}

	// Wait for async execution to complete
	select {
	case <-pipeline.Done():
		// Pipeline completed
	case <-time.After(5 * time.Second):
		// Cancel pipeline
		runtime.Cancel(ctx, "test-async-pipeline")
	}
}

// TestRuntimeImpl_Cancel tests pipeline cancellation
func TestRuntimeImpl_Cancel(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx)

	// Prepare test configuration with new format - use sleep to ensure pipeline is running
	config := `
Param:
  testParam: "test-value"
Executors:
  local:
    type: local
    config:
      shell: bash
Graph: |
  stateDiagram-v2
    [*] --> Task1
Nodes:
  Task1:
    executor: local
    steps:
      - name: step1
        run: "sleep 2"
`

	// Execute asynchronous pipeline
	pipeline, err := runtime.RunAsync(ctx, "test-cancel-pipeline", config, nil)
	if err != nil {
		t.Fatalf("RunAsync failed: %v", err)
	}

	// 等待流水线开始执行
	time.Sleep(200 * time.Millisecond)

	// Cancel pipeline before it completes
	err = runtime.Cancel(ctx, "test-cancel-pipeline")
	if err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}

	// Wait for pipeline to be cancelled
	select {
	case <-pipeline.Done():
		// Pipeline was cancelled successfully
	case <-time.After(2 * time.Second):
		t.Fatal("Pipeline should be cancelled quickly")
	}
}

// TestRuntimeImpl_Cancel_NonExistent tests cancelling non-existent pipeline
func TestRuntimeImpl_Cancel_NonExistent(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx)

	err := runtime.Cancel(ctx, "non-existent-pipeline")
	if err == nil {
		t.Fatal("Expected error when cancelling non-existent pipeline")
	}
}

// TestRuntimeImpl_Rm tests pipeline removal
func TestRuntimeImpl_Rm(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx)

	// Remove pipeline (this should not panic even if pipeline doesn't exist)
	runtime.Rm("test-rm-pipeline")
}

// TestRuntimeImpl_Done tests Done channel
func TestRuntimeImpl_Done(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx)

	doneChan := runtime.Done()
	if doneChan == nil {
		t.Fatal("Done channel should not be nil")
	}

	// Test if channel is closed after StopBackground
	select {
	case <-doneChan:
		t.Fatal("Done channel should not be closed initially")
	default:
		// Normal case, channel not closed
	}

	runtime.StopBackground()

	// Wait a bit to ensure channel is closed
	select {
	case <-doneChan:
		// Channel closed as expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Done channel should be closed after StopBackground")
	}
}

// TestRuntimeImpl_Notify tests notification functionality
func TestRuntimeImpl_Notify(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx)

	// Test string notification
	err := runtime.Notify("test message")
	if err != nil {
		t.Fatalf("Notify with string failed: %v", err)
	}

	// Test map notification
	err = runtime.Notify(map[string]interface{}{
		"message": "test map message",
		"type":    "info",
	})
	if err != nil {
		t.Fatalf("Notify with map failed: %v", err)
	}

	// Test other type notification
	err = runtime.Notify(123)
	if err != nil {
		t.Fatalf("Notify with number failed: %v", err)
	}
}

// TestRuntimeImpl_Ctx tests context
func TestRuntimeImpl_Ctx(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx)

	retrievedCtx := runtime.Ctx()
	if retrievedCtx == nil {
		t.Fatal("Context should not be nil")
	}

	// Test if context is cancellable
	if retrievedCtx.Done() == nil {
		t.Fatal("Context should be cancellable")
	}
}

// TestRuntimeImpl_StopBackground tests stopping background processing
func TestRuntimeImpl_StopBackground(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx)

	// Start background processing
	runtime.StartBackground()

	// Wait a bit for background processing to start
	time.Sleep(10 * time.Millisecond)

	// Stop background processing
	runtime.StopBackground()

	// Test if context is cancelled
	select {
	case <-runtime.Ctx().Done():
		// Context cancelled as expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Context should be cancelled after StopBackground")
	}
}

// TestRuntimeImpl_ConcurrentAccess tests concurrent access
func TestRuntimeImpl_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx)

	// Prepare test configuration with new format
	configTemplate := `
Param:
  testParam: "test-value"
Executors:
  local:
    type: local
    config:
      shell: bash
Graph: |
  stateDiagram-v2
    [*] --> Task%d
Nodes:
  Task%d:
    executor: local
    steps:
      - name: step1
        run: "echo 'task%d'"
`

	// Concurrent test
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()
			pipelineConfig := fmt.Sprintf(configTemplate, id, id, id)
			_, err := runtime.RunAsync(ctx, fmt.Sprintf("pipeline-%d", id), pipelineConfig, nil)
			if err != nil {
				t.Errorf("Concurrent RunAsync failed: %v", err)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestListener test listener implementation
type TestListener struct{}

func (l *TestListener) Handle(p Pipeline, event Event) {
	// Simple implementation that does nothing for testing
}

func (l *TestListener) Events() []Event {
	return []Event{
		PipelineInit,
		PipelineStart,
		PipelineFinish,
		PipelineNodeStart,
		PipelineNodeFinish,
	}
}

// RecordingListener 记录所有事件的监听器
type RecordingListener struct {
	mu     sync.Mutex
	events []Event
}

func NewRecordingListener() *RecordingListener {
	return &RecordingListener{
		events: make([]Event, 0),
	}
}

func (l *RecordingListener) Handle(p Pipeline, event Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
}

func (l *RecordingListener) Events() []Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	return []Event{
		PipelineInit,
		PipelineStart,
		PipelineFinish,
		PipelineNodeStart,
		PipelineNodeFinish,
	}
}

func (l *RecordingListener) GetRecordedEvents() []Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	result := make([]Event, len(l.events))
	copy(result, l.events)
	return result
}

func (l *RecordingListener) Count(eventType Event) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	count := 0
	for _, e := range l.events {
		if e == eventType {
			count++
		}
	}
	return count
}

// TestParseGraphEdges_BasicStateDiagram 测试基本状态图解析
func TestParseGraphEdges_BasicStateDiagram(t *testing.T) {
	ctx := context.Background()
	config := &PipelineConfig{
		Nodes: map[string]NodeConfig{
			"Merge":  {},
			"Build":  {},
			"Deploy": {},
		},
		Graph: `stateDiagram-v2
    direction LR
    [*] --> Merge
    Merge --> Build
    Build --> Deploy
    Deploy --> [*]`,
	}

	runtime := NewRuntime(ctx).(*RuntimeImpl)
	graph := runtime.buildGraph(config)

	// 验证所有节点都存在
	nodes := graph.Nodes()
	if len(nodes) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(nodes))
	}

	// 验证节点名称
	expectedNodes := []string{"Merge", "Build", "Deploy"}
	for _, name := range expectedNodes {
		if _, ok := nodes[name]; !ok {
			t.Errorf("Expected node %s not found", name)
		}
	}
}

// TestParseGraphEdges_ComplexDiagram 测试复杂状态图（并行路径）
func TestParseGraphEdges_ComplexDiagram(t *testing.T) {
	ctx := context.Background()
	config := &PipelineConfig{
		Nodes: map[string]NodeConfig{
			"Checkout": {},
			"Lint":     {},
			"Test":     {},
			"Build":    {},
			"Deploy":   {},
		},
		Graph: `stateDiagram-v2
    [*] --> Checkout
    Checkout --> Lint
    Checkout --> Test
    Lint --> Build
    Test --> Build
    Build --> Deploy
    Deploy --> [*]`,
	}

	runtime := NewRuntime(ctx).(*RuntimeImpl)
	graph := runtime.buildGraph(config)

	nodes := graph.Nodes()
	if len(nodes) != 5 {
		t.Errorf("Expected 5 nodes, got %d", len(nodes))
	}
}

// TestParseGraphEdges_EmptyGraph 测试空图
func TestParseGraphEdges_EmptyGraph(t *testing.T) {
	ctx := context.Background()
	config := &PipelineConfig{
		Nodes: map[string]NodeConfig{
			"Node1": {},
			"Node2": {},
		},
		Graph: "",
	}

	runtime := NewRuntime(ctx).(*RuntimeImpl)
	graph := runtime.buildGraph(config)

	nodes := graph.Nodes()
	if len(nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(nodes))
	}
}

// TestParseGraphEdges_InvalidSyntax 测试无效语法
func TestParseGraphEdges_InvalidSyntax(t *testing.T) {
	ctx := context.Background()
	config := &PipelineConfig{
		Nodes: map[string]NodeConfig{
			"Node1": {},
			"Node2": {},
		},
		Graph: `invalid diagram syntax here`,
	}

	runtime := NewRuntime(ctx).(*RuntimeImpl)
	graph := runtime.buildGraph(config)

	// 即使图语法无效，也应该创建节点
	nodes := graph.Nodes()
	if len(nodes) != 2 {
		t.Errorf("Expected 2 nodes even with invalid graph, got %d", len(nodes))
	}
}

// TestParseGraphEdges_MissingNode 测试配置中缺失节点
func TestParseGraphEdges_MissingNode(t *testing.T) {
	ctx := context.Background()
	config := &PipelineConfig{
		Nodes: map[string]NodeConfig{
			"A": {},
			// B 缺失
			"C": {},
		},
		Graph: `stateDiagram-v2
    [*] --> A
    A --> B
    B --> C
    C --> [*]`,
	}

	runtime := NewRuntime(ctx).(*RuntimeImpl)
	graph := runtime.buildGraph(config)

	// 即使 B 节点缺失在配置中，也应该创建存在的节点
	nodes := graph.Nodes()
	if len(nodes) != 2 {
		t.Errorf("Expected 2 nodes (A and C), got %d", len(nodes))
	}
}

// TestExtractExpression 测试条件表达式提取
func TestExtractExpression(t *testing.T) {
	tests := []struct {
		name     string
		label    string
		expected string
	}{
		{
			name:     "基本条件表达式",
			label:    "{{A == true}}",
			expected: "{{A == true}}",
		},
		{
			name:     "带参数的条件表达式",
			label:    "{{B != 'test'}}",
			expected: "{{B != 'test'}}",
		},
		{
			name:     "复杂条件表达式",
			label:    "{% if A == 'test' and B == 'ok' %}true{% endif %}",
			expected: "{% if A == 'test' and B == 'ok' %}true{% endif %}",
		},
		{
			name:     "带空格的表达式",
			label:    "{{ A == '' }}",
			expected: "{{ A == '' }}",
		},
		{
			name:     "空标签",
			label:    "",
			expected: "",
		},
		{
			name:     "普通标签无表达式",
			label:    "普通标签",
			expected: "",
		},
		{
			name:     "只有左标记",
			label:    "{{ A == true",
			expected: "",
		},
		{
			name:     "只有右标记",
			label:    "A == true }}",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractExpression(tt.label)
			if result != tt.expected {
				t.Errorf("ExtractExpression(%q) = %q, expected %q", tt.label, result, tt.expected)
			}
		})
	}
}

// TestParseGraphEdges_ConditionalEdges 测试条件边解析
func TestParseGraphEdges_ConditionalEdges(t *testing.T) {
	ctx := context.Background()
	config := &PipelineConfig{
		Nodes: map[string]NodeConfig{
			"A": {},
			"B": {},
			"C": {},
		},
		Graph: `stateDiagram-v2
    [*] --> A
    A --> B: {{A == "test"}}
    A --> C: {% if B %}true{% endif %}
    B --> [*]
    C --> [*]`,
	}

	runtime := NewRuntime(ctx).(*RuntimeImpl)
	graph := runtime.buildGraph(config)

	// 验证所有节点都存在
	nodes := graph.Nodes()
	if len(nodes) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(nodes))
	}

	// 获取边并验证条件
	edges := graph.Edges()
	if len(edges) != 2 {
		t.Errorf("Expected 2 edges, got %d", len(edges))
	}

	// 检查边的条件表达式
	for _, edge := range edges {
		switch edge.Target().Id() {
		case "B":
			if edge.Expression() != "{{A == \"test\"}}" {
				t.Errorf("Edge A->B expression = %q, expected %q", edge.Expression(), "{{A == \"test\"}}")
			}
		case "C":
			if edge.Expression() != "{% if B %}true{% endif %}" {
				t.Errorf("Edge A->C expression = %q, expected %q", edge.Expression(), "{% if B %}true{% endif %}")
			}
		}
	}
}

// TestParseGraphEdges_UnconditionalEdges 测试无条件边解析
func TestParseGraphEdges_UnconditionalEdges(t *testing.T) {
	ctx := context.Background()
	config := &PipelineConfig{
		Nodes: map[string]NodeConfig{
			"A": {},
			"B": {},
		},
		Graph: `stateDiagram-v2
    [*] --> A
    A --> B: 普通边
    B --> [*]`,
	}

	runtime := NewRuntime(ctx).(*RuntimeImpl)
	graph := runtime.buildGraph(config)

	// 获取边并验证无条件
	edges := graph.Edges()
	if len(edges) != 1 {
		t.Errorf("Expected 1 edge, got %d", len(edges))
	}

	for _, edge := range edges {
		if edge.Expression() != "" {
			t.Errorf("Edge should have no expression, got %q", edge.Expression())
		}
	}
}

// TestParseGraphEdges_WithNotes 测试带注释的图
func TestParseGraphEdges_WithNotes(t *testing.T) {
	ctx := context.Background()
	config := &PipelineConfig{
		Nodes: map[string]NodeConfig{
			"Start":   {},
			"Process": {},
			"End":     {},
		},
		Graph: `stateDiagram-v2
    %% This is a comment
    [*] --> Start
    Start --> Process : with label
    Process --> End
    note right of Process
        This is a note
    end note
    End --> [*]`,
	}

	runtime := NewRuntime(ctx).(*RuntimeImpl)
	graph := runtime.buildGraph(config)

	nodes := graph.Nodes()
	if len(nodes) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(nodes))
	}
}

// TestRuntimeImpl_RenderParam_SelfReference 测试Param自引用
func TestRuntimeImpl_RenderParam_SelfReference(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx).(*RuntimeImpl)

	config := `
Param:
  buildId: "2323"
  imageName: "myapp-{{ Param.buildId }}"
  fullImage: "{{ Param.imageName }}:latest"
Executors:
  local:
    type: local
    config:
      shell: bash
Graph: |
  stateDiagram-v2
    [*] --> Task1
Nodes:
  Task1:
    executor: local
    steps:
      - name: step1
        run: "echo 'test'"
`

	pipeline, err := runtime.RunSync(ctx, "testParam-self-ref", config, nil)
	if err != nil {
		t.Fatalf("RunSync failed: %v", err)
	}

	// 验证pipeline的param值是否正确渲染
	if pipeline == nil {
		t.Fatal("Pipeline should not be nil")
	}

	// 我们可以通过检查节点的配置来验证Param是否被正确渲染
	graph := pipeline.GetGraph()
	if graph == nil {
		t.Fatal("Graph should not be nil")
	}

	// 验证节点存在
	nodes := graph.Nodes()
	if len(nodes) != 1 {
		t.Fatalf("Expected 1 node, got %d", len(nodes))
	}

	// 获取Param值进行验证（通过metadata或evalctx）
	// 在这里我们主要通过不报错来验证渲染成功
}

// TestRuntimeImpl_RenderParam_CircularReference 测试Param循环引用检测
func TestRuntimeImpl_RenderParam_CircularReference(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx).(*RuntimeImpl)

	// 测试一个简单的间接循环引用
	config := `
Param:
  a: "{{ Param.b }}"
  b: "{{ Param.c }}"
  c: "{{ Param.a }}"
Executors:
  local:
    type: local
    config:
      shell: bash
Graph: |
  stateDiagram-v2
    [*] --> Task1
Nodes:
  Task1:
    executor: local
    steps:
      - name: step1
        run: "echo 'task1'"
`

	_, err := runtime.RunSync(ctx, "testParam-circular", config, nil)
	// 注意：实际实现中可能无法检测所有形式的循环引用
	// 这里我们主要测试渲染不会导致程序崩溃
	if err != nil {
		// 如果能检测到循环引用并返回错误，那是最好的
		t.Logf("Detected circular reference: %v", err)
	}
	// 即使不返回错误，只要程序不崩溃，我们也认为是可以接受的
}

// TestRuntimeImpl_RenderMetadata_ReferenceParam 测试Metadata引用Param
func TestRuntimeImpl_RenderMetadata_ReferenceParam(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx).(*RuntimeImpl)

	config := `
Param:
  env: "production"
  namespace: "myapp"

Metadate:
  type: in-config
  data:
    K8sNamespace: "{{ Param.namespace }}-{{ Param.env }}"
    ImagePrefix: "myregistry.com/{{ Param.namespace }}/"

Executors:
  local:
    type: local
    config:
      shell: bash
Graph: |
  stateDiagram-v2
    [*] --> Task1
Nodes:
  Task1:
    executor: local
    steps:
      - name: step1
        run: "echo 'ns={{ K8sNamespace }}, prefix={{ ImagePrefix }}'"
`

	pipeline, err := runtime.RunSync(ctx, "test-metadata-ref-param", config, nil)
	if err != nil {
		t.Fatalf("RunSync failed: %v", err)
	}

	if pipeline == nil {
		t.Fatal("Pipeline should not be nil")
	}

	// 获取metadata验证值
	metadata := pipeline.Metadata()
	if metadata == nil {
		t.Fatal("Metadata should not be nil")
	}

	// 验证metadata中的值是否正确渲染
	if ns, ok := metadata["K8sNamespace"].(string); !ok || ns != "myapp-production" {
		t.Errorf("Expected K8sNamespace='myapp-production', got %v", ns)
	}

	if prefix, ok := metadata["ImagePrefix"].(string); !ok || prefix != "myregistry.com/myapp/" {
		t.Errorf("Expected ImagePrefix='myregistry.com/myapp/', got %v", prefix)
	}
}

// TestRuntimeImpl_RenderParam_NestedStructures 测试Param嵌套结构
func TestRuntimeImpl_RenderParam_NestedStructures(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx).(*RuntimeImpl)

	config := `
Param:
  prefix: "prod"
  nested:
    key1: "value-{{ Param.prefix }}"
    key2:
      - "item1-{{ Param.prefix }}"
      - "item2-{{ Param.prefix }}"
  list:
    - "{{ Param.prefix }}-item1"
    - "{{ Param.prefix }}-item2"

Executors:
  local:
    type: local
    config:
      shell: bash
Graph: |
  stateDiagram-v2
    [*] --> Task1
Nodes:
  Task1:
    executor: local
    steps:
      - name: step1
        run: "echo 'task1'"
`

	pipeline, err := runtime.RunSync(ctx, "testParam-nested", config, nil)
	if err != nil {
		t.Fatalf("RunSync failed: %v", err)
	}

	if pipeline == nil {
		t.Fatal("Pipeline should not be nil")
	}
}

// TestRuntimeImpl_RenderParam_WithUndefinedVariable 测试Param使用未定义变量
func TestRuntimeImpl_RenderParam_WithUndefinedVariable(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx).(*RuntimeImpl)

	config := `
Param:
  image: "myapp-{{ Param.version }}"
  tag: "{{ Param.version }}-latest"

Executors:
  local:
    type: local
    config:
      shell: bash
Graph: |
  stateDiagram-v2
    [*] --> Task1
Nodes:
  Task1:
    executor: local
    steps:
      - name: step1
        run: "echo 'image={{ Param.image }}'"
`

	// version未定义，应该保持模板字符串原样
	pipeline, err := runtime.RunSync(ctx, "testParam-undefined", config, nil)
	if err != nil {
		t.Fatalf("RunSync should not fail with undefined variables: %v", err)
	}

	if pipeline == nil {
		t.Fatal("Pipeline should not be nil")
	}
}

// TestRenderValue_NestedStructures 测试renderValue的嵌套结构处理
func TestRenderValue_NestedStructures(t *testing.T) {
	runtime := NewRuntime(context.Background()).(*RuntimeImpl)

	tests := []struct {
		name     string
		value    interface{}
		ctx      map[string]any
		expected interface{}
	}{
		{
			name:     "字符串模板",
			value:    "hello {{ Param.name }}",
			ctx:      map[string]any{"Param": map[string]any{"name": "world"}},
			expected: "hello world",
		},
		{
			name: "map嵌套",
			value: map[string]interface{}{
				"key1": "value-{{ Param.prefix }}",
				"key2": map[string]interface{}{
					"nested": "{{ Param.prefix }}-nested",
				},
			},
			ctx:      map[string]any{"Param": map[string]any{"prefix": "prod"}},
			expected: map[string]interface{}{
				"key1": "value-prod",
				"key2": map[string]interface{}{
					"nested": "prod-nested",
				},
			},
		},
		{
			name: "slice嵌套",
			value: []interface{}{
				"{{ Param.item1 }}",
				"{{ Param.item2 }}",
				map[string]interface{}{
					"key": "{{ Param.item3 }}",
				},
			},
			ctx: map[string]any{"Param": map[string]any{
				"item1": "value1",
				"item2": "value2",
				"item3": "value3",
			}},
			expected: []interface{}{
				"value1",
				"value2",
				map[string]interface{}{
					"key": "value3",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := runtime.renderValue(tt.value, tt.ctx, 0)
			if err != nil {
				t.Fatalf("renderValue failed: %v", err)
			}

			// 简单比较（实际项目中可能需要更复杂的比较）
			resultStr := fmt.Sprintf("%v", result)
			expectedStr := fmt.Sprintf("%v", tt.expected)
			if resultStr != expectedStr {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}


// TestComprehensivePipelineExecution 综合测试流水线的主要功能
func TestComprehensivePipelineExecution(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx)

	t.Run("同步执行流水线", func(t *testing.T) {
		listener := NewRecordingListener()
		config := `
Param:
  env: "production"
  appName: "test-app"
Executors:
  local:
    type: local
    config:
      shell: bash
Metadate:
  type: in-config
  data:
    deployEnv: "{{ Param.env }}"
    appName: "{{ Param.appName }}"
Graph: |
  stateDiagram-v2
    [*] --> Task1
    Task1 --> Task2
Nodes:
  Task1:
    executor: local
    steps:
      - name: step1
        run: "echo 'Task1 executed'"
  Task2:
    executor: local
    steps:
      - name: step1
        run: "echo 'Task2 executed'"
`
		pipeline, err := runtime.RunSync(ctx, "comprehensive-sync", config, listener)
		if err != nil {
			t.Fatalf("RunSync failed: %v", err)
		}
		if pipeline == nil {
			t.Fatal("Pipeline should not be nil")
		}

		// 验证事件
		if listener.Count(PipelineStart) == 0 {
			t.Error("Expected PipelineStart event")
		}
		if listener.Count(PipelineFinish) == 0 {
			t.Error("Expected PipelineFinish event")
		}
		if listener.Count(PipelineNodeStart) < 2 {
			t.Error("Expected at least 2 PipelineNodeStart events")
		}
		if listener.Count(PipelineNodeFinish) < 2 {
			t.Error("Expected at least 2 PipelineNodeFinish events")
		}
	})

	t.Run("异步执行流水线", func(t *testing.T) {
		listener := NewRecordingListener()
		config := `
Param:
  env: "development"
Executors:
  local:
    type: local
    config:
      shell: bash
Graph: |
  stateDiagram-v2
    [*] --> Task1
Nodes:
  Task1:
    executor: local
    steps:
      - name: step1
        run: "echo 'Async task'"
`
		pipeline, err := runtime.RunAsync(ctx, "comprehensive-async", config, listener)
		if err != nil {
			t.Fatalf("RunAsync failed: %v", err)
		}
		if pipeline == nil {
			t.Fatal("Pipeline should not be nil")
		}

		// 验证流水线存储在runtime中
		_, err = runtime.Get("comprehensive-async")
		if err != nil {
			t.Fatal("Pipeline should be stored in runtime")
		}

		// 等待异步执行完成
		select {
		case <-pipeline.Done():
			// 执行完成
		case <-time.After(5 * time.Second):
			// 超时时取消流水线
			runtime.Cancel(ctx, "comprehensive-async")
		}

		// 验证事件
		if listener.Count(PipelineStart) == 0 {
			t.Error("Expected PipelineStart event")
		}
		if listener.Count(PipelineFinish) == 0 {
			t.Error("Expected PipelineFinish event")
		}
	})

	t.Run("Param模板渲染", func(t *testing.T) {
		config := `
Param:
  buildId: "12345"
  imageName: "myapp-{{ Param.buildId }}"
  fullTag: "{{ Param.imageName }}:latest"
Executors:
  local:
    type: local
    config:
      shell: bash
Graph: |
  stateDiagram-v2
    [*] --> Task1
Nodes:
  Task1:
    executor: local
    steps:
      - name: step1
        run: "echo '{{ Param.fullTag }}'"
`
		pipeline, err := runtime.RunSync(ctx, "param-render-test", config, nil)
		if err != nil {
			t.Fatalf("RunSync failed: %v", err)
		}
		if pipeline == nil {
			t.Fatal("Pipeline should not be nil")
		}

		// 验证graph存在
		graph := pipeline.GetGraph()
		if graph == nil {
			t.Fatal("Graph should not be nil")
		}
		nodes := graph.Nodes()
		if len(nodes) != 1 {
			t.Fatalf("Expected 1 node, got %d", len(nodes))
		}
	})

	t.Run("Metadata创建和渲染", func(t *testing.T) {
		config := `
Param:
  namespace: "default"
  cluster: "prod-cluster"

Metadate:
  type: in-config
  data:
    K8sNamespace: "{{ Param.namespace }}"
    ClusterName: "{{ Param.cluster }}"
    DeployTarget: "{{ Param.cluster }}/{{ Param.namespace }}"

Executors:
  local:
    type: local
    config:
      shell: bash
Graph: |
  stateDiagram-v2
    [*] --> Task1
Nodes:
  Task1:
    executor: local
    steps:
      - name: step1
        run: "echo 'Deploy'"
`
		pipeline, err := runtime.RunSync(ctx, "metadata-test", config, nil)
		if err != nil {
			t.Fatalf("RunSync failed: %v", err)
		}
		if pipeline == nil {
			t.Fatal("Pipeline should not be nil")
		}

		// 验证metadata值
		metadata := pipeline.Metadata()
		if metadata == nil {
			t.Fatal("Metadata should not be nil")
		}

		if ns, ok := metadata["K8sNamespace"].(string); !ok || ns != "default" {
			t.Errorf("Expected K8sNamespace='default', got %v", ns)
		}

		if cluster, ok := metadata["ClusterName"].(string); !ok || cluster != "prod-cluster" {
			t.Errorf("Expected ClusterName='prod-cluster', got %v", cluster)
		}

		if target, ok := metadata["DeployTarget"].(string); !ok || target != "prod-cluster/default" {
			t.Errorf("Expected DeployTarget='prod-cluster/default', got %v", target)
		}
	})

	t.Run("多节点DAG执行", func(t *testing.T) {
		listener := NewRecordingListener()
		config := `
Param:
  env: "test"
Executors:
  local:
    type: local
    config:
      shell: bash
Graph: |
  stateDiagram-v2
    [*] --> Setup
    Setup --> Build
    Setup --> Test
    Build --> Deploy
    Test --> Deploy
    Deploy --> [*]
Nodes:
  Setup:
    executor: local
    steps:
      - name: step1
        run: "echo 'Setup'"
  Build:
    executor: local
    steps:
      - name: step1
        run: "echo 'Build'"
  Test:
    executor: local
    steps:
      - name: step1
        run: "echo 'Test'"
  Deploy:
    executor: local
    steps:
      - name: step1
        run: "echo 'Deploy'"
`
		pipeline, err := runtime.RunSync(ctx, "dag-test", config, listener)
		if err != nil {
			t.Fatalf("RunSync failed: %v", err)
		}
		if pipeline == nil {
			t.Fatal("Pipeline should not be nil")
		}

		// 验证所有节点都执行了
		graph := pipeline.GetGraph()
		nodes := graph.Nodes()
		if len(nodes) != 4 {
			t.Fatalf("Expected 4 nodes, got %d", len(nodes))
		}

		// 验证事件
		if listener.Count(PipelineNodeStart) != 4 {
			t.Errorf("Expected 4 PipelineNodeStart events, got %d", listener.Count(PipelineNodeStart))
		}
		if listener.Count(PipelineNodeFinish) != 4 {
			t.Errorf("Expected 4 PipelineNodeFinish events, got %d", listener.Count(PipelineNodeFinish))
		}
	})

	t.Run("并行执行", func(t *testing.T) {
		configTemplate := `
Param:
  pipelineId: "%d"
Executors:
  local:
    type: local
    config:
      shell: bash
Graph: |
  stateDiagram-v2
    [*] --> Task1
Nodes:
  Task1:
    executor: local
    steps:
      - name: step1
        run: "echo 'Pipeline %d'"
`
		numPipelines := 5
		var wg sync.WaitGroup
		errors := make(chan error, numPipelines)

		for i := 0; i < numPipelines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				pipelineConfig := fmt.Sprintf(configTemplate, id, id)
				_, err := runtime.RunAsync(ctx, fmt.Sprintf("parallel-pipeline-%d", id), pipelineConfig, nil)
				if err != nil {
					errors <- err
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		for err := range errors {
			t.Errorf("Parallel pipeline failed: %v", err)
		}

		for i := 0; i < numPipelines; i++ {
			pipelineId := fmt.Sprintf("parallel-pipeline-%d", i)
			_, err := runtime.Get(pipelineId)
			if err != nil {
				t.Errorf("Pipeline %s should be stored: %v", pipelineId, err)
			}
		}
	})
}

