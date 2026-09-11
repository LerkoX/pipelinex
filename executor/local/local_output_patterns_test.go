package local

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestExecuteCommandWithStreaming_LongLine 测试长行
func TestExecuteCommandWithStreaming_LongLine(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	longLine := strings.Repeat("x", 100000)
	err := exec.executeCommandWithStreaming(ctx, fmt.Sprintf("echo '%s'", longLine), "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) == 0 {
		t.Error("Expected some output")
	}
}

// TestExecuteCommandWithStreaming_ManyLines 测试多行
func TestExecuteCommandWithStreaming_ManyLines(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, "for i in $(seq 1 1000); do echo line$i; done", "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) < 1000 {
		t.Errorf("Expected at least 1000 lines, got %d", len(outputs))
	}
}

// TestExecuteCommandWithStreaming_RapidOutput 测试快速输出
func TestExecuteCommandWithStreaming_RapidOutput(t *testing.T) {
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
	err := exec.executeCommandWithStreaming(ctx, "for i in $(seq 1 100); do echo $i; done", "test", nil, callback, nil, nil)
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

// TestExecuteCommandWithStreaming_SlowOutput 测试慢速输出
func TestExecuteCommandWithStreaming_SlowOutput(t *testing.T) {
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
	err := exec.executeCommandWithStreaming(ctx, "for i in 1 2 3; do echo $i; sleep 0.1; done", "test", nil, callback, nil, nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if elapsed < 200*time.Millisecond {
		t.Errorf("Expected execution to take at least 200ms, took %v", elapsed)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) < 3 {
		t.Errorf("Expected at least 3 outputs, got %d", len(outputs))
	}
}

// TestExecuteCommandWithStreaming_InterleavedOutput 测试交错输出
func TestExecuteCommandWithStreaming_InterleavedOutput(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, "for i in 1 2 3; do echo out$i; echo err$i >\u00262; done", "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) < 6 {
		t.Errorf("Expected at least 6 outputs (3 stdout + 3 stderr), got %d", len(outputs))
	}
}

// TestExecuteCommandWithStreaming_PartialLine 测试部分行
func TestExecuteCommandWithStreaming_PartialLine(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, `printf 'partial'`, "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) == 0 {
		t.Error("Expected some output for partial line")
	}
}

// TestExecuteCommandWithStreaming_EmptyLines 测试空行
func TestExecuteCommandWithStreaming_EmptyLines(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, `echo ''; echo ''; echo 'done'`, "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) < 3 {
		t.Errorf("Expected at least 3 outputs (2 empty + 1 done), got %d", len(outputs))
	}
}

// TestExecuteCommandWithStreaming_WhitespaceLines 测试空白行
func TestExecuteCommandWithStreaming_WhitespaceLines(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, `echo '   '; echo 'done'`, "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) < 2 {
		t.Errorf("Expected at least 2 outputs, got %d", len(outputs))
	}
}

// TestExecuteCommandWithStreaming_TrailingNewline 测试尾部换行符
func TestExecuteCommandWithStreaming_TrailingNewline(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, `echo 'with newline'`, "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) == 0 {
		t.Error("Expected some output")
	}
}

// TestExecuteCommandWithStreaming_NoTrailingNewline 测试无尾部换行符
func TestExecuteCommandWithStreaming_NoTrailingNewline(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, `printf 'no newline'`, "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) == 0 {
		t.Error("Expected some output")
	}
}

// TestExecuteCommandWithStreaming_MultipleTrailingNewlines 测试多个尾部换行符
func TestExecuteCommandWithStreaming_MultipleTrailingNewlines(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, `printf 'done


'`, "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) < 3 {
		t.Errorf("Expected at least 3 outputs, got %d", len(outputs))
	}
}

// TestExecuteCommandWithStreaming_CarriageReturnNewline 测试 CRLF
func TestExecuteCommandWithStreaming_CarriageReturnNewline(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, `printf 'line1
line2
'`, "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) < 2 {
		t.Errorf("Expected at least 2 outputs, got %d", len(outputs))
	}
}

// TestExecuteCommandWithStreaming_MixedLineEndings 测试混合行尾
func TestExecuteCommandWithStreaming_MixedLineEndings(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, `printf 'line1
line2
line3
line4
'`, "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) < 4 {
		t.Errorf("Expected at least 4 outputs, got %d", len(outputs))
	}
}
