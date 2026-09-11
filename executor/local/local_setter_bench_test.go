package local

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LerkoX/flowx/executor"
)

// TestSettersAndGetters 测试设置器和获取器
func TestSettersAndGetters(t *testing.T) {
	exec := NewLocalExecutor()

	exec.setWorkdir("/tmp")
	if exec.GetWorkdir() != "/tmp" {
		t.Errorf("Expected workdir '/tmp', got '%s'", exec.GetWorkdir())
	}

	exec.setShell("/bin/zsh")
	if exec.GetShell() != "/bin/zsh" {
		t.Errorf("Expected shell '/bin/zsh', got '%s'", exec.GetShell())
	}

	exec.setTimeout(5 * time.Second)
	if exec.timeout != 5*time.Second {
		t.Errorf("Expected timeout 5s, got %v", exec.timeout)
	}

	exec.setPTY(true)
	if !exec.usePTY {
		t.Error("Expected usePTY to be true")
	}

	exec.setPTYSize(120, 40)
	if exec.ptyWidth != 120 {
		t.Errorf("Expected ptyWidth 120, got %d", exec.ptyWidth)
	}
	if exec.ptyHeight != 40 {
		t.Errorf("Expected ptyHeight 40, got %d", exec.ptyHeight)
	}

	info := exec.GetRuntimeInfo()
	if info["workdir"] != "/tmp" {
		t.Errorf("Expected runtime info workdir '/tmp', got '%v'", info["workdir"])
	}
	if info["shell"] != "/bin/zsh" {
		t.Errorf("Expected runtime info shell '/bin/zsh', got '%v'", info["shell"])
	}
}

// TestGetType 测试获取类型
func TestGetType(t *testing.T) {
	exec := NewLocalExecutor()
	if exec.GetType() != "local" {
		t.Errorf("Expected type 'local', got '%s'", exec.GetType())
	}
}

// TestNewLocalExecutor_Defaults 测试默认配置
func TestNewLocalExecutor_Defaults(t *testing.T) {
	exec := NewLocalExecutor()

	if exec.workdir != "" {
		t.Errorf("Expected empty workdir, got '%s'", exec.workdir)
	}
	if exec.timeout != 0 {
		t.Errorf("Expected timeout 0, got %v", exec.timeout)
	}
	if exec.usePTY {
		t.Error("Expected usePTY to be false")
	}
	if exec.ptyWidth != 80 {
		t.Errorf("Expected ptyWidth 80, got %d", exec.ptyWidth)
	}
	if exec.ptyHeight != 24 {
		t.Errorf("Expected ptyHeight 24, got %d", exec.ptyHeight)
	}
	if exec.shell == "" {
		t.Error("Expected shell to be set")
	}
	if len(exec.env) != 0 {
		t.Errorf("Expected empty env, got %d items", len(exec.env))
	}
}

// TestDetectDefaultShell 测试默认 shell 检测
func TestDetectDefaultShell(t *testing.T) {
	shell := detectDefaultShell()
	if shell == "" {
		t.Error("Expected shell to be detected")
	}

	if _, err := exec.LookPath(shell); err != nil {
		if runtime.GOOS != "windows" {
			t.Errorf("Detected shell '%s' not found: %v", shell, err)
		}
	}
}

// TestExecuteCommandStreaming_ResultFormat 测试结果格式
func TestExecuteCommandStreaming_ResultFormat(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	resultChan := make(chan any, 10)
	inputChan := make(chan []byte)

	start := time.Now()
	exec.executeCommandStreaming(ctx, "echo test", "test_step", nil, resultChan, inputChan)
	close(resultChan)
	elapsed := time.Since(start)

	var foundResult bool
	for result := range resultChan {
		if sr, ok := result.(*executor.StepResult); ok {
			foundResult = true
			if sr.StepName != "test_step" {
				t.Errorf("Expected step name 'test_step', got '%s'", sr.StepName)
			}
			if sr.Command != "echo test" {
				t.Errorf("Expected command 'echo test', got '%s'", sr.Command)
			}
			if sr.Error != nil {
				t.Errorf("Expected no error, got: %v", sr.Error)
			}
			if sr.StartTime.IsZero() {
				t.Error("Expected StartTime to be set")
			}
			if sr.FinishTime.IsZero() {
				t.Error("Expected FinishTime to be set")
			}
			if sr.FinishTime.Before(sr.StartTime) {
				t.Error("Expected FinishTime to be after StartTime")
			}
		}
	}

	if !foundResult {
		t.Error("Expected to find a StepResult")
	}

	if elapsed > 5*time.Second {
		t.Errorf("Expected execution to be quick, took %v", elapsed)
	}
}

// TestStreamOutput_EmptyInput 测试空输入
func TestStreamOutput_EmptyInput(t *testing.T) {
	exec := NewLocalExecutor()

	reader := strings.NewReader("")
	var outputs []string

	callback := func(data []byte) {
		outputs = append(outputs, string(data))
	}

	exec.streamOutput(context.Background(), reader, callback, "test", nil)

	if len(outputs) != 0 {
		t.Errorf("Expected no output for empty input, got %d items", len(outputs))
	}
}

// TestStreamOutput_OnlyInputRequest 测试只有输入请求
func TestStreamOutput_OnlyInputRequest(t *testing.T) {
	exec := NewLocalExecutor()

	input := "```flowx-input\ntype: text\n```\n"
	reader := strings.NewReader(input)

	var requests []*executor.InputRequest
	callback := func(data []byte) {}

	onInputRequest := func(req *executor.InputRequest) {
		requests = append(requests, req)
	}

	exec.streamOutput(context.Background(), reader, callback, "test", onInputRequest)

	if len(requests) != 1 {
		t.Errorf("Expected 1 input request, got %d", len(requests))
	}
}

// TestStreamOutput_MultipleInputRequests 测试多个输入请求
func TestStreamOutput_MultipleInputRequests(t *testing.T) {
	exec := NewLocalExecutor()

	input := "```flowx-input\ntype: text\n```\n" +
		"normal output\n" +
		"```flowx-input\ntype: password\n```\n"

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

	if len(requests) != 2 {
		t.Errorf("Expected 2 input requests, got %d", len(requests))
	}

	if len(outputs) != 1 {
		t.Errorf("Expected 1 normal output, got %d", len(outputs))
	}
}

// TestTransfer_MultipleCommands 测试多个命令顺序执行
func TestTransfer_MultipleCommands(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	resultChan := make(chan any, 20)
	commandChan := make(chan any, 3)
	inputChan := make(chan []byte)

	commandChan <- executor.CommandWrapper{StepName: "step1", Command: "echo first"}
	commandChan <- executor.CommandWrapper{StepName: "step2", Command: "echo second"}
	commandChan <- executor.CommandWrapper{StepName: "step3", Command: "echo third"}
	close(commandChan)

	exec.Transfer(ctx, resultChan, commandChan, inputChan)

	var stepResults []*executor.StepResult
	for result := range resultChan {
		if sr, ok := result.(*executor.StepResult); ok {
			stepResults = append(stepResults, sr)
		}
	}

	if len(stepResults) != 3 {
		t.Errorf("Expected 3 step results, got %d", len(stepResults))
	}

	for i, sr := range stepResults {
		if sr.Error != nil {
			t.Errorf("Step %d failed: %v", i+1, sr.Error)
		}
	}
}

// TestExecuteCommandWithStreaming_ExitError 测试命令退出错误
func TestExecuteCommandWithStreaming_ExitError(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	callback := func(data []byte) {}

	err := exec.executeCommandWithStreaming(ctx, "exit 1", "test", nil, callback, nil, nil)
	if err == nil {
		t.Fatal("Expected error for exit code 1")
	}
	if !strings.Contains(err.Error(), "exited with code 1") {
		t.Errorf("Expected 'exited with code 1' error, got: %v", err)
	}
}

// TestParseInputRequest_JSONWithExtraFields 测试 JSON 解析额外字段
func TestParseInputRequest_JSONWithExtraFields(t *testing.T) {
	content := `{"type":"confirm","prompt":"Continue?","timeout":30,"extra":"ignored"}`
	req := parseInputRequest(content)
	if req == nil {
		t.Fatal("Expected non-nil request")
	}
	if req.Type != "confirm" {
		t.Errorf("Type = %q, want %q", req.Type, "confirm")
	}
	if req.Prompt != "Continue?" {
		t.Errorf("Prompt = %q, want %q", req.Prompt, "Continue?")
	}
	if req.Timeout != 30 {
		t.Errorf("Timeout = %d, want 30", req.Timeout)
	}
}

// TestParseInputRequest_YAMLWithExtraFields 测试 YAML 解析额外字段
func TestParseInputRequest_YAMLWithExtraFields(t *testing.T) {
	content := "type: password\nprompt: \"Enter password:\"\ntimeout: 60\nextra: ignored"
	req := parseInputRequest(content)
	if req == nil {
		t.Fatal("Expected non-nil request")
	}
	if req.Type != "password" {
		t.Errorf("Type = %q, want %q", req.Type, "password")
	}
	if req.Timeout != 60 {
		t.Errorf("Timeout = %d, want 60", req.Timeout)
	}
}

// TestConcurrentAccess 测试并发访问安全
func TestConcurrentAccess(t *testing.T) {
	exec := NewLocalExecutor()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			exec.setEnv(fmt.Sprintf("KEY_%d", index), fmt.Sprintf("value_%d", index))
			exec.GetWorkdir()
			exec.GetShell()
			exec.buildEnvList()
		}(i)
	}

	wg.Wait()

	envList := exec.buildEnvList()
	var count int
	for _, env := range envList {
		if strings.HasPrefix(env, "KEY_") {
			count++
		}
	}
	if count != 100 {
		t.Errorf("Expected 100 custom env vars, got %d", count)
	}
}

// BenchmarkBuildEnvList 测试环境变量构建性能
func BenchmarkBuildEnvList(b *testing.B) {
	exec := NewLocalExecutor()
	exec.setEnv("KEY1", "value1")
	exec.setEnv("KEY2", "value2")
	exec.setEnv("KEY3", "value3")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		exec.buildEnvList()
	}
}

// BenchmarkStreamOutput 测试流输出性能
func BenchmarkStreamOutput(b *testing.B) {
	exec := NewLocalExecutor()
	input := strings.Repeat("line\n", 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := strings.NewReader(input)
		exec.streamOutput(context.Background(), reader, func(data []byte) {}, "test", nil)
	}
}
