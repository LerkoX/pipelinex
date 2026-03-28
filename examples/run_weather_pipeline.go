package main

import (
	"context"
	"fmt"
	"os"
	pipelinex "github.com/chenyingqiao/pipelinex"
)

// TestListener 实现 PipelineListener 接口用于监听事件
type TestListener struct{}

func (l *TestListener) OnPipelineEvent(event pipelinex.PipelineEvent) {
	fmt.Printf("[Pipeline] %s: %s\n", event.Type, event.Message)
}

func (l *TestListener) OnNodeEvent(event pipelinex.NodeEvent) {
	fmt.Printf("[Node] %s.%s: %s\n", event.PipelineName, event.NodeName, event.Message)
}

func main() {
	ctx := context.Background()
	runtime := pipelinex.NewRuntime(ctx)

	// 加载天气飞书通知配置
	configPath := "workflows/weather_feishu_notify.yaml"
	configData, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Printf("Failed to read config: %v\n", err)
		os.Exit(1)
	}

	// 创建监听器
	listener := &TestListener{}

	fmt.Println("=== 开始运行天气飞书通知 Pipeline ===")
	fmt.Println()

	// 同步执行 Pipeline
	pipeline, err := runtime.RunSync(ctx, "weather-feishu-notify", string(configData), listener)
	if err != nil {
		fmt.Printf("Pipeline 执行失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("=== Pipeline 执行完成 ===")
	fmt.Printf("Pipeline Name: %s\n", pipeline.GetName())
	fmt.Printf("Pipeline ID: %s\n", pipeline.GetID())

	// 检查节点状态
	nodes := pipeline.GetAllNodes()
	fmt.Println("\n节点状态:")
	for _, node := range nodes {
		state := node.GetState()
		fmt.Printf("  - %s: %s\n", node.GetName(), state)
	}
}
