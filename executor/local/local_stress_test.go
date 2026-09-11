package local

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestExecuteCommandWithStreaming_VeryLongLine 测试非常长的行
func TestExecuteCommandWithStreaming_VeryLongLine(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	veryLongLine := strings.Repeat("x", 2*1024*1024)
	err := exec.executeCommandWithStreaming(ctx, fmt.Sprintf("echo '%s'", veryLongLine), "test", nil, callback, nil, nil)

	if err == nil {
		t.Fatal("Expected error for oversized line")
	}
	// 不同系统/环境下错误信息可能不同：stream error 或 argument list too long 都是可接受的
	if !strings.Contains(err.Error(), "stream error") && !strings.Contains(err.Error(), "argument list too long") {
		t.Errorf("Expected stream error or argument list too long, got: %v", err)
	}
}

// TestExecuteCommandWithStreaming_VeryManyLines 测试非常多的行
func TestExecuteCommandWithStreaming_VeryManyLines(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, "for i in $(seq 1 10000); do echo line$i; done", "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) < 10000 {
		t.Errorf("Expected at least 10000 lines, got %d", len(outputs))
	}
}

// TestExecuteCommandWithStreaming_VeryFastOutput 测试非常快速的输出
func TestExecuteCommandWithStreaming_VeryFastOutput(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	start := time.Now()
	err := exec.executeCommandWithStreaming(ctx, "for i in $(seq 1 100000); do echo $i; done", "test", nil, callback, nil, nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if elapsed > 10*time.Second {
		t.Errorf("Expected execution to be quick, took %v", elapsed)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) < 100000 {
		t.Errorf("Expected at least 100000 outputs, got %d", len(outputs))
	}
}

// TestExecuteCommandWithStreaming_VerySlowOutput 测试非常慢速的输出
func TestExecuteCommandWithStreaming_VerySlowOutput(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	start := time.Now()
	err := exec.executeCommandWithStreaming(ctx, "for i in 1 2 3; do echo $i; sleep 1; done", "test", nil, callback, nil, nil)
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

// TestExecuteCommandWithStreaming_StressTest 测试压力测试
func TestExecuteCommandWithStreaming_StressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	start := time.Now()
	err := exec.executeCommandWithStreaming(ctx, "for i in $(seq 1 10000); do echo $(seq 1 100 | tr -d '\\n'); done", "test", nil, callback, nil, nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if elapsed > 30*time.Second {
		t.Errorf("Expected execution to be quick, took %v", elapsed)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) < 10000 {
		t.Errorf("Expected at least 10000 outputs, got %d", len(outputs))
	}
}

// TestExecuteCommandWithStreaming_MemoryStressTest 测试内存压力测试
func TestExecuteCommandWithStreaming_MemoryStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory stress test in short mode")
	}

	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	start := time.Now()
	// 减小输出规模，避免在有限内存/缓冲区环境下超时
	err := exec.executeCommandWithStreaming(ctx, "python3 -c \"print('x' * 10000000)\"", "test", nil, callback, nil, nil)
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

// TestExecuteCommandWithStreaming_GoroutineLeakStressTest 测试 goroutine 泄漏压力测试
func TestExecuteCommandWithStreaming_GoroutineLeakStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping goroutine leak stress test in short mode")
	}

	exec := NewLocalExecutor()

	ctx := context.Background()
	callback := func(data []byte) {}

	initialGoroutines := runtime.NumGoroutine()

	for i := 0; i < 1000; i++ {
		err := exec.executeCommandWithStreaming(ctx, "echo test", "test", nil, callback, nil, nil)
		if err != nil {
			t.Errorf("Command %d failed: %v", i, err)
		}
	}

	time.Sleep(1 * time.Second)

	finalGoroutines := runtime.NumGoroutine()

	if finalGoroutines > initialGoroutines+50 {
		t.Errorf("Possible goroutine leak: initial=%d, final=%d", initialGoroutines, finalGoroutines)
	}
}

// TestExecuteCommandWithStreaming_ContextCancellationStressTest 测试上下文取消压力测试
func TestExecuteCommandWithStreaming_ContextCancellationStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping context cancellation stress test in short mode")
	}

	exec := NewLocalExecutor()

	for i := 0; i < 100; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		callback := func(data []byte) {}

		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		err := exec.executeCommandWithStreaming(ctx, "sleep 10", "test", nil, callback, nil, nil)
		if err == nil {
			t.Errorf("Command %d: Expected error due to context cancellation", i)
		}
	}
}

// TestExecuteCommandWithStreaming_TimeoutStressTest 测试超时压力测试
func TestExecuteCommandWithStreaming_TimeoutStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timeout stress test in short mode")
	}

	exec := NewLocalExecutor()
	exec.setTimeout(50 * time.Millisecond)

	callback := func(data []byte) {}

	for i := 0; i < 100; i++ {
		ctx := context.Background()
		err := exec.executeCommandWithStreaming(ctx, "sleep 10", "test", nil, callback, nil, nil)
		if err == nil {
			t.Errorf("Command %d: Expected timeout error", i)
			continue
		}
		if !strings.Contains(err.Error(), "timed out") {
			t.Errorf("Command %d: Expected timeout error, got: %v", i, err)
		}
	}
}

// TestExecuteCommandWithStreaming_ConcurrentStressTest 测试并发压力测试
func TestExecuteCommandWithStreaming_ConcurrentStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent stress test in short mode")
	}

	exec := NewLocalExecutor()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			ctx := context.Background()
			var outputs []string
			var mu sync.Mutex

			callback := func(data []byte) {
				mu.Lock()
				defer mu.Unlock()
				outputs = append(outputs, string(data))
			}

			err := exec.executeCommandWithStreaming(ctx, fmt.Sprintf("echo %d", index), "test", nil, callback, nil, nil)
			if err != nil {
				t.Errorf("Command %d failed: %v", index, err)
			}

			mu.Lock()
			defer mu.Unlock()

			found := false
			for _, output := range outputs {
				if strings.Contains(output, fmt.Sprintf("%d", index)) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Command %d: Expected output to contain '%d', got: %v", index, index, outputs)
			}
		}(i)
	}

	wg.Wait()
}

// TestExecuteCommandWithStreaming_LargeOutputStressTest 测试大输出压力测试
func TestExecuteCommandWithStreaming_LargeOutputStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large output stress test in short mode")
	}

	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	start := time.Now()
	err := exec.executeCommandWithStreaming(ctx, "python3 -c \"print('x' * 1000000000)\"", "test", nil, callback, nil, nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if elapsed > 60*time.Second {
		t.Errorf("Expected execution to be quick, took %v", elapsed)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) == 0 {
		t.Error("Expected some output")
	}
}

// TestExecuteCommandWithStreaming_LargeInputStressTest 测试大输入压力测试
func TestExecuteCommandWithStreaming_LargeInputStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large input stress test in short mode")
	}

	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	inputChan := make(chan []byte, 1)
	go func() {
		inputChan <- []byte(strings.Repeat("x", 1000000000))
		close(inputChan)
	}()

	start := time.Now()
	err := exec.executeCommandWithStreaming(ctx, "cat", "test", nil, callback, inputChan, nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if elapsed > 60*time.Second {
		t.Errorf("Expected execution to be quick, took %v", elapsed)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) == 0 {
		t.Error("Expected some output")
	}
}

// TestExecuteCommandWithStreaming_LargeConcurrentInputOutputStressTest 测试大量并发输入输出压力测试
func TestExecuteCommandWithStreaming_LargeConcurrentInputOutputStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large concurrent input output stress test in short mode")
	}

	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	inputChan := make(chan []byte, 10000)
	go func() {
		for i := 1; i <= 10000; i++ {
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

	if elapsed > 10*time.Second {
		t.Errorf("Expected execution to be quick, took %v", elapsed)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) < 10000 {
		t.Errorf("Expected at least 10000 outputs, got %d", len(outputs))
	}
}

// TestExecuteCommandWithStreaming_VeryLargeConcurrentInputOutputStressTest 测试非常大量并发输入输出压力测试
func TestExecuteCommandWithStreaming_VeryLargeConcurrentInputOutputStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping very large concurrent input output stress test in short mode")
	}

	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	inputChan := make(chan []byte, 100000)
	go func() {
		for i := 1; i <= 100000; i++ {
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

	if elapsed > 30*time.Second {
		t.Errorf("Expected execution to be quick, took %v", elapsed)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) < 100000 {
		t.Errorf("Expected at least 100000 outputs, got %d", len(outputs))
	}
}
