package local

import (
	"context"
	"strings"
	"testing"
)

// 命令级环境变量：以真实进程环境变量注入（不经 shell 解析），
// 值中的引号/换行/特殊字符原样保留
func TestCommandLevelEnv(t *testing.T) {
	exec := NewLocalExecutor()
	if err := exec.Prepare(context.Background()); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	defer exec.Destruction(context.Background())

	tricky := "it's \"quoted\" $HOME `id`\n第二行"
	env := map[string]string{
		"FLOWX_TEST_TRICKY": tricky,
		"FLOWX_TEST_PLAIN":  "hello",
	}

	var output strings.Builder
	callback := func(data []byte) { output.Write(data) }

	err := exec.executeCommandWithStreaming(
		context.Background(),
		"printf '%s' \"$FLOWX_TEST_TRICKY\"; echo; printf '%s' \"$FLOWX_TEST_PLAIN\"",
		"test", env, callback, nil, nil,
	)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := output.String()
	if !strings.Contains(got, tricky) {
		t.Errorf("tricky env value not preserved:\n got %q\nwant substring %q", got, tricky)
	}
	if !strings.Contains(got, "hello") {
		t.Errorf("plain env value missing: %q", got)
	}
}
