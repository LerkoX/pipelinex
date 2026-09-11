package local

import (
	"context"
	"fmt"
	"github.com/LerkoX/flowx/executor"
	"sync"
	"testing"
	"time"
)

// TestExecuteCommandWithStreaming_ContextCancellation 测试上下文取消
func TestExecuteCommandWithStreaming_ContextCancellation(t *testing.T) {
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

// TestTransfer_NilInputChan 测试 nil inputChan
func TestTransfer_NilInputChan(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)

	commandChan <- executor.CommandWrapper{StepName: "test", Command: "echo hello"}
	close(commandChan)

	exec.Transfer(ctx, resultChan, commandChan, nil)

	var foundResult bool
	for result := range resultChan {
		if sr, ok := result.(*executor.StepResult); ok {
			foundResult = true
			if sr.Error != nil {
				t.Errorf("Expected no error, got: %v", sr.Error)
			}
		}
	}

	if !foundResult {
		t.Error("Expected to find a StepResult")
	}
}

// TestTransfer_EmptyCommandChan 测试空 commandChan
func TestTransfer_EmptyCommandChan(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	resultChan := make(chan any, 10)
	commandChan := make(chan any)
	inputChan := make(chan []byte)

	close(commandChan)

	exec.Transfer(ctx, resultChan, commandChan, inputChan)

	if len(resultChan) != 0 {
		t.Errorf("Expected no results for empty commandChan, got %d", len(resultChan))
	}
}

// TestConcurrentTransfers 测试并发 Transfer 调用
func TestConcurrentTransfers(t *testing.T) {
	exec := NewLocalExecutor()

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			ctx := context.Background()
			resultChan := make(chan any, 10)
			commandChan := make(chan any, 1)
			inputChan := make(chan []byte)

			commandChan <- executor.CommandWrapper{
				StepName: fmt.Sprintf("step_%d", index),
				Command:  fmt.Sprintf("echo %d", index),
			}
			close(commandChan)

			exec.Transfer(ctx, resultChan, commandChan, inputChan)

			var foundResult bool
			for result := range resultChan {
				if sr, ok := result.(*executor.StepResult); ok {
					foundResult = true
					if sr.Error != nil {
						t.Errorf("Transfer %d failed: %v", index, sr.Error)
					}
				}
			}
			if !foundResult {
				t.Errorf("Transfer %d: Expected to find a StepResult", index)
			}
		}(i)
	}

	wg.Wait()
}

// TestTransfer_ContextCancellationDuringExecution 测试执行期间上下文取消
func TestTransfer_ContextCancellationDuringExecution(t *testing.T) {
	exec := NewLocalExecutor()

	ctx, cancel := context.WithCancel(context.Background())
	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)
	inputChan := make(chan []byte)

	commandChan <- executor.CommandWrapper{StepName: "test", Command: "sleep 10"}
	close(commandChan)

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	exec.Transfer(ctx, resultChan, commandChan, inputChan)

	var foundResult bool
	for result := range resultChan {
		if sr, ok := result.(*executor.StepResult); ok {
			foundResult = true
			if sr.Error == nil {
				t.Error("Expected error due to context cancellation")
			}
		}
	}

	if !foundResult {
		t.Error("Expected to find a StepResult")
	}
}
