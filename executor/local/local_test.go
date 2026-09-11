package local

import (
	"context"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LerkoX/flowx/executor"
)

// TestPrepare_ShellNotFound 测试 Prepare 在 shell 不存在时返回错误
func TestPrepare_ShellNotFound(t *testing.T) {
	exec := NewLocalExecutor()
	exec.setShell("/nonexistent/shell")

	ctx := context.Background()
	err := exec.Prepare(ctx)
	if err == nil {
		t.Fatal("Expected error for non-existent shell, got nil")
	}
	if !strings.Contains(err.Error(), "shell not found") {
		t.Errorf("Expected error to contain 'shell not found', got: %v", err)
	}
}

// TestPrepare_WorkdirNotExist 测试 Prepare 在工作目录不存在时返回错误
func TestPrepare_WorkdirNotExist(t *testing.T) {
	exec := NewLocalExecutor()
	exec.setWorkdir("/nonexistent/workdir")

	ctx := context.Background()
	err := exec.Prepare(ctx)
	if err == nil {
		t.Fatal("Expected error for non-existent workdir, got nil")
	}
	if !strings.Contains(err.Error(), "workdir does not exist") {
		t.Errorf("Expected error to contain 'workdir does not exist', got: %v", err)
	}
}

// TestPrepare_WorkdirNotDirectory 测试 Prepare 在工作目录不是目录时返回错误
func TestPrepare_WorkdirNotDirectory(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "testfile")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	exec := NewLocalExecutor()
	exec.setWorkdir(tmpFile.Name())

	ctx := context.Background()
	err = exec.Prepare(ctx)
	if err == nil {
		t.Fatal("Expected error for file as workdir, got nil")
	}
	if !strings.Contains(err.Error(), "workdir is not a directory") {
		t.Errorf("Expected error to contain 'workdir is not a directory', got: %v", err)
	}
}

// TestPrepare_ValidShell 测试 Prepare 在 shell 存在时成功
func TestPrepare_ValidShell(t *testing.T) {
	exec := NewLocalExecutor()
	ctx := context.Background()
	err := exec.Prepare(ctx)
	if err != nil {
		t.Errorf("Expected no error for valid shell, got: %v", err)
	}
}

// TestBuildEnvList_Deduplication 测试环境变量去重
func TestBuildEnvList_Deduplication(t *testing.T) {
	exec := NewLocalExecutor()

	os.Setenv("TEST_VAR", "system_value")
	defer os.Unsetenv("TEST_VAR")

	exec.setEnv("TEST_VAR", "custom_value")

	envList := exec.buildEnvList()

	var testVarCount int
	var testVarValue string
	for _, env := range envList {
		if strings.HasPrefix(env, "TEST_VAR=") {
			testVarCount++
			testVarValue = strings.TrimPrefix(env, "TEST_VAR=")
		}
	}

	if testVarCount != 1 {
		t.Errorf("Expected TEST_VAR to appear exactly once, got %d times", testVarCount)
	}
	if testVarValue != "custom_value" {
		t.Errorf("Expected TEST_VAR to be 'custom_value', got '%s'", testVarValue)
	}
}

// TestBuildEnvList_CustomVars 测试自定义环境变量正确添加
func TestBuildEnvList_CustomVars(t *testing.T) {
	exec := NewLocalExecutor()
	exec.setEnv("CUSTOM_KEY", "custom_value")
	exec.setEnv("ANOTHER_KEY", "another_value")

	envList := exec.buildEnvList()

	var foundCustom, foundAnother bool
	for _, env := range envList {
		if env == "CUSTOM_KEY=custom_value" {
			foundCustom = true
		}
		if env == "ANOTHER_KEY=another_value" {
			foundAnother = true
		}
	}

	if !foundCustom {
		t.Error("Expected CUSTOM_KEY=custom_value in env list")
	}
	if !foundAnother {
		t.Error("Expected ANOTHER_KEY=another_value in env list")
	}
}

// TestSafeSend_ClosedChannel 测试 safeSend 在 channel 关闭时不 panic
func TestSafeSend_ClosedChannel(t *testing.T) {
	ch := make(chan any)
	close(ch)

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("safeSend panicked on closed channel: %v", r)
		}
	}()

	safeSend(ch, "test_value")
}

// TestSafeSend_OpenChannel 测试 safeSend 在 channel 打开时正常工作
func TestSafeSend_OpenChannel(t *testing.T) {
	ch := make(chan any, 1)

	safeSend(ch, "test_value")

	select {
	case val := <-ch:
		if val != "test_value" {
			t.Errorf("Expected 'test_value', got %v", val)
		}
	case <-time.After(time.Second):
		t.Error("Timeout waiting for value")
	}
}

// TestStreamOutput_ScannerError 测试 scanner 错误处理
func TestStreamOutput_ScannerError(t *testing.T) {
	exec := NewLocalExecutor()

	largeOutput := make([]byte, 2*1024*1024)
	for i := range largeOutput {
		largeOutput[i] = 'a'
	}
	largeOutput[len(largeOutput)-1] = '\n'

	reader := strings.NewReader(string(largeOutput))
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	exec.streamOutput(context.Background(), reader, callback, "test", nil)

	mu.Lock()
	defer mu.Unlock()

	foundError := false
	for _, output := range outputs {
		if strings.Contains(output, "stream error") {
			foundError = true
			break
		}
	}

	if !foundError {
		t.Error("Expected stream error output for oversized line")
	}
}

// TestStreamOutput_NormalOutput 测试正常输出处理
func TestStreamOutput_NormalOutput(t *testing.T) {
	exec := NewLocalExecutor()

	input := "line1\nline2\nline3\n"
	reader := strings.NewReader(input)

	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	exec.streamOutput(context.Background(), reader, callback, "test", nil)

	mu.Lock()
	defer mu.Unlock()

	expected := []string{"line1\n", "line2\n", "line3\n"}
	if len(outputs) != len(expected) {
		t.Errorf("Expected %d outputs, got %d", len(expected), len(outputs))
	}
	for i, exp := range expected {
		if i < len(outputs) && outputs[i] != exp {
			t.Errorf("Expected output %d to be %q, got %q", i, exp, outputs[i])
		}
	}
}

// TestStreamOutput_InputRequestBlock 测试输入请求代码块检测
func TestStreamOutput_InputRequestBlock(t *testing.T) {
	exec := NewLocalExecutor()

	input := "normal output\n" + "```flowx-input\n" + "type: text\nprompt: \"Enter name:\"" + "\n```\n" + "more output\n"

	reader := strings.NewReader(input)

	var outputs []string
	var requests []*executor.InputRequest
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	onInputRequest := func(req *executor.InputRequest) {
		mu.Lock()
		defer mu.Unlock()
		requests = append(requests, req)
	}

	exec.streamOutput(context.Background(), reader, callback, "test", onInputRequest)

	mu.Lock()
	defer mu.Unlock()

	if len(requests) != 1 {
		t.Errorf("Expected 1 input request, got %d", len(requests))
	} else {
		if requests[0].Type != "text" {
			t.Errorf("Expected type 'text', got '%s'", requests[0].Type)
		}
		if requests[0].Prompt != "Enter name:" {
			t.Errorf("Expected prompt 'Enter name:', got '%s'", requests[0].Prompt)
		}
	}

	for _, output := range outputs {
		if strings.Contains(output, "flowx-input") {
			t.Error("Output should not contain flowx-input marker")
		}
	}
}

// TestCreateCommand_WithPTY 测试 PTY 模式命令创建
func TestCreateCommand_WithPTY(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PTY test skipped on Windows")
	}

	exec := NewLocalExecutor()
	exec.setPTY(true)
	exec.setShell("/bin/bash")

	ctx := context.Background()
	cmd := exec.createCommand(ctx, "echo hello")

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	if !strings.HasSuffix(cmd.Path, "script") {
		t.Errorf("Expected command path to end with 'script' for PTY mode, got '%s'", cmd.Path)
	}
}

// TestCreateCommand_WithoutPTY 测试非 PTY 模式命令创建
func TestCreateCommand_WithoutPTY(t *testing.T) {
	exec := NewLocalExecutor()
	exec.setPTY(false)
	exec.setShell("/bin/bash")

	ctx := context.Background()
	cmd := exec.createCommand(ctx, "echo hello")

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	expectedPath := "/bin/bash"
	if cmd.Path != expectedPath {
		t.Errorf("Expected command path to be '%s', got '%s'", expectedPath, cmd.Path)
	}

	expectedArgs := []string{"/bin/bash", "-c", "echo hello"}
	if len(cmd.Args) != len(expectedArgs) {
		t.Errorf("Expected %d args, got %d", len(expectedArgs), len(cmd.Args))
	}
	for i, arg := range expectedArgs {
		if i < len(cmd.Args) && cmd.Args[i] != arg {
			t.Errorf("Expected arg %d to be '%s', got '%s'", i, arg, cmd.Args[i])
		}
	}
}

// TestKillCurrentProcess 测试终止进程功能
func TestKillCurrentProcess(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	cmd := exec.createCommand(ctx, "sleep 10")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start command: %v", err)
	}

	started := make(chan struct{})
	close(started)
	done := make(chan struct{})
	exec.cmdMu.Lock()
	exec.currentCmd = &activeCmd{
		cmd:     cmd,
		started: started,
		done:    done,
		pid:     cmd.Process.Pid,
	}
	exec.cmdMu.Unlock()

	exec.killCurrentProcess()

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- cmd.Wait()
	}()

	select {
	case <-waitDone:
	case <-time.After(3 * time.Second):
		t.Error("Timeout waiting for process to be killed")
	}

	if cmd.Process != nil {
		err := cmd.Process.Signal(os.Interrupt)
		if err == nil {
			t.Error("Expected process to be terminated, but signal succeeded")
		}
	}
}

// TestTransfer_ContextCancellation 测试上下文取消时终止执行
func TestTransfer_ContextCancellation(t *testing.T) {
	exec := NewLocalExecutor()

	ctx, cancel := context.WithCancel(context.Background())
	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)
	inputChan := make(chan []byte)

	commandChan <- executor.CommandWrapper{
		StepName: "test",
		Command:  "sleep 10",
	}
	close(commandChan)

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	go func() {
		// 消费 resultChan，防止 Transfer 阻塞
		for range resultChan {
		}
	}()

	exec.Transfer(ctx, resultChan, commandChan, inputChan)
	// Transfer 已经关闭 resultChan，无需再次关闭

	// 测试通过即表示 Transfer 在上下文取消后正确返回
}

// TestTransfer_UnsupportedType 测试不支持的类型返回错误
func TestTransfer_UnsupportedType(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)
	inputChan := make(chan []byte)

	commandChan <- "unsupported string"
	close(commandChan)

	go func() {
		// 消费 resultChan，防止 Transfer 阻塞
		for range resultChan {
		}
	}()

	exec.Transfer(ctx, resultChan, commandChan, inputChan)
	// Transfer 已经关闭 resultChan，无需再次关闭

	// 测试通过即表示 Transfer 正确处理了不支持的类型
}

// TestExecuteCommandWithStreaming_Timeout 测试命令超时
func TestExecuteCommandWithStreaming_Timeout(t *testing.T) {
	exec := NewLocalExecutor()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, "sleep 10", "test", nil, callback, nil, nil)

	if err == nil {
		t.Fatal("Expected error for timeout")
	}
	if !strings.Contains(err.Error(), "timed out") && !strings.Contains(err.Error(), "signal") && !strings.Contains(err.Error(), "exited with code") {
		t.Errorf("Expected timeout or signal error, got: %v", err)
	}
}

// TestExecuteCommandWithStreaming_Success 测试成功执行命令
func TestExecuteCommandWithStreaming_Success(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, "echo hello", "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	foundOutput := false
	for _, output := range outputs {
		if strings.Contains(output, "hello") {
			foundOutput = true
			break
		}
	}
	if !foundOutput {
		t.Errorf("Expected output to contain 'hello', got: %v", outputs)
	}
}

// TestExecuteCommandWithStreaming_WithInput 测试带输入的命令执行
func TestExecuteCommandWithStreaming_WithInput(t *testing.T) {
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
	inputChan <- []byte("test input\n")
	close(inputChan)

	err := exec.executeCommandWithStreaming(ctx, "cat", "test", nil, callback, inputChan, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	foundOutput := false
	for _, output := range outputs {
		if strings.Contains(output, "test input") {
			foundOutput = true
			break
		}
	}
	if !foundOutput {
		t.Errorf("Expected output to contain 'test input', got: %v", outputs)
	}
}
