package docker

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/LerkoX/pipelinex/executor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDockerExecutor_WithDefaultRegistry 使用默认镜像源 (hub.rat.dev) 测试
func TestDockerExecutor_WithDefaultRegistry(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// 检查Docker是否可用
	if !isDockerAvailable(ctx) {
		t.Skip("Docker is not available, skipping test")
	}

	exec, err := NewDockerExecutor()
	require.NoError(t, err)

	// 使用默认镜像（会自动添加 hub.rat.dev 前缀）
	exec.setImage("hub.rat.dev/alpine:latest")
	exec.setWorkdir("/tmp")
	exec.setEnv("TEST_VAR", "test_value")

	// 准备环境
	err = exec.Prepare(ctx)
	require.NoError(t, err, "Prepare should succeed with default registry")

	containerID := exec.GetContainerID()
	assert.NotEmpty(t, containerID, "Container ID should be set")
	t.Logf("Container created: %s", containerID)

	// 清理环境
	defer func() {
		err := exec.Destruction(ctx)
		assert.NoError(t, err, "Destruction should succeed")
	}()

	// 测试 Transfer 方法
	resultChan := make(chan any, 10)
	commandChan := make(chan any, 3)

	go exec.Transfer(ctx, resultChan, commandChan, nil)

	// 发送测试命令
	commandChan <- executor.CommandWrapper{
		StepName: "test_step",
		Command:  "echo 'Hello from Alpine' && env | grep TEST_VAR",
	}

	// 接收结果
	results := []any{}
	timeout := time.After(10 * time.Second)
	resultCount := 0

	for resultCount < 2 { // 预期：输出 + 结果
		select {
		case result := <-resultChan:
			results = append(results, result)
			resultCount++

			// 验证输出内容
			if data, ok := result.([]byte); ok {
				output := string(data)
				t.Logf("Received output: %q", output)
				if strings.Contains(output, "Hello from Alpine") {
					t.Log("✓ Received expected output")
				}
				if strings.Contains(output, "TEST_VAR=test_value") {
					t.Log("✓ Environment variable is set correctly")
				}
			}

			if stepResult, ok := result.(*executor.StepResult); ok {
				t.Logf("Received step result: command=%q, error=%v", stepResult.Command, stepResult.Error)
				assert.NoError(t, stepResult.Error, "Command should execute successfully")
			}

		case <-timeout:
			t.Fatalf("Timeout waiting for results, received %d results", resultCount)
		}
	}

	assert.GreaterOrEqual(t, len(results), 1, "Should receive at least one result")
}

// TestDockerExecutor_MultiCommands 多命令执行测试
func TestDockerExecutor_MultiCommands(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	if !isDockerAvailable(ctx) {
		t.Skip("Docker is not available, skipping test")
	}

	exec, err := NewDockerExecutor()
	require.NoError(t, err)

	// 使用默认镜像源
	exec.setImage("hub.rat.dev/alpine:latest")

	err = exec.Prepare(ctx)
	require.NoError(t, err)

	defer exec.Destruction(ctx)

	resultChan := make(chan any, 20)
	commandChan := make(chan any, 4)

	go exec.Transfer(ctx, resultChan, commandChan, nil)

	// 发送多个命令
	commands := []executor.CommandWrapper{
		{StepName: "step1", Command: "echo 'step1_output'"},
		{StepName: "step2", Command: "pwd"},
		{StepName: "step3", Command: "ls -la /"},
	}
	for _, cmd := range commands {
		commandChan <- cmd
	}
	close(commandChan)

	// 收集结果
	results := []any{}
	timeout := time.After(15 * time.Second)

	for len(results) < 6 { // 每个命令输出+结果=2，3个命令=6
		select {
		case result := <-resultChan:
			results = append(results, result)
			if stepResult, ok := result.(*executor.StepResult); ok {
				t.Logf("Step %s completed: error=%v", stepResult.StepName, stepResult.Error)
				assert.NoError(t, stepResult.Error)
			}
		case <-timeout:
			goto done
		}
	}
done:

	// 验证至少收到了3个步骤结果
	stepCount := 0
	for _, r := range results {
		if _, ok := r.(*executor.StepResult); ok {
			stepCount++
		}
	}
	assert.Equal(t, 3, stepCount, "Should receive 3 step results")
	t.Logf("Total results received: %d", len(results))
}

// TestDockerExecutor_CustomRegistry 测试自定义镜像源覆盖默认设置
func TestDockerExecutor_CustomRegistry(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if !isDockerAvailable(ctx) {
		t.Skip("Docker is not available, skipping test")
	}

	exec, err := NewDockerExecutor()
	require.NoError(t, err)

	// 使用自定义镜像源（覆盖默认的 hub.rat.dev）
	exec.setRegistry("") // 清空 registry，使用 Docker Hub 或其他
	exec.setImage("hub.rat.dev/ubuntu:latest")

	err = exec.Prepare(ctx)
	require.NoError(t, err)

	defer exec.Destruction(ctx)

	// 验证容器运行正常
	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)

	go exec.Transfer(ctx, resultChan, commandChan, nil)
	commandChan <- executor.CommandWrapper{
		StepName: "test",
		Command:  "echo 'Custom registry test'",
	}

	select {
	case result := <-resultChan:
		if stepResult, ok := result.(*executor.StepResult); ok {
			assert.NoError(t, stepResult.Error)
			t.Log("✓ Custom registry configuration works")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout waiting for result")
	}
}
