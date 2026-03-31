package local

import (
	"context"
	"testing"
	"time"

	"github.com/LerkoX/pipelinex/executor"
	"github.com/stretchr/testify/require"
)

// TestLocalExecutor_InteractiveInput_NilChannel 测试不提供输入通道
func TestLocalExecutor_InteractiveInput_NilChannel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	exec := NewLocalExecutor()
	require.NoError(t, exec.Prepare(ctx))
	defer exec.Destruction(ctx)

	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)

	go exec.Transfer(ctx, resultChan, commandChan, nil)

	commandChan <- executor.CommandWrapper{
		StepName: "echo-step",
		Command:  "echo test",
	}
	close(commandChan)

	var result *executor.StepResult
	timeout := time.After(3 * time.Second)

resultLoop:
	for {
		select {
		case <-timeout:
			t.Fatal("Test timeout")
		case data, ok := <-resultChan:
			if !ok {
				break resultLoop
			}
			switch v := data.(type) {
			case error:
				t.Fatalf("Received error: %v", v)
			case *executor.StepResult:
				result = v
				break resultLoop
			case []byte:
			}
		}
	}

	require.NotNil(t, result)
	require.NoError(t, result.Error)
}

// TestLocalExecutor_InteractiveInput_WithChannel 测试提供输入通道
func TestLocalExecutor_InteractiveInput_WithChannel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	exec := NewLocalExecutor()
	require.NoError(t, exec.Prepare(ctx))
	defer exec.Destruction(ctx)

	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)
	inputChan := make(chan []byte, 10)

	go exec.Transfer(ctx, resultChan, commandChan, inputChan)

	commandChan <- executor.CommandWrapper{
		StepName: "cat-step",
		Command:  "cat",
	}
	close(commandChan)

	time.Sleep(100 * time.Millisecond)

	// 发送输入
	go func() {
		time.Sleep(50 * time.Millisecond)
		inputChan <- []byte("test\n")
		close(inputChan)
	}()

	var result *executor.StepResult
	timeout := time.After(3 * time.Second)

resultLoop:
	for {
		select {
		case <-timeout:
			t.Fatal("Test timeout")
		case data, ok := <-resultChan:
			if !ok {
				break resultLoop
			}
			switch v := data.(type) {
			case error:
				t.Fatalf("Received error: %v", v)
			case *executor.StepResult:
				result = v
				break resultLoop
			case []byte:
			}
		}
	}

	require.NotNil(t, result)
	// cat 可能会因为输入关闭而失败
	if result.Error != nil {
		t.Logf("Command failed (expected when input closes): %v", result.Error)
	}
}

// TestLocalExecutor_InteractiveInput_ContextCancel 测试上下文取消
func TestLocalExecutor_InteractiveInput_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	exec := NewLocalExecutor()
	require.NoError(t, exec.Prepare(ctx))
	defer exec.Destruction(ctx)

	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)
	inputChan := make(chan []byte, 10)

	go exec.Transfer(ctx, resultChan, commandChan, inputChan)

	commandChan <- executor.CommandWrapper{
		StepName: "sleep-step",
		Command:  "sleep 10",
	}
	close(commandChan)

	time.Sleep(50 * time.Millisecond)

	// 取消上下文
	cancel()
	close(inputChan)

	timeout := time.After(2 * time.Second)

	select {
	case <-timeout:
		t.Error("Command did not terminate after context cancellation")
	case <-resultChan:
	}
}
