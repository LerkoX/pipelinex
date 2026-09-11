package local

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestExecuteCommandWithStreaming_ShellScript 测试 shell 脚本执行
func TestExecuteCommandWithStreaming_ShellScript(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	script := "for i in 1 2 3; do\n    echo \"line $i\"\ndone"

	err := exec.executeCommandWithStreaming(ctx, script, "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	for i := 1; i <= 3; i++ {
		expected := fmt.Sprintf("line %d", i)
		found := false
		for _, output := range outputs {
			if strings.Contains(output, expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected output to contain '%s', got: %v", expected, outputs)
		}
	}
}

// TestExecuteCommandWithStreaming_Workflow 测试管道命令
func TestExecuteCommandWithStreaming_Workflow(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, "echo 'hello world' | wc -w", "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	foundCount := false
	for _, output := range outputs {
		if strings.Contains(output, "2") {
			foundCount = true
			break
		}
	}
	if !foundCount {
		t.Errorf("Expected output to contain word count, got: %v", outputs)
	}
}

// TestExecuteCommandWithStreaming_Redirection 测试重定向
func TestExecuteCommandWithStreaming_Redirection(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, "echo 'redirected' | cat", "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	foundRedirected := false
	for _, output := range outputs {
		if strings.Contains(output, "redirected") {
			foundRedirected = true
			break
		}
	}
	if !foundRedirected {
		t.Errorf("Expected output to contain 'redirected', got: %v", outputs)
	}
}

// TestExecuteCommandWithStreaming_SignalHandling 测试信号处理
func TestExecuteCommandWithStreaming_SignalHandling(t *testing.T) {
	exec := NewLocalExecutor()

	ctx, cancel := context.WithCancel(context.Background())
	callback := func(data []byte) {}

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	err := exec.executeCommandWithStreaming(ctx, "sleep 10", "test", nil, callback, nil, nil)

	if err == nil {
		t.Fatal("Expected error due to context cancellation")
	}
}

// TestExecuteCommandWithStreaming_ResourceCleanup 测试资源清理
func TestExecuteCommandWithStreaming_ResourceCleanup(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	callback := func(data []byte) {}

	for i := 0; i < 50; i++ {
		err := exec.executeCommandWithStreaming(ctx, "echo test", "test", nil, callback, nil, nil)
		if err != nil {
			t.Errorf("Command %d failed: %v", i, err)
		}
	}

	exec.mu.Lock()
	defer exec.mu.Unlock()

	if exec.currentCmd != nil {
		t.Error("Expected currentCmd to be nil after execution")
	}
}

// TestExecuteCommandWithStreaming_LargeStderr 测试大量 stderr 输出
func TestExecuteCommandWithStreaming_LargeStderr(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, "for i in $(seq 1 100); do echo error >&2; done; exit 0", "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error (exit 0), got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) < 50 {
		t.Errorf("Expected at least 50 stderr outputs, got %d", len(outputs))
	}
}

// TestExecuteCommandWithStreaming_MixedOutput 测试混合输出
func TestExecuteCommandWithStreaming_MixedOutput(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, "echo out1 && echo err1 >&2 && echo out2 && echo err2 >&2", "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) < 4 {
		t.Errorf("Expected at least 4 outputs (2 stdout + 2 stderr), got %d", len(outputs))
	}
}
