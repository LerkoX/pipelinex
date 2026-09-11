package local

import (
	"context"
	"strings"
	"sync"
	"testing"
)

// TestExecuteCommandWithStreaming_QuotedStrings 测试引号字符串
func TestExecuteCommandWithStreaming_QuotedStrings(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, `echo "hello 'world'"`, "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	foundHello := false
	for _, output := range outputs {
		if strings.Contains(output, "hello") {
			foundHello = true
			break
		}
	}
	if !foundHello {
		t.Errorf("Expected output to contain 'hello', got: %v", outputs)
	}
}

// TestExecuteCommandWithStreaming_EscapeSequences 测试转义序列
func TestExecuteCommandWithStreaming_EscapeSequences(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, `echo -e "line1\nline2"`, "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) < 2 {
		t.Errorf("Expected at least 2 lines, got %d", len(outputs))
	}
}

// TestExecuteCommandWithStreaming_SpecialCharactersInCommand 测试命令中的特殊字符
func TestExecuteCommandWithStreaming_SpecialCharactersInCommand(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, `echo 'special: !@#$%^&*()_+-=[]{}|;:,<>?'`, "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	foundSpecial := false
	for _, output := range outputs {
		if strings.Contains(output, "!@#$%^&*()") {
			foundSpecial = true
			break
		}
	}
	if !foundSpecial {
		t.Errorf("Expected output to contain special chars, got: %v", outputs)
	}
}

// TestExecuteCommandWithStreaming_NewlineHandling 测试换行符处理
func TestExecuteCommandWithStreaming_NewlineHandling(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, `printf 'line1
line2
line3
'`, "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) < 3 {
		t.Errorf("Expected at least 3 lines, got %d", len(outputs))
	}
}

// TestExecuteCommandWithStreaming_CarriageReturn 测试回车符
func TestExecuteCommandWithStreaming_CarriageReturn(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, `printf 'line1
line2
line3
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

// TestExecuteCommandWithStreaming_TabCharacters 测试制表符
func TestExecuteCommandWithStreaming_TabCharacters(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, `printf 'col1	col2	col3
'`, "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	foundTab := false
	for _, output := range outputs {
		if strings.Contains(output, "col1") && strings.Contains(output, "col2") {
			foundTab = true
			break
		}
	}
	if !foundTab {
		t.Errorf("Expected output to contain tab-separated columns, got: %v", outputs)
	}
}

// TestExecuteCommandWithStreaming_Backspace 测试退格符
func TestExecuteCommandWithStreaming_Backspace(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, `printf 'abcd
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

// TestExecuteCommandWithStreaming_NullCharacters 测试空字符
func TestExecuteCommandWithStreaming_NullCharacters(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, `printf 'a\\x00b\\x00c\\n'`, "test", nil, callback, nil, nil)

	mu.Lock()
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	defer mu.Unlock()

	if len(outputs) == 0 {
		t.Error("Expected some output")
	}
}

// TestExecuteCommandWithStreaming_HighUnicode 测试高 Unicode 字符
func TestExecuteCommandWithStreaming_HighUnicode(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, `echo '𠜎 𠜱 𠝹 𠱓 𠱸 𠲖 𠳏 𠳕'`, "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) == 0 {
		t.Error("Expected some output")
	}
}

// TestExecuteCommandWithStreaming_Emoji 测试 Emoji
func TestExecuteCommandWithStreaming_Emoji(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, `echo '🎉 🎊 🎁 🎈 🎀 🎄 🎃 🎅'`, "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) == 0 {
		t.Error("Expected some output")
	}
}

// TestExecuteCommandWithStreaming_MixedEncoding 测试混合编码
func TestExecuteCommandWithStreaming_MixedEncoding(t *testing.T) {
	exec := NewLocalExecutor()

	ctx := context.Background()
	var outputs []string
	var mu sync.Mutex

	callback := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		outputs = append(outputs, string(data))
	}

	err := exec.executeCommandWithStreaming(ctx, `echo 'Hello 世界 🌍 مرحبا'`, "test", nil, callback, nil, nil)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputs) == 0 {
		t.Error("Expected some output")
	}
}
