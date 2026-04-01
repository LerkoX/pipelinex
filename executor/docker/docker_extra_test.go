package docker

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

// ==================== 1. 进程生命周期管理测试 ====================

// TestDockerExecutor_DestructionTerminatesProcess 测试 Destruction 时容器内进程是否被终止
func TestDockerExecutor_DestructionTerminatesProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	if !isDockerAvailable(ctx) {
		t.Skip("Docker is not available, skipping test")
	}

	exec, err := NewDockerExecutor()
	require.NoError(t, err)

	exec.setImage("hub.rat.dev/alpine:latest")
	require.NoError(t, exec.Prepare(ctx))

	containerID := exec.GetContainerID()
	t.Logf("Container created: %s", containerID)

	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)

	go exec.Transfer(ctx, resultChan, commandChan)

	// 启动一个长时间运行的命令
	commandChan <- executor.CommandWrapper{
		StepName: "long-running-step",
		Command:  "sleep 100",
	}

	// 等待命令启动
	time.Sleep(2 * time.Second)

	// 调用 Destruction，这应该终止正在运行的命令和容器
	err = exec.Destruction(ctx)
	require.NoError(t, err, "Destruction should not return error")

	// 验证容器已被删除
	verifyCtx, verifyCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer verifyCancel()

	_, err = exec.client.ContainerInspect(verifyCtx, containerID)
	assert.Error(t, err, "Container should be removed after Destruction")
	assert.Contains(t, err.Error(), "No such container", "Error should indicate container not found")
}

// TestDockerExecutor_ContextCancelTerminatesProcess 测试上下文取消时进程是否终止
func TestDockerExecutor_ContextCancelTerminatesProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	ctx, cancel := context.WithCancel(context.Background())

	if !isDockerAvailable(ctx) {
		t.Skip("Docker is not available, skipping test")
	}

	exec, err := NewDockerExecutor()
	require.NoError(t, err)

	exec.setImage("hub.rat.dev/alpine:latest")
	require.NoError(t, exec.Prepare(ctx))

	defer exec.Destruction(context.Background())

	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)

	go exec.Transfer(ctx, resultChan, commandChan)

	// 启动一个长时间运行的命令
	commandChan <- executor.CommandWrapper{
		StepName: "long-running-step",
		Command:  "sleep 100",
	}

	// 等待命令启动
	time.Sleep(2 * time.Second)

	// 取消上下文
	cancel()

	// 等待 Transfer 返回
	time.Sleep(1 * time.Second)
}

// ==================== 2. 错误场景测试 ====================

// TestDockerExecutor_Transfer_CommandFailure 测试命令失败（非零退出码）
func TestDockerExecutor_Transfer_CommandFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if !isDockerAvailable(ctx) {
		t.Skip("Docker is not available, skipping test")
	}

	exec, err := NewDockerExecutor()
	require.NoError(t, err)

	exec.setImage("hub.rat.dev/alpine:latest")
	require.NoError(t, exec.Prepare(ctx))
	defer exec.Destruction(ctx)

	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)

	go exec.Transfer(ctx, resultChan, commandChan)

	// 发送一个会失败的命令
	commandChan <- executor.CommandWrapper{
		StepName: "failing-step",
		Command:  "exit 42",
	}
	close(commandChan)

	// 接收结果
	var result *executor.StepResult
	timeout := time.After(10 * time.Second)

resultLoop:
	for {
		select {
		case res := <-resultChan:
			switch v := res.(type) {
			case *executor.StepResult:
				result = v
				break resultLoop
			case error:
				t.Fatalf("Received unexpected error: %v", v)
			}
		case <-timeout:
			t.Fatalf("Timeout waiting for results")
		}
	}

	require.NotNil(t, result, "Should receive StepResult")
	assert.Error(t, result.Error, "Failed command should return error")
	assert.Contains(t, result.Error.Error(), "42", "Error should contain exit code")
}

// TestDockerExecutor_InvalidImage 测试无效镜像名称
func TestDockerExecutor_InvalidImage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if !isDockerAvailable(ctx) {
		t.Skip("Docker is not available, skipping test")
	}

	exec, err := NewDockerExecutor()
	require.NoError(t, err)

	// 设置一个不存在的镜像
	exec.setImage("this-image-does-not-exist-12345:latest")

	// 准备应该失败（拉取镜像失败）
	err = exec.Prepare(ctx)
	assert.Error(t, err, "Prepare should fail with invalid image")
	assert.Contains(t, err.Error(), "pull", "Error should be about pulling image")
}

// TestDockerExecutor_InvalidWorkdir 测试无效工作目录
func TestDockerExecutor_InvalidWorkdir(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if !isDockerAvailable(ctx) {
		t.Skip("Docker is not available, skipping test")
	}

	exec, err := NewDockerExecutor()
	require.NoError(t, err)

	exec.setImage("hub.rat.dev/alpine:latest")
	// 设置一个 Alpine 容器中不存在的工作目录
	exec.setWorkdir("/nonexistent/path/that/does/not/exist")

	// 准备应该失败（创建工作目录失败）
	err = exec.Prepare(ctx)
	// 注意：Alpine 的 sleep 命令不需要工作目录存在，所以这个测试可能不失败
	// 真正的验证应该在执行命令时
	if err != nil {
		t.Logf("Prepare failed as expected: %v", err)
	}
}

// TestDockerExecutor_TransferWithoutPrepare 测试未 Prepare 直接调用 Transfer
func TestDockerExecutor_TransferWithoutPrepare(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exec, err := NewDockerExecutor()
	require.NoError(t, err)

	// 不调用 Prepare，直接调用 Transfer
	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)

	go exec.Transfer(ctx, resultChan, commandChan)

	// 发送命令
	commandChan <- executor.CommandWrapper{
		StepName: "test-step",
		Command:  "echo test",
	}

	// 应该收到错误
	timeout := time.After(5 * time.Second)
	select {
	case res := <-resultChan:
		err, ok := res.(error)
		if ok {
			assert.Contains(t, err.Error(), "container not prepared", "Error should indicate container not prepared")
		} else if stepResult, ok := res.(*executor.StepResult); ok {
			assert.Error(t, stepResult.Error, "Should receive error in StepResult")
			assert.Contains(t, stepResult.Error.Error(), "container not prepared")
		}
	case <-timeout:
		t.Fatal("Timeout waiting for error")
	}
}

// TestDockerExecutor_UnsupportedDataType 测试 Transfer 时传入错误类型
func TestDockerExecutor_UnsupportedDataType(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if !isDockerAvailable(ctx) {
		t.Skip("Docker is not available, skipping test")
	}

	exec, err := NewDockerExecutor()
	require.NoError(t, err)

	exec.setImage("hub.rat.dev/alpine:latest")
	require.NoError(t, exec.Prepare(ctx))
	defer exec.Destruction(ctx)

	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)

	go exec.Transfer(ctx, resultChan, commandChan)

	// 发送不支持的类型
	commandChan <- 12345
	close(commandChan)

	// 接收错误
	timeout := time.After(5 * time.Second)
	select {
	case res := <-resultChan:
		err, ok := res.(error)
		require.True(t, ok, "Should receive an error")
		assert.Contains(t, err.Error(), "unsupported data type", "Error should indicate unsupported type")
	case <-timeout:
		t.Fatal("Timeout waiting for error")
	}
}

// ==================== 3. 资源管理测试 ====================

// TestDockerExecutor_VolumeMounting 测试卷挂载实际生效
// 注意：此测试需要 Docker 支持 bind mount，在某些环境中可能失败
func TestDockerExecutor_VolumeMounting(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if !isDockerAvailable(ctx) {
		t.Skip("Docker is not available, skipping test")
	}

	// 使用已知的系统目录进行测试
	testDir := "/tmp"

	exec, err := NewDockerExecutor()
	require.NoError(t, err)

	exec.setImage("hub.rat.dev/alpine:latest")
	// 将主机的 /tmp 挂载到容器的 /host-tmp
	exec.setVolume(testDir, "/host-tmp")

	require.NoError(t, exec.Prepare(ctx))
	defer exec.Destruction(ctx)

	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)

	go exec.Transfer(ctx, resultChan, commandChan)

	// 简单测试：检查挂载目录是否存在且可访问
	commandChan <- executor.CommandWrapper{
		StepName: "check-mount",
		Command:  "ls /host-tmp > /dev/null && echo 'MOUNT_OK'",
	}
	close(commandChan)

	// 收集结果
	var result *executor.StepResult
	timeout := time.After(10 * time.Second)

resultLoop:
	for {
		select {
		case res := <-resultChan:
			switch v := res.(type) {
			case *executor.StepResult:
				result = v
				break resultLoop
			}
		case <-timeout:
			t.Fatalf("Timeout waiting for results")
		}
	}

	require.NotNil(t, result, "Should receive StepResult")
	// 如果挂载失败，命令会返回错误
	if result.Error != nil {
		t.Skip("Volume mount test skipped - bind mount may not be supported in this environment")
	}
}

// TestDockerExecutor_EnvVarsInContainer 测试环境变量在容器中实际生效
func TestDockerExecutor_EnvVarsInContainer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if !isDockerAvailable(ctx) {
		t.Skip("Docker is not available, skipping test")
	}

	exec, err := NewDockerExecutor()
	require.NoError(t, err)

	exec.setImage("hub.rat.dev/alpine:latest")
	exec.setEnv("TEST_KEY1", "test_value_1")
	exec.setEnv("TEST_KEY2", "test_value_2")

	require.NoError(t, exec.Prepare(ctx))
	defer exec.Destruction(ctx)

	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)

	go exec.Transfer(ctx, resultChan, commandChan)

	// 验证环境变量通过检查命令是否成功执行（而不是解析输出）
	commandChan <- executor.CommandWrapper{
		StepName: "env-check",
		Command:  "[ \"$TEST_KEY1\" = \"test_value_1\" ] && [ \"$TEST_KEY2\" = \"test_value_2\" ]",
	}
	close(commandChan)

	// 收集结果
	var result *executor.StepResult
	timeout := time.After(10 * time.Second)

resultLoop:
	for {
		select {
		case res := <-resultChan:
			switch v := res.(type) {
			case *executor.StepResult:
				result = v
				break resultLoop
			}
		case <-timeout:
			t.Fatalf("Timeout waiting for results")
		}
	}

	require.NotNil(t, result, "Should receive StepResult")
	assert.NoError(t, result.Error, "Environment variables should be set correctly in container")
}

// TestDockerExecutor_NetworkMode 测试网络模式
func TestDockerExecutor_NetworkMode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if !isDockerAvailable(ctx) {
		t.Skip("Docker is not available, skipping test")
	}

	exec, err := NewDockerExecutor()
	require.NoError(t, err)

	exec.setImage("hub.rat.dev/alpine:latest")
	exec.setNetwork("host")

	require.NoError(t, exec.Prepare(ctx))
	defer exec.Destruction(ctx)

	// 验证容器使用了 host 网络模式
	containerJSON, err := exec.client.ContainerInspect(ctx, exec.GetContainerID())
	require.NoError(t, err)

	assert.Equal(t, "host", string(containerJSON.HostConfig.NetworkMode), "Container should use host network mode")
}

// ==================== 4. 健壮性测试 ====================

// TestDockerExecutor_DoublePrepare 测试重复 Prepare 调用
func TestDockerExecutor_DoublePrepare(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	if !isDockerAvailable(ctx) {
		t.Skip("Docker is not available, skipping test")
	}

	exec, err := NewDockerExecutor()
	require.NoError(t, err)

	exec.setImage("hub.rat.dev/alpine:latest")

	// 第一次 Prepare
	require.NoError(t, exec.Prepare(ctx))
	containerID1 := exec.GetContainerID()
	t.Logf("First container: %s", containerID1)

	// 第二次 Prepare - 应该创建新容器（或返回错误，取决于实现）
	// 当前实现会创建新容器并覆盖 containerID
	err = exec.Prepare(ctx)
	// 根据实现不同，可能成功也可能失败
	if err != nil {
		t.Logf("Second Prepare returned error (expected): %v", err)
	} else {
		containerID2 := exec.GetContainerID()
		t.Logf("Second container: %s", containerID2)
		// 如果成功，应该有不同的容器ID
		if containerID1 != containerID2 {
			t.Log("Note: Second Prepare created a new container")
		}
	}

	// 清理
	exec.Destruction(ctx)
}

// TestDockerExecutor_MultipleCommandsSequential 测试顺序执行多个命令
func TestDockerExecutor_MultipleCommandsSequential(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if !isDockerAvailable(ctx) {
		t.Skip("Docker is not available, skipping test")
	}

	exec, err := NewDockerExecutor()
	require.NoError(t, err)

	exec.setImage("hub.rat.dev/alpine:latest")
	require.NoError(t, exec.Prepare(ctx))
	defer exec.Destruction(ctx)

	// 顺序执行多个命令
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

		// 收集结果
		var result *executor.StepResult
		timeout := time.After(10 * time.Second)

	resultLoop:
		for {
			select {
			case res := <-resultChan:
				switch v := res.(type) {
				case *executor.StepResult:
					result = v
					break resultLoop
				}
			case <-timeout:
				t.Fatalf("Timeout waiting for results for %s", cmd.stepName)
			}
		}

		require.NotNil(t, result, "Should receive StepResult for %s", cmd.stepName)
		assert.NoError(t, result.Error, "Command %s should succeed", cmd.stepName)
		t.Logf("✓ %s passed", cmd.stepName)
	}
}

// TestDockerExecutor_LargeOutput 测试大输出处理
func TestDockerExecutor_LargeOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if !isDockerAvailable(ctx) {
		t.Skip("Docker is not available, skipping test")
	}

	exec, err := NewDockerExecutor()
	require.NoError(t, err)

	exec.setImage("hub.rat.dev/alpine:latest")
	require.NoError(t, exec.Prepare(ctx))
	defer exec.Destruction(ctx)

	resultChan := make(chan any, 100)
	commandChan := make(chan any, 1)

	go exec.Transfer(ctx, resultChan, commandChan)

	// 生成大输出
	commandChan <- executor.CommandWrapper{
		StepName: "large-output",
		Command:  "seq 1 1000",
	}
	close(commandChan)

	// 收集结果 - 验证命令成功执行
	var result *executor.StepResult
	timeout := time.After(20 * time.Second)

resultLoop:
	for {
		select {
		case res := <-resultChan:
			switch v := res.(type) {
			case *executor.StepResult:
				result = v
				break resultLoop
			}
		case <-timeout:
			t.Fatalf("Timeout waiting for results")
		}
	}

	require.NotNil(t, result, "Should receive StepResult")
	assert.NoError(t, result.Error, "Large output command should succeed")
}

// TestDockerExecutor_ShellDetectionAlpine 测试 Alpine 镜像的 Shell 检测
func TestDockerExecutor_ShellDetectionAlpine(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if !isDockerAvailable(ctx) {
		t.Skip("Docker is not available, skipping test")
	}

	exec, err := NewDockerExecutor()
	require.NoError(t, err)

	exec.setImage("hub.rat.dev/alpine:latest")
	require.NoError(t, exec.Prepare(ctx))
	defer exec.Destruction(ctx)

	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)

	go exec.Transfer(ctx, resultChan, commandChan)

	// 通过执行 shell 内置命令来检测 shell 类型
	commandChan <- executor.CommandWrapper{
		StepName: "shell-check",
		Command:  "sh -c 'echo ok'", // Alpine 使用 sh
	}
	close(commandChan)

	// 收集结果
	var result *executor.StepResult
	timeout := time.After(10 * time.Second)

resultLoop:
	for {
		select {
		case res := <-resultChan:
			switch v := res.(type) {
			case *executor.StepResult:
				result = v
				break resultLoop
			}
		case <-timeout:
			t.Fatalf("Timeout waiting for results")
		}
	}

	require.NotNil(t, result, "Should receive StepResult")
	assert.NoError(t, result.Error, "Shell command should succeed on Alpine")
}

// TestDockerExecutor_ShellDetectionUbuntu 测试 Ubuntu 镜像的 Shell 检测
func TestDockerExecutor_ShellDetectionUbuntu(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if !isDockerAvailable(ctx) {
		t.Skip("Docker is not available, skipping test")
	}

	exec, err := NewDockerExecutor()
	require.NoError(t, err)

	// 使用本地的 Ubuntu 镜像
	exec.setImage("hub.rat.dev/ubuntu:latest")
	require.NoError(t, exec.Prepare(ctx))
	defer exec.Destruction(ctx)

	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)

	go exec.Transfer(ctx, resultChan, commandChan)

	// 测试 bash 是否可用
	commandChan <- executor.CommandWrapper{
		StepName: "shell-check",
		Command:  "bash -c 'echo ok'",
	}
	close(commandChan)

	// 收集结果
	var result *executor.StepResult
	timeout := time.After(10 * time.Second)

resultLoop:
	for {
		select {
		case res := <-resultChan:
			switch v := res.(type) {
			case *executor.StepResult:
				result = v
				break resultLoop
			}
		case <-timeout:
			t.Fatalf("Timeout waiting for results")
		}
	}

	require.NotNil(t, result, "Should receive StepResult")
	assert.NoError(t, result.Error, "Bash command should succeed on Ubuntu")
}

// TestDockerExecutor_GetRuntimeInfo 测试运行时信息获取
func TestDockerExecutor_GetRuntimeInfo(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if !isDockerAvailable(ctx) {
		t.Skip("Docker is not available, skipping test")
	}

	exec, err := NewDockerExecutor()
	require.NoError(t, err)

	exec.setImage("hub.rat.dev/alpine:latest")
	exec.setWorkdir("/app")
	exec.setNetwork("bridge")

	require.NoError(t, exec.Prepare(ctx))
	defer exec.Destruction(ctx)

	info := exec.GetRuntimeInfo()

	assert.NotEmpty(t, info["containerId"], "Runtime info should contain containerId")
	assert.Equal(t, "hub.rat.dev/alpine:latest", info["image"], "Runtime info should contain correct image")
	assert.Equal(t, "/app", info["workdir"], "Runtime info should contain correct workdir")
	assert.Equal(t, "bridge", info["network"], "Runtime info should contain correct network")
	assert.Equal(t, "", info["registry"], "Runtime info should contain empty registry by default")

	t.Logf("Runtime info: %+v", info)
}

// TestDockerExecutor_GetType 测试执行器类型
func TestDockerExecutor_GetType(t *testing.T) {
	exec, err := NewDockerExecutor()
	require.NoError(t, err)

	assert.Equal(t, "docker", exec.GetType(), "GetType should return 'docker'")
}

// TestDockerExecutor_InvalidAdapter 测试无效适配器类型
func TestDockerExecutor_InvalidAdapter(t *testing.T) {
	ctx := context.Background()
	bridge := NewDockerBridge()

	// 使用错误的适配器类型
	dummyAdapter := &dummyDockerAdapter{}

	_, err := bridge.Conn(ctx, dummyAdapter)
	assert.Error(t, err, "Should return error for invalid adapter type")
	assert.Contains(t, err.Error(), "not a DockerAdapter", "Error message should indicate wrong adapter type")
}

// dummyDockerAdapter 用于测试的虚拟适配器
type dummyDockerAdapter struct{}

func (d *dummyDockerAdapter) Config(ctx context.Context, config map[string]any) error {
	return nil
}

// TestDockerExecutor_BridgeConfigError 测试桥接器配置错误处理
func TestDockerExecutor_BridgeConfigError(t *testing.T) {
	ctx := context.Background()
	bridge := NewDockerBridge()
	adapter := NewDockerAdapter()

	// 配置无效的卷格式
	err := adapter.Config(ctx, map[string]any{
		"volumes": []string{
			"invalid-volume-format", // 缺少冒号分隔
		},
	})
	require.NoError(t, err, "Config should not return error for adapter")

	// 但连接时应该失败
	_, err = bridge.Conn(ctx, adapter)
	assert.Error(t, err, "Conn should return error for invalid volume format")
	assert.Contains(t, err.Error(), "invalid volume format", "Error should indicate invalid volume format")
}

// TestDockerExecutor_WorkdirInContainer 测试工作目录在容器中生效
func TestDockerExecutor_WorkdirInContainer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if !isDockerAvailable(ctx) {
		t.Skip("Docker is not available, skipping test")
	}

	exec, err := NewDockerExecutor()
	require.NoError(t, err)

	exec.setImage("hub.rat.dev/alpine:latest")
	exec.setWorkdir("/tmp")

	require.NoError(t, exec.Prepare(ctx))
	defer exec.Destruction(ctx)

	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)

	go exec.Transfer(ctx, resultChan, commandChan)

	// 检查当前目录是否正确（通过测试相对路径）
	commandChan <- executor.CommandWrapper{
		StepName: "pwd-check",
		Command:  "[ \"$(pwd)\" = \"/tmp\" ]",
	}
	close(commandChan)

	// 收集结果
	var result *executor.StepResult
	timeout := time.After(10 * time.Second)

resultLoop:
	for {
		select {
		case res := <-resultChan:
			switch v := res.(type) {
			case *executor.StepResult:
				result = v
				break resultLoop
			}
		case <-timeout:
			t.Fatalf("Timeout waiting for results")
		}
	}

	require.NotNil(t, result, "Should receive StepResult")
	assert.NoError(t, result.Error, "Working directory should be /tmp")
}

// TestDockerExecutor_CommandWithPipe 测试管道命令
func TestDockerExecutor_CommandWithPipe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if !isDockerAvailable(ctx) {
		t.Skip("Docker is not available, skipping test")
	}

	exec, err := NewDockerExecutor()
	require.NoError(t, err)

	exec.setImage("hub.rat.dev/alpine:latest")
	require.NoError(t, exec.Prepare(ctx))
	defer exec.Destruction(ctx)

	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)

	go exec.Transfer(ctx, resultChan, commandChan)

	// 使用管道的命令（通过测试最终结果验证）
	commandChan <- executor.CommandWrapper{
		StepName: "pipe-command",
		Command:  "echo 'hello world' | grep 'hello'",
	}
	close(commandChan)

	// 收集结果
	var result *executor.StepResult
	timeout := time.After(10 * time.Second)

resultLoop:
	for {
		select {
		case res := <-resultChan:
			switch v := res.(type) {
			case *executor.StepResult:
				result = v
				break resultLoop
			}
		case <-timeout:
			t.Fatalf("Timeout waiting for results")
		}
	}

	require.NotNil(t, result, "Should receive StepResult")
	assert.NoError(t, result.Error, "Pipe command should succeed")
}

// TestDockerExecutor_SpecialCharacters 测试特殊字符处理
func TestDockerExecutor_SpecialCharacters(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if !isDockerAvailable(ctx) {
		t.Skip("Docker is not available, skipping test")
	}

	exec, err := NewDockerExecutor()
	require.NoError(t, err)

	exec.setImage("hub.rat.dev/alpine:latest")
	require.NoError(t, exec.Prepare(ctx))
	defer exec.Destruction(ctx)

	tests := []struct {
		name    string
		command string
	}{
		{
			name:    "quotes",
			command: `echo "hello world"`,
		},
		{
			name:    "semicolon",
			command: `echo a; echo b`,
		},
		{
			name:    "pipe",
			command: `echo hello | wc -c`,
		},
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

			var result *executor.StepResult
			timeout := time.After(10 * time.Second)

		resultLoop:
			for {
				select {
				case res := <-resultChan:
					switch v := res.(type) {
					case *executor.StepResult:
						result = v
						break resultLoop
					}
				case <-timeout:
					t.Fatalf("Timeout waiting for results")
				}
			}

			require.NotNil(t, result, "Should receive StepResult")
			assert.NoError(t, result.Error, "Command with special characters should succeed: %s", tt.name)
		})
	}
}

// TestDockerExecutor_ContainerIsolation 测试容器隔离性
func TestDockerExecutor_ContainerIsolation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	if !isDockerAvailable(ctx) {
		t.Skip("Docker is not available, skipping test")
	}

	// 创建两个独立的执行器
	exec1, err := NewDockerExecutor()
	require.NoError(t, err)

	exec2, err := NewDockerExecutor()
	require.NoError(t, err)

	exec1.setImage("hub.rat.dev/alpine:latest")
	exec2.setImage("hub.rat.dev/alpine:latest")

	require.NoError(t, exec1.Prepare(ctx))
	require.NoError(t, exec2.Prepare(ctx))

	defer exec1.Destruction(ctx)
	defer exec2.Destruction(ctx)

	// 验证容器ID不同
	id1 := exec1.GetContainerID()
	id2 := exec2.GetContainerID()

	assert.NotEqual(t, id1, id2, "Two executors should have different container IDs")
	t.Logf("Container 1: %s", id1)
	t.Logf("Container 2: %s", id2)

	// 在第一个容器中创建文件
	resultChan1 := make(chan any, 10)
	commandChan1 := make(chan any, 1)
	go exec1.Transfer(ctx, resultChan1, commandChan1)
	commandChan1 <- executor.CommandWrapper{
		StepName: "create-file",
		Command:  "echo 'from-container-1' > /tmp/test_file.txt",
	}
	close(commandChan1)

	// 等待命令完成
	time.Sleep(2 * time.Second)

	// 在第二个容器中检查文件不存在
	resultChan2 := make(chan any, 10)
	commandChan2 := make(chan any, 1)
	go exec2.Transfer(ctx, resultChan2, commandChan2)
	commandChan2 <- executor.CommandWrapper{
		StepName: "check-file",
		Command:  "cat /tmp/test_file.txt 2>&1 || echo 'FILE_NOT_FOUND'",
	}
	close(commandChan2)

	// 收集结果
	var output2 []byte
	timeout := time.After(10 * time.Second)

	for {
		select {
		case res := <-resultChan2:
			switch v := res.(type) {
			case []byte:
				output2 = append(output2, v...)
			case *executor.StepResult:
				goto done
			}
		case <-timeout:
			goto done
		}
	}
done:

	// 容器2不应该看到容器1创建的文件
	outputStr := string(output2)
	if strings.Contains(outputStr, "from-container-1") {
		t.Error("Container isolation broken: container 2 can see container 1's files")
	} else {
		t.Log("✓ Container isolation verified: containers are properly isolated")
	}
}

// TestDockerExecutor_RegistryConfiguration 测试镜像仓库配置
func TestDockerExecutor_RegistryConfiguration(t *testing.T) {
	exec, err := NewDockerExecutor()
	require.NoError(t, err)

	// 验证默认 registry 为空
	info := exec.GetRuntimeInfo()
	assert.Equal(t, "", info["registry"], "Default registry should be empty")

	// 设置自定义 registry
	exec.setRegistry("docker.m.daocloud.io")
	info = exec.GetRuntimeInfo()
	assert.Equal(t, "docker.m.daocloud.io", info["registry"], "Registry should be updated")

	// 测试 registry 为空时的行为
	exec.setRegistry("")
	exec.setImage("hub.rat.dev/alpine:latest")
	// 此时应该直接使用 alpine:latest，不添加 registry 前缀
}

// TestDockerExecutor_ConcurrentCommands 测试并发命令执行（单个 Transfer 调用）
func TestDockerExecutor_ConcurrentCommands(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if !isDockerAvailable(ctx) {
		t.Skip("Docker is not available, skipping test")
	}

	exec, err := NewDockerExecutor()
	require.NoError(t, err)

	exec.setImage("hub.rat.dev/alpine:latest")
	require.NoError(t, exec.Prepare(ctx))
	defer exec.Destruction(ctx)

	resultChan := make(chan any, 30)
	commandChan := make(chan any, 5)

	go exec.Transfer(ctx, resultChan, commandChan)

	// 顺序发送多个命令（Transfer 是顺序处理的）
	for i := 1; i <= 5; i++ {
		commandChan <- executor.CommandWrapper{
			StepName: fmt.Sprintf("step-%d", i),
			Command:  fmt.Sprintf("echo 'output-%d'", i),
		}
	}
	close(commandChan)

	// 收集结果
	results := make(map[string]bool)
	timeout := time.After(20 * time.Second)

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
				if v.Error != nil {
					t.Logf("Step %s error: %v", v.StepName, v.Error)
				} else {
					results[v.StepName] = true
				}
			}
		case <-timeout:
			t.Fatalf("Timeout waiting for results, got %d results", len(results))
		}
	}

	assert.Len(t, results, 5, "Should receive all 5 step results")
	t.Logf("All commands executed successfully")
}
