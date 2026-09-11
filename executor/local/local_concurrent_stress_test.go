package local

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestExecuteCommandWithStreaming_VeryLargeInput 测试非常大的输入
func TestExecuteCommandWithStreaming_VeryLargeInput(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	veryLargeInput := strings.Repeat("x", 1024*1024)
	inputChan := make(chan []byte, 1)
	go func() {
		inputChan <- []byte(veryLargeInput)
		close(inputChan)
	}()

	start := time.Now()
	err := exec.executeCommandWithStreaming(ctx, "cat", "test", nil, callback, inputChan, nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if elapsed > 30*time.Second {
		t.Errorf("Expected execution to be quick, took %v", elapsed)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) == 0 {
		t.Error("Expected some output")
	}
}

// TestExecuteCommandWithStreaming_VerySlowInput 测试非常慢速的输入
func TestExecuteCommandWithStreaming_VerySlowInput(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	inputChan := make(chan []byte, 3)
	go func() {
		for i := 1; i <= 3; i++ {
			inputChan <- []byte(fmt.Sprintf("line%d\n", i))
			time.Sleep(1 * time.Second)
		}
		close(inputChan)
	}()

	start := time.Now()
	err := exec.executeCommandWithStreaming(ctx, "cat", "test", nil, callback, inputChan, nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if elapsed < 2*time.Second {
		t.Errorf("Expected execution to take at least 2s, took %v", elapsed)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) < 3 {
		t.Errorf("Expected at least 3 outputs, got %d", len(outputs))
	}
}

// TestExecuteCommandWithStreaming_VeryFastInput 测试非常快速的输入
func TestExecuteCommandWithStreaming_VeryFastInput(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	inputChan := make(chan []byte, 1000)
	go func() {
		for i := 1; i <= 1000; i++ {
			inputChan <- []byte(fmt.Sprintf("line%d\n", i))
		}
		close(inputChan)
	}()

	start := time.Now()
	err := exec.executeCommandWithStreaming(ctx, "cat", "test", nil, callback, inputChan, nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if elapsed > 5*time.Second {
		t.Errorf("Expected execution to be quick, took %v", elapsed)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) < 1000 {
		t.Errorf("Expected at least 1000 outputs, got %d", len(outputs))
	}
}

// TestExecuteCommandWithStreaming_MixedInputOutput 测试混合输入输出
func TestExecuteCommandWithStreaming_MixedInputOutput(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	inputChan := make(chan []byte, 3)
	inputChan <- []byte("hello\n")
	inputChan <- []byte("world\n")
	inputChan <- []byte("done\n")
	close(inputChan)

	err := exec.executeCommandWithStreaming(ctx, "cat", "test", nil, callback, inputChan, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	foundHello := false
	foundWorld := false
	foundDone := false
	for _, output := range outputs {
		if strings.Contains(output, "hello") {
			foundHello = true
		}
		if strings.Contains(output, "world") {
			foundWorld = true
		}
		if strings.Contains(output, "done") {
			foundDone = true
		}
	}
	if !foundHello {
		t.Errorf("Expected output to contain 'hello', got: %v", outputs)
	}
	if !foundWorld {
		t.Errorf("Expected output to contain 'world', got: %v", outputs)
	}
	if !foundDone {
		t.Errorf("Expected output to contain 'done', got: %v", outputs)
	}
}

// TestExecuteCommandWithStreaming_ConcurrentInputOutput 测试并发输入输出
func TestExecuteCommandWithStreaming_ConcurrentInputOutput(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	inputChan := make(chan []byte, 10)
	go func() {
		for i := 1; i <= 10; i++ {
			inputChan <- []byte(fmt.Sprintf("line%d\n", i))
			time.Sleep(50 * time.Millisecond)
		}
		close(inputChan)
	}()

	err := exec.executeCommandWithStreaming(ctx, "cat", "test", nil, callback, inputChan, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) < 10 {
		t.Errorf("Expected at least 10 outputs, got %d", len(outputs))
	}
}

// TestExecuteCommandWithStreaming_LargeConcurrentInputOutput 测试大量并发输入输出
func TestExecuteCommandWithStreaming_LargeConcurrentInputOutput(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	inputChan := make(chan []byte, 100)
	go func() {
		for i := 1; i <= 100; i++ {
			inputChan <- []byte(fmt.Sprintf("line%d\n", i))
		}
		close(inputChan)
	}()

	err := exec.executeCommandWithStreaming(ctx, "cat", "test", nil, callback, inputChan, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) < 100 {
		t.Errorf("Expected at least 100 outputs, got %d", len(outputs))
	}
}

// TestExecuteCommandWithStreaming_VeryLargeConcurrentInputOutput 测试非常大量并发输入输出
func TestExecuteCommandWithStreaming_VeryLargeConcurrentInputOutput(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	inputChan := make(chan []byte, 1000)
	go func() {
		for i := 1; i <= 1000; i++ {
			inputChan <- []byte(fmt.Sprintf("line%d\n", i))
		}
		close(inputChan)
	}()

	start := time.Now()
	err := exec.executeCommandWithStreaming(ctx, "cat", "test", nil, callback, inputChan, nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if elapsed > 5*time.Second {
		t.Errorf("Expected execution to be quick, took %v", elapsed)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) < 1000 {
		t.Errorf("Expected at least 1000 outputs, got %d", len(outputs))
	}
}

// TestExecuteCommandWithStreaming_SlowConcurrentInputOutput 测试慢速并发输入输出
func TestExecuteCommandWithStreaming_SlowConcurrentInputOutput(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	inputChan := make(chan []byte, 10)
	go func() {
		for i := 1; i <= 10; i++ {
			inputChan <- []byte(fmt.Sprintf("line%d\n", i))
			time.Sleep(100 * time.Millisecond)
		}
		close(inputChan)
	}()

	start := time.Now()
	err := exec.executeCommandWithStreaming(ctx, "cat", "test", nil, callback, inputChan, nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if elapsed < 1*time.Second {
		t.Errorf("Expected execution to take at least 1s, took %v", elapsed)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) < 10 {
		t.Errorf("Expected at least 10 outputs, got %d", len(outputs))
	}
}

// TestExecuteCommandWithStreaming_FastConcurrentInputOutput 测试快速并发输入输出
func TestExecuteCommandWithStreaming_FastConcurrentInputOutput(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	inputChan := make(chan []byte, 100)
	go func() {
		for i := 1; i <= 100; i++ {
			inputChan <- []byte(fmt.Sprintf("line%d\n", i))
			time.Sleep(1 * time.Millisecond)
		}
		close(inputChan)
	}()

	start := time.Now()
	err := exec.executeCommandWithStreaming(ctx, "cat", "test", nil, callback, inputChan, nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if elapsed > 5*time.Second {
		t.Errorf("Expected execution to be quick, took %v", elapsed)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) < 100 {
		t.Errorf("Expected at least 100 outputs, got %d", len(outputs))
	}
}

// TestExecuteCommandWithStreaming_MixedConcurrentInputOutput 测试混合并发输入输出
func TestExecuteCommandWithStreaming_MixedConcurrentInputOutput(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	inputChan := make(chan []byte, 10)
	go func() {
		inputChan <- []byte("small\n")
		inputChan <- []byte(strings.Repeat("x", 1000) + "\n")
		inputChan <- []byte("medium\n")
		inputChan <- []byte(strings.Repeat("y", 10000) + "\n")
		inputChan <- []byte("large\n")
		close(inputChan)
	}()

	err := exec.executeCommandWithStreaming(ctx, "cat", "test", nil, callback, inputChan, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) < 5 {
		t.Errorf("Expected at least 5 outputs, got %d", len(outputs))
	}
}
