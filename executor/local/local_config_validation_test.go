package local

import (
	"context"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// TestExecuteCommandWithStreaming_Workdir 测试工作目录设置
func TestExecuteCommandWithStreaming_Workdir(t *testing.T) {
	exec := NewLocalExecutor()
	exec.setWorkdir("/tmp")

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, "pwd", "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	foundTmp := false
	for _, output := range outputs {
		if strings.Contains(output, "/tmp") {
			foundTmp = true
			break
		}
	}
	if !foundTmp {
		t.Errorf("Expected output to contain '/tmp', got: %v", outputs)
	}
}

// TestExecuteCommandWithStreaming_EnvVars 测试环境变量传递
func TestExecuteCommandWithStreaming_EnvVars(t *testing.T) {
	exec := NewLocalExecutor()
	exec.setEnv("TEST_VAR", "test_value")
	exec.setEnv("ANOTHER_VAR", "another_value")

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, "echo $TEST_VAR $ANOTHER_VAR", "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	foundVars := false
	for _, output := range outputs {
		if strings.Contains(output, "test_value") && strings.Contains(output, "another_value") {
			foundVars = true
			break
		}
	}
	if !foundVars {
		t.Errorf("Expected output to contain environment variables, got: %v", outputs)
	}
}

// TestExecuteCommandWithStreaming_ShellSetting 测试 shell 设置
func TestExecuteCommandWithStreaming_ShellSetting(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Shell setting test skipped on Windows")
	}

	exec := NewLocalExecutor()
	exec.setShell("/bin/sh")

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, "echo $0", "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	foundSh := false
	for _, output := range outputs {
		if strings.Contains(output, "/bin/sh") {
			foundSh = true
			break
		}
	}
	if !foundSh {
		t.Errorf("Expected output to contain '/bin/sh', got: %v", outputs)
	}
}

// TestStreamOutput_NilCallback 测试 nil callback
func TestStreamOutput_NilCallback(t *testing.T) {
	exec := NewLocalExecutor()

	input := "some output\n"
	reader := strings.NewReader(input)

	exec.streamOutput(context.Background(), reader, nil, "test", nil)
}

// TestStreamOutput_NilOnInputRequest 测试 nil onInputRequest
func TestStreamOutput_NilOnInputRequest(t *testing.T) {
	exec := NewLocalExecutor()

	input := "```flowx-input\ntype: text\n```\n"
	reader := strings.NewReader(input)

	var outputs []string
	callback := func(data []byte) {
		outputs = append(outputs, string(data))
	}

	exec.streamOutput(context.Background(), reader, callback, "test", nil)

	if len(outputs) != 0 {
		t.Errorf("Expected no output when onInputRequest is nil, got %d items", len(outputs))
	}
}

// TestExecuteCommandWithStreaming_InvalidCommand 测试命令不存在
func TestExecuteCommandWithStreaming_InvalidCommand(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	callback := func(data []byte) {}

	err := exec.executeCommandWithStreaming(ctx, "/nonexistent/command/that/does/not/exist", "test", nil, callback, nil, nil)

	if err == nil {
		t.Fatal("Expected error for non-existent command")
	}
}

// TestExecuteCommandWithStreaming_InvalidShell 测试无效 shell
func TestExecuteCommandWithStreaming_InvalidShell(t *testing.T) {
	exec := NewLocalExecutor()
	exec.setShell("/nonexistent/shell")

	ctx := context.Background()
	callback := func(data []byte) {}

	err := exec.executeCommandWithStreaming(ctx, "echo test", "test", nil, callback, nil, nil)

	if err == nil {
		t.Fatal("Expected error for invalid shell")
	}
}
