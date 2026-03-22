package pipelinex

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestInConfigMetadataStore_ThreadSafety 测试 InConfigMetadataStore 线程安全性
func TestInConfigMetadataStore_ThreadSafety(t *testing.T) {
	ctx := context.Background()
	config := MetadataConfig{
		Type: "in-config",
		Data: map[string]interface{}{
			"key1": "value1",
		},
	}

	store, err := NewInConfigMetadataStore(config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// 并发写入测试
	const numGoroutines = 100
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(n int) {
			key := fmt.Sprintf("key%d", n)
			value := fmt.Sprintf("value%d", n)
			err := store.Set(ctx, key, value)
			if err != nil {
				t.Errorf("Failed to set %s: %v", key, err)
			}
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// 验证数据
	for i := 0; i < numGoroutines; i++ {
		key := fmt.Sprintf("key%d", i)
		expected := fmt.Sprintf("value%d", i)
		actual, err := store.Get(ctx, key)
		if err != nil {
			t.Errorf("Failed to get %s: %v", key, err)
		}
		if actual != expected {
			t.Errorf("Expected %s=%s, got %s", key, expected, actual)
		}
	}
}

// TestInConfigMetadataStore_Delete 测试 Delete 功能
func TestInConfigMetadataStore_Delete(t *testing.T) {
	ctx := context.Background()
	config := MetadataConfig{
		Type: "in-config",
		Data: map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		},
	}

	store, err := NewInConfigMetadataStore(config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// 删除一个键
	err = store.Delete(ctx, "key1")
	if err != nil {
		t.Fatalf("Failed to delete key1: %v", err)
	}

	// 验证 key1 已删除
	_, err = store.Get(ctx, "key1")
	if err == nil {
		t.Error("Expected error when getting deleted key1")
	}

	// 验证 key2 仍然存在
	val, err := store.Get(ctx, "key2")
	if err != nil {
		t.Errorf("Failed to get key2: %v", err)
	}
	if val != "value2" {
		t.Errorf("Expected key2=value2, got %s", val)
	}
}

// TestExtractOutput_Basic 测试基本的输出提取功能
func TestExtractOutput_Basic(t *testing.T) {
	ctx := context.Background()
	pipeline := &PipelineImpl{
		id:        "test-pipeline",
		executors:  make(map[string]Executor),
		metadata:   make(Metadata),
		param:      make(map[string]interface{}),
		templateEngine: NewPongo2TemplateEngine(),
	}

	// 创建 mock node
	nodeConfig := map[string]interface{}{
		"extract": map[string]interface{}{
			"type": "codec-block",
		},
	}

	node := NewDGANodeWithConfig("TestNode", "", "", "", []Step{}, nodeConfig)

	// 创建模拟的 StepResult
	stepResult := &StepResult{
		StepName:   "test-step",
		Command:    "echo test",
		Output:     "test output",
		StartTime:  time.Now(),
		FinishTime: time.Now(),
	}

	// 完整输出包含 codec-block
	fullOutput := "Regular output\n```pipelinex-json\n{\"extracted\": \"value123\", \"count\": 42}\n```\nMore output"

	// 执行提取
	err := pipeline.extractOutput(ctx, node, stepResult, fullOutput)
	if err != nil {
		t.Fatalf("Failed to extract output: %v", err)
	}

	// 验证 metadata
	metadata := pipeline.Metadata()

	if extracted, ok := metadata["TestNode.extracted"]; !ok || extracted != "value123" {
		t.Errorf("Expected TestNode.extracted=value123, got %v (ok: %v)", extracted, ok)
	}

	// JSON 解析后的数字可能是 int 或 float64
	if count, ok := metadata["TestNode.count"]; !ok {
		t.Errorf("Expected TestNode.count=42, got missing (ok: %v)", ok)
	} else if countInt, ok := count.(int64); ok && countInt != 42 {
		t.Errorf("Expected TestNode.count=42 (int64), got %v", countInt)
	} else if countInt, ok := count.(int); ok && countInt != 42 {
		t.Errorf("Expected TestNode.count=42 (int), got %v", countInt)
	} else if countInt, ok := count.(float64); ok && countInt != 42 {
		t.Errorf("Expected TestNode.count=42 (float64), got %v", countInt)
	}
}
