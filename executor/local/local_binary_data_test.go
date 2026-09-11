package local

import (
	"context"
	"sync"
	"testing"
)

// TestExecuteCommandWithStreaming_ControlCharacters 测试控制字符
func TestExecuteCommandWithStreaming_ControlCharacters(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, `printf 'belltab	newline
'`, "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) == 0 {
		t.Error("Expected some output")
	}
}

// TestExecuteCommandWithStreaming_EscapeSequencesInOutput 测试输出中的转义序列
func TestExecuteCommandWithStreaming_EscapeSequencesInOutput(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, `echo -e '[31mred[0m'`, "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) == 0 {
		t.Error("Expected some output")
	}
}

// TestExecuteCommandWithStreaming_BinaryData 测试二进制数据
func TestExecuteCommandWithStreaming_BinaryData(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, `printf '\\x01\\x02\\x03\\x04\\x05\\x06\\x07\\x08\\x09'`, "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) == 0 {
		t.Error("Expected some output")
	}
}

// TestExecuteCommandWithStreaming_RandomBytes 测试随机字节
func TestExecuteCommandWithStreaming_RandomBytes(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, `printf '￿￾�￼￻￺'`, "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) == 0 {
		t.Error("Expected some output")
	}
}

// TestExecuteCommandWithStreaming_InvalidUTF8 测试无效 UTF-8
func TestExecuteCommandWithStreaming_InvalidUTF8(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, `printf '￿￾ﾀﾁﾂﾃ'`, "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) == 0 {
		t.Error("Expected some output")
	}
}

// TestExecuteCommandWithStreaming_OverlongUTF8 测试过长 UTF-8
func TestExecuteCommandWithStreaming_OverlongUTF8(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, `printf '삀ﾀ'`, "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) == 0 {
		t.Error("Expected some output")
	}
}

// TestExecuteCommandWithStreaming_UTF16 测试 UTF-16
func TestExecuteCommandWithStreaming_UTF16(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, `printf '￿￾ｈ＀･＀ｬ＀ｬ＀ｯ＀'`, "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) == 0 {
		t.Error("Expected some output")
	}
}

// TestExecuteCommandWithStreaming_UTF32 测试 UTF-32
func TestExecuteCommandWithStreaming_UTF32(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, `printf '￿￾＀＀ｈ＀＀＀'`, "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) == 0 {
		t.Error("Expected some output")
	}
}
