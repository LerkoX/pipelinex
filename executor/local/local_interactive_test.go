package local

import (
	"context"
	"strings"
	"sync"
	"testing"
)

// TestExecuteCommandWithStreaming_InteractiveCommand 测试交互式命令
func TestExecuteCommandWithStreaming_InteractiveCommand(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	inputChan := make(chan []byte, 2)
	inputChan <- []byte("yes\n")
	inputChan <- []byte("no\n")
	close(inputChan)

	err := exec.executeCommandWithStreaming(ctx, "cat", "test", nil, callback, inputChan, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	foundYes := false
	foundNo := false
	for _, output := range outputs {
		if strings.Contains(output, "yes") {
			foundYes = true
		}
		if strings.Contains(output, "no") {
			foundNo = true
		}
	}
	if !foundYes {
		t.Errorf("Expected output to contain 'yes', got: %v", outputs)
	}
	if !foundNo {
		t.Errorf("Expected output to contain 'no', got: %v", outputs)
	}
}

// TestExecuteCommandWithStreaming_BackgroundProcess 测试后台进程
func TestExecuteCommandWithStreaming_BackgroundProcess(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, "(sleep 1 &) && echo done", "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	foundDone := false
	for _, output := range outputs {
		if strings.Contains(output, "done") {
			foundDone = true
			break
		}
	}
	if !foundDone {
		t.Errorf("Expected output to contain 'done', got: %v", outputs)
	}
}

// TestExecuteCommandWithStreaming_SubShell 测试子 shell
func TestExecuteCommandWithStreaming_SubShell(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, "(echo sub1; echo sub2)", "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	foundSub1 := false
	foundSub2 := false
	for _, output := range outputs {
		if strings.Contains(output, "sub1") {
			foundSub1 = true
		}
		if strings.Contains(output, "sub2") {
			foundSub2 = true
		}
	}
	if !foundSub1 {
		t.Errorf("Expected output to contain 'sub1', got: %v", outputs)
	}
	if !foundSub2 {
		t.Errorf("Expected output to contain 'sub2', got: %v", outputs)
	}
}

// TestExecuteCommandWithStreaming_CommandSubstitution 测试命令替换
func TestExecuteCommandWithStreaming_CommandSubstitution(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, "echo $(echo replaced)", "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	foundReplaced := false
	for _, output := range outputs {
		if strings.Contains(output, "replaced") {
			foundReplaced = true
			break
		}
	}
	if !foundReplaced {
		t.Errorf("Expected output to contain 'replaced', got: %v", outputs)
	}
}

// TestExecuteCommandWithStreaming_VariableExpansion 测试变量扩展
func TestExecuteCommandWithStreaming_VariableExpansion(t *testing.T) {
	exec := NewLocalExecutor()
	exec.setEnv("MY_VAR", "expanded")

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, "echo $MY_VAR", "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	foundExpanded := false
	for _, output := range outputs {
		if strings.Contains(output, "expanded") {
			foundExpanded = true
			break
		}
	}
	if !foundExpanded {
		t.Errorf("Expected output to contain 'expanded', got: %v", outputs)
	}
}

// TestExecuteCommandWithStreaming_GlobPattern 测试 glob 模式
func TestExecuteCommandWithStreaming_GlobPattern(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, "ls /tmp/* 2>/dev/null || echo 'no files'", "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) == 0 {
		t.Error("Expected some output")
	}
}
