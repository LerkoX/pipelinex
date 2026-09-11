package flowx

import (
	"context"
	"strings"
	"testing"
	"time"
)

// 端到端：节点 config.env + config.params 经 dag 渲染后由 local 执行器真实注入
func TestRuntime_NodeEnvParams_E2E(t *testing.T) {
	yamlCfg := `
Name: env-e2e
Param:
  base: /data/proj
Executors:
  sh:
    type: local
    config:
      shell: bash
Graph: |
  stateDiagram-v2
    [*] --> A
    A --> [*]
Nodes:
  A:
    executor: sh
    config:
      params:
        message: "{{ Param.base }}/backend"
      env:
        ECHO_MESSAGE: "{{ Param.message }}"
    steps:
      - name: run
        run: |
          printf 'text=%s\n' "$ECHO_MESSAGE"
    extract:
      type: regex
      patterns:
        text: "text=(.*)"
`
	r := NewRuntime(context.Background())

	done := make(chan struct{})
	var gotMeta map[string]interface{}
	_ = gotMeta
	wf, err := r.RunAsync(context.Background(), "env-e2e-1", yamlCfg, nil)
	if err != nil {
		t.Fatalf("RunAsync: %v", err)
	}
	_ = wf
	go func() {
		time.Sleep(15 * time.Second)
		close(done)
	}()

	// 简单轮询 metadata 直到出现 A.text
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		m := wf.Metadata()
		if m != nil {
			if v, ok := m["A.text"]; ok {
				t.Logf("A.text = %v", v.Value)
				if s, _ := v.Value.(string); s == "/data/proj/backend" {
					return
				}
				t.Fatalf("A.text = %q, want /data/proj/backend", v.Value)
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	<-done
	t.Fatalf("A.text not found in metadata: %v", wf.Metadata())
	_ = strings.Contains
}
