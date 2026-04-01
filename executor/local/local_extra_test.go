package local

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/LerkoX/pipelinex/executor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// collectStepResult 辅助函数：从 resultChan 收集 StepResult
func collectStepResult(resultChan <-chan any, timeout time.Duration) *executor.StepResult {
	timeoutCh := time.After(timeout)
	for {
		select {
		case res := <-resultChan:
			switch v := res.(type) {
			case *executor.StepResult:
				return v
			case []byte:
				// 忽略输出字节，继续等待 StepResult
				continue
			case error:
				// 返回包含错误的 StepResult
				return &executor.StepResult{Error: v}
			}
		case <-timeoutCh:
			return nil
		}
	}
}

// ==================== 1. 多命令执行测试 ====================

// TestLocalExecutor_MultipleCommandsSequential 测试顺序执行多个命令
func TestLocalExecutor_MultipleCommandsSequential(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	exec := NewLocalExecutor()
	require.NoError(t, exec.Prepare(ctx))
	defer exec.Destruction(ctx)

	commands := []struct {
		stepName string
		command  string
	}{
		{"step1", "echo 'first'"},
		{"step2", "echo 'second'"},
		{"step3", "echo 'third'"},
	}

	for _, cmd := range commands {
		resultChan := make(chan any, 10)
		commandChan := make(chan any, 1)

		go exec.Transfer(ctx, resultChan, commandChan)

		commandChan <- executor.CommandWrapper{
			StepName: cmd.stepName,
			Command:  cmd.command,
		}
		close(commandChan)

		result := collectStepResult(resultChan, 5*time.Second)
		require.NotNil(t, result, "Should receive StepResult for %s", cmd.stepName)
		assert.NoError(t, result.Error, "Command %s should succeed", cmd.stepName)
		assert.Equal(t, cmd.stepName, result.StepName, "Step name should match")
	}
}

// TestLocalExecutor_ConcurrentCommands 测试并发命令执行
func TestLocalExecutor_ConcurrentCommands(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	exec := NewLocalExecutor()
	require.NoError(t, exec.Prepare(ctx))
	defer exec.Destruction(ctx)

	resultChan := make(chan any, 30)
	commandChan := make(chan any, 5)

	go exec.Transfer(ctx, resultChan, commandChan)

	for i := 1; i <= 5; i++ {
		commandChan <- executor.CommandWrapper{
			StepName: fmt.Sprintf("step-%d", i),
			Command:  fmt.Sprintf("echo 'output-%d'", i),
		}
	}
	close(commandChan)

	results := make(map[string]bool)
	timeout := time.After(10 * time.Second)

	for len(results) < 5 {
		select {
		case res := <-resultChan:
			switch v := res.(type) {
			case []byte:
				output := string(v)
				for i := 1; i <= 5; i++ {
					if strings.Contains(output, fmt.Sprintf("output-%d", i)) {
						results[fmt.Sprintf("output-%d", i)] = true
					}
				}
			case *executor.StepResult:
				if v.Error == nil {
					results[v.StepName] = true
				}
			}
		case <-timeout:
			t.Fatalf("Timeout waiting for results, got %d results", len(results))
		}
	}

	assert.Len(t, results, 5, "Should receive all 5 step results")
}

// ==================== 2. 大输出测试 ====================

// TestLocalExecutor_LargeOutput 测试大输出处理
func TestLocalExecutor_LargeOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	exec := NewLocalExecutor()
	require.NoError(t, exec.Prepare(ctx))
	defer exec.Destruction(ctx)

	resultChan := make(chan any, 100)
	commandChan := make(chan any, 1)

	go exec.Transfer(ctx, resultChan, commandChan)

	commandChan <- executor.CommandWrapper{
		StepName: "large-output",
		Command:  "seq 1 1000",
	}
	close(commandChan)

	result := collectStepResult(resultChan, 15*time.Second)
	require.NotNil(t, result, "Should receive StepResult")
	assert.NoError(t, result.Error, "Large output command should succeed")
}

// ==================== 3. 管道和特殊字符测试 ====================

// TestLocalExecutor_CommandWithPipe 测试管道命令
func TestLocalExecutor_CommandWithPipe(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exec := NewLocalExecutor()
	require.NoError(t, exec.Prepare(ctx))
	defer exec.Destruction(ctx)

	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)

	go exec.Transfer(ctx, resultChan, commandChan)

	commandChan <- executor.CommandWrapper{
		StepName: "pipe-command",
		Command:  "echo 'hello world' | grep 'hello'",
	}
	close(commandChan)

	result := collectStepResult(resultChan, 5*time.Second)
	require.NotNil(t, result, "Should receive StepResult")
	assert.NoError(t, result.Error, "Pipe command should succeed")
}

// TestLocalExecutor_SpecialCharacters 测试特殊字符处理
func TestLocalExecutor_SpecialCharacters(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	exec := NewLocalExecutor()
	require.NoError(t, exec.Prepare(ctx))
	defer exec.Destruction(ctx)

	tests := []struct {
		name    string
		command string
	}{
		{name: "quotes", command: `echo "hello world"`},
		{name: "semicolon", command: `echo a; echo b`},
		{name: "pipe", command: `echo hello | wc -c`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultChan := make(chan any, 10)
			commandChan := make(chan any, 1)

			go exec.Transfer(ctx, resultChan, commandChan)

			commandChan <- executor.CommandWrapper{
				StepName: tt.name,
				Command:  tt.command,
			}
			close(commandChan)

			result := collectStepResult(resultChan, 5*time.Second)
			require.NotNil(t, result, "Should receive StepResult")
			assert.NoError(t, result.Error, "Command with special characters should succeed: %s", tt.name)
		})
	}
}

// ==================== 4. Bridge 和 Adapter 错误处理测试 ====================

// TestLocalExecutor_InvalidAdapterExtra 测试无效适配器类型
func TestLocalExecutor_InvalidAdapterExtra(t *testing.T) {
	ctx := context.Background()
	bridge := NewLocalBridge()

	dummyAdapter := &dummyLocalAdapter{}

	_, err := bridge.Conn(ctx, dummyAdapter)
	assert.Error(t, err, "Should return error for invalid adapter type")
	assert.Contains(t, err.Error(), "not a LocalAdapter", "Error message should indicate wrong adapter type")
}

// dummyLocalAdapter 用于测试的虚拟适配器
type dummyLocalAdapter struct{}

func (d *dummyLocalAdapter) Config(ctx context.Context, config map[string]any) error {
	return nil
}

// TestLocalExecutor_BridgeConfigError 测试桥接器配置错误处理
func TestLocalExecutor_BridgeConfigError(t *testing.T) {
	ctx := context.Background()
	bridge := NewLocalBridge()
	adapter := NewLocalAdapter()

	err := adapter.Config(ctx, map[string]any{
		"timeout": "invalid-timeout-format",
	})
	require.NoError(t, err, "Config should not return error for adapter")

	_, err = bridge.Conn(ctx, adapter)
	if err != nil {
		t.Logf("Conn returned error for invalid timeout: %v", err)
	}
}

// ==================== 5. Shell 和超时配置测试 ====================

// TestLocalExecutor_ShellConfiguration 测试 Shell 配置
func TestLocalExecutor_ShellConfiguration(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exec := NewLocalExecutor()
	exec.setShell("/bin/bash")
	require.NoError(t, exec.Prepare(ctx))
	defer exec.Destruction(ctx)

	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)

	go exec.Transfer(ctx, resultChan, commandChan)

	commandChan <- executor.CommandWrapper{
		StepName: "bash-test",
		Command:  "echo $BASH_VERSION",
	}
	close(commandChan)

	result := collectStepResult(resultChan, 5*time.Second)
	require.NotNil(t, result, "Should receive StepResult")
	assert.NoError(t, result.Error, "Bash command should succeed")
}

// TestLocalExecutor_Timeout 测试超时功能
func TestLocalExecutor_Timeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	ctx := context.Background()

	exec := NewLocalExecutor()
	exec.setTimeout(1 * time.Second)
	require.NoError(t, exec.Prepare(ctx))
	defer exec.Destruction(ctx)

	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)

	go exec.Transfer(ctx, resultChan, commandChan)

	commandChan <- executor.CommandWrapper{
		StepName: "timeout-test",
		Command:  "sleep 5",
	}
	close(commandChan)

	result := collectStepResult(resultChan, 10*time.Second)
	require.NotNil(t, result, "Should receive StepResult")
	assert.Error(t, result.Error, "Command should timeout")
	assert.Contains(t, result.Error.Error(), "timed out", "Error should indicate timeout")
}

// ==================== 6. Getter 和类型测试 ====================

// TestLocalExecutor_GetType 测试执行器类型
func TestLocalExecutor_GetType(t *testing.T) {
	exec := NewLocalExecutor()
	assert.Equal(t, "local", exec.GetType(), "GetType should return 'local'")
}

// TestLocalExecutor_GetRuntimeInfo 测试运行时信息
func TestLocalExecutor_GetRuntimeInfo(t *testing.T) {
	exec := NewLocalExecutor()
	exec.setWorkdir("/tmp")
	exec.setShell("/bin/bash")

	info := exec.GetRuntimeInfo()

	assert.Equal(t, "/tmp", info["workdir"], "Runtime info should contain correct workdir")
	assert.Equal(t, "/bin/bash", info["shell"], "Runtime info should contain correct shell")
}

// ==================== 7. 工作目录测试 ====================

// TestLocalExecutor_WorkdirValidation 测试工作目录验证
func TestLocalExecutor_WorkdirValidation(t *testing.T) {
	ctx := context.Background()

	t.Run("valid workdir", func(t *testing.T) {
		exec := NewLocalExecutor()
		exec.setWorkdir("/tmp")
		err := exec.Prepare(ctx)
		assert.NoError(t, err, "Prepare should succeed with valid workdir")
		exec.Destruction(ctx)
	})

	t.Run("non-existent workdir", func(t *testing.T) {
		exec := NewLocalExecutor()
		exec.setWorkdir("/this/path/does/not/exist/12345")
		err := exec.Prepare(ctx)
		assert.Error(t, err, "Prepare should fail with non-existent workdir")
		assert.Contains(t, err.Error(), "does not exist", "Error should indicate workdir does not exist")
	})

	t.Run("file as workdir", func(t *testing.T) {
		exec := NewLocalExecutor()
		exec.setWorkdir("/etc/passwd")
		err := exec.Prepare(ctx)
		assert.Error(t, err, "Prepare should fail when workdir is a file")
	})
}

// TestLocalExecutor_WorkdirInCommand 测试命令在工作目录中执行
func TestLocalExecutor_WorkdirInCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exec := NewLocalExecutor()
	exec.setWorkdir("/tmp")
	require.NoError(t, exec.Prepare(ctx))
	defer exec.Destruction(ctx)

	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)

	go exec.Transfer(ctx, resultChan, commandChan)

	commandChan <- executor.CommandWrapper{
		StepName: "pwd-check",
		Command:  `[ "$(pwd)" = "/tmp" ]`,
	}
	close(commandChan)

	result := collectStepResult(resultChan, 5*time.Second)
	require.NotNil(t, result, "Should receive StepResult")
	assert.NoError(t, result.Error, "Command should be executed in /tmp")
}

// ==================== 8. 环境变量覆盖测试 ====================

// TestLocalExecutor_EnvOverride 测试环境变量覆盖
func TestLocalExecutor_EnvOverride(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exec := NewLocalExecutor()
	exec.setEnv("TEST_VAR", "test_value")
	require.NoError(t, exec.Prepare(ctx))
	defer exec.Destruction(ctx)

	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)

	go exec.Transfer(ctx, resultChan, commandChan)

	commandChan <- executor.CommandWrapper{
		StepName: "env-check",
		Command:  `[ "$TEST_VAR" = "test_value" ]`,
	}
	close(commandChan)

	result := collectStepResult(resultChan, 5*time.Second)
	require.NotNil(t, result, "Should receive StepResult")
	assert.NoError(t, result.Error, "Custom environment variable should be set")
}

// ==================== 9. 空命令和边界情况测试 ====================

// TestLocalExecutor_EmptyCommand 测试空命令
func TestLocalExecutor_EmptyCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exec := NewLocalExecutor()
	require.NoError(t, exec.Prepare(ctx))
	defer exec.Destruction(ctx)

	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)

	go exec.Transfer(ctx, resultChan, commandChan)

	commandChan <- executor.CommandWrapper{
		StepName: "empty-command",
		Command:  "true",
	}
	close(commandChan)

	result := collectStepResult(resultChan, 5*time.Second)
	require.NotNil(t, result, "Should receive StepResult")
	assert.NoError(t, result.Error, "Empty/true command should succeed")
}

// TestLocalExecutor_CommandNotFound 测试命令不存在
func TestLocalExecutor_CommandNotFound(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exec := NewLocalExecutor()
	require.NoError(t, exec.Prepare(ctx))
	defer exec.Destruction(ctx)

	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)

	go exec.Transfer(ctx, resultChan, commandChan)

	commandChan <- executor.CommandWrapper{
		StepName: "not-found",
		Command:  "this_command_does_not_exist_12345",
	}
	close(commandChan)

	result := collectStepResult(resultChan, 5*time.Second)
	require.NotNil(t, result, "Should receive StepResult")
	assert.Error(t, result.Error, "Non-existent command should return error")
}

// ==================== 10. PTY 配置测试 ====================

// TestLocalExecutor_PTYConfiguration 测试 PTY 配置
func TestLocalExecutor_PTYConfiguration(t *testing.T) {
	exec := NewLocalExecutor()

	assert.False(t, exec.usePTY, "Default usePTY should be false")

	exec.setPTY(true)
	assert.True(t, exec.usePTY, "usePTY should be true after set")

	exec.setPTYSize(120, 40)
	assert.Equal(t, 120, exec.ptyWidth, "PTY width should be 120")
	assert.Equal(t, 40, exec.ptyHeight, "PTY height should be 40")
}

// TestLocalExecutor_DefaultShellDetectionExtra 测试默认 Shell 检测
func TestLocalExecutor_DefaultShellDetectionExtra(t *testing.T) {
	shell := detectDefaultShell()
	assert.NotEmpty(t, shell, "Should detect a default shell")

	switch runtime.GOOS {
	case "windows":
		assert.True(t, shell == "pwsh" || shell == "powershell" || shell == "cmd",
			"Windows should have powershell or cmd, got: %s", shell)
	default:
		assert.True(t, shell == "/bin/bash" || shell == "/bin/sh" || shell == "bash" || shell == "sh",
			"Unix should have bash or sh, got: %s", shell)
	}
}
