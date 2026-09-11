package dag

import (
	"testing"

	"github.com/LerkoX/flowx/core"
	"github.com/LerkoX/flowx/metadata"
	"github.com/LerkoX/flowx/template"
)

// 节点级参数作用域：节点 params 绑定优先于 workflow 级 Param，
// 绑定值中的模板先在 workflow 上下文求值（支持嵌入片段、上游输出）
func TestBuildNodeRenderContext_NodeParams(t *testing.T) {
	wf := NewWorkflow(t.Context()).(*WorkflowImpl)
	wf.SetTemplateEngine(template.NewPongo2TemplateEngine())
	wf.SetParamForTest(map[string]interface{}{
		"project_dir": "/data/proj",
		"shared":      "workflow-level",
	})

	node := NewDGANodeWithConfig("N1", "", "", "", nil, map[string]any{
		"params": map[string]string{
			"constant":  "/opt/fixed",
			"fromParam": "{{ Param.project_dir }}",
			"fragment":  "{{ Param.project_dir }}/backend",
			"shared":    "node-level", // 遮蔽 workflow 同名参数
		},
	})

	ctx := wf.buildNodeRenderContext(node)
	param, _ := ctx["Param"].(map[string]any)

	if got := param["constant"]; got != "/opt/fixed" {
		t.Errorf("constant binding = %v", got)
	}
	if got := param["fromParam"]; got != "/data/proj" {
		t.Errorf("fromParam binding = %v", got)
	}
	if got := param["fragment"]; got != "/data/proj/backend" {
		t.Errorf("fragment binding = %v, want /data/proj/backend", got)
	}
	if got := param["shared"]; got != "node-level" {
		t.Errorf("node binding should shadow workflow param, got %v", got)
	}
	// 顶层展开一致
	if got := ctx["fragment"]; got != "/data/proj/backend" {
		t.Errorf("top-level fragment = %v", got)
	}
}

// 无 params 的节点回退到 workflow 级 Param（兼容旧行为）
func TestBuildNodeRenderContext_Fallback(t *testing.T) {
	wf := NewWorkflow(t.Context()).(*WorkflowImpl)
	wf.SetTemplateEngine(template.NewPongo2TemplateEngine())
	wf.SetParamForTest(map[string]interface{}{"a": "wf-a"})

	node := NewDGANodeWithConfig("N1", "", "", "", nil, nil)
	ctx := wf.buildNodeRenderContext(node)
	param, _ := ctx["Param"].(map[string]any)
	if got := param["a"]; got != "wf-a" {
		t.Errorf("fallback to workflow param failed, got %v", got)
	}
}

// 绑定值引用上游节点输出（metadata）
func TestBuildNodeRenderContext_UpstreamOutput(t *testing.T) {
	wf := NewWorkflow(t.Context()).(*WorkflowImpl)
	wf.SetTemplateEngine(template.NewPongo2TemplateEngine())
	store, err := metadata.NewInConfigMetadataStore(core.MetadataConfig{
		Type: "in-config",
		Data: map[string]interface{}{"Up.city": "shanghai"},
	})
	if err != nil {
		t.Fatalf("metadata store: %v", err)
	}
	defer store.Close()
	wf.SetMetadata(store)

	node := NewDGANodeWithConfig("N1", "", "", "", nil, map[string]any{
		"params": map[string]string{"city": "{{ Up.city }}"},
	})
	ctx := wf.buildNodeRenderContext(node)
	param, _ := ctx["Param"].(map[string]any)
	if got := param["city"]; got != "shanghai" {
		t.Errorf("upstream output binding = %v", got)
	}
}
