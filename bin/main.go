package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/LerkoX/pipelinex"
	"github.com/LerkoX/pipelinex/logger"
)

// PipelineListener监听流水线事件执行
type PipelineListener struct {
	pusher logger.Pusher
	ctx    context.Context
}

func (l *PipelineListener) Handle(p pipelinex.Pipeline, event pipelinex.Event) {
	switch event {
	case pipelinex.PipelineInit:
		if l.pusher != nil {
			l.pusher.Push(l.ctx, logger.Entry{
				Pipeline: p.Id(),
				Level:    logger.LevelInfo,
				Message:  "流水线初始化",
			})
		}
		fmt.Printf("[事件] 流水线初始化: %s\n", p.Id())
	case pipelinex.PipelineStart:
		if l.pusher != nil {
			l.pusher.Push(l.ctx, logger.Entry{
				Pipeline: p.Id(),
				Level:    logger.LevelInfo,
				Message:  "流水线开始执行",
			})
		}
		fmt.Printf("[事件] 流水线开始执行: %s\n", p.Id())
	case pipelinex.PipelineFinish:
		if l.pusher != nil {
			l.pusher.Push(l.ctx, logger.Entry{
				Pipeline: p.Id(),
				Level:    logger.LevelInfo,
				Message:  fmt.Sprintf("流水线执行完成，状态: %s", p.Status()),
			})
		}
		fmt.Printf("[事件] 流水线执行完成: %s, 状态: %s\n", p.Id(), p.Status())
	case pipelinex.PipelineExecutorPrepare:
		if l.pusher != nil {
			l.pusher.Push(l.ctx, logger.Entry{
				Level:   logger.LevelDebug,
				Message: "执行器准备中",
			})
		}
		fmt.Printf("[事件] 执行器准备中\n")
	case pipelinex.PipelineNodeStart:
		if l.pusher != nil {
			l.pusher.Push(l.ctx, logger.Entry{
				Level:   logger.LevelDebug,
				Message: "节点开始执行",
			})
		}
		fmt.Printf("[事件] 节点开始执行\n")
	case pipelinex.PipelineNodeFinish:
		if l.pusher != nil {
			l.pusher.Push(l.ctx, logger.Entry{
				Level:   logger.LevelDebug,
				Message: "节点执行完成",
			})
		}
		fmt.Printf("[事件] 节点执行完成\n")
	default:
		fmt.Printf("[事件] 未知事件: %s\n", event)
	}
}

func (l *PipelineListener) Events() []pipelinex.Event {
	return []pipelinex.Event{
		pipelinex.PipelineInit,
		pipelinex.PipelineStart,
		pipelinex.PipelineFinish,
		pipelinex.PipelineExecutorPrepare,
		pipelinex.PipelineNodeStart,
		pipelinex.PipelineNodeFinish,
	}
}

func main() {
	fmt.Println("=== PipelineX 基础示例 ===")
	fmt.Println()

	// 创建上下文
	ctx := context.Background()

	// 创建控制台日志推送器
	consolePusher := logger.NewConsolePusher()

	// 创建 Runtime
	runtime := pipelinex.NewRuntime(ctx)
	runtime.SetPusher(consolePusher)

	// 创建监听器（带有日志推送器）
	listener := &PipelineListener{
		pusher: consolePusher,
		ctx:    ctx,
	}

	// 读取配置文件
	configData, err := os.ReadFile("./config.yaml")
	if err != nil {
		fmt.Printf("Failed to read config file: %v\n", err)
		os.Exit(1)
	}
	configYAML := string(configData)

	// 同步执行流水线
	pipelineID := "demo-pipeline-" + time.Now().Format("20060102150405")

	fmt.Printf("开始执行流水线: %s\n", pipelineID)
	fmt.Println("----------------------------------------")

	pipeline, err := runtime.RunSync(ctx, pipelineID, configYAML, listener)
	if err != nil {
		fmt.Printf("执行失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("----------------------------------------")
	fmt.Printf("流水线执行成功!\n")
	fmt.Printf("Pipeline ID: %s\n", pipeline.Id())
	fmt.Printf("最终状态: %s\n", pipeline.Status())

	// 打印节点状态
	graph := pipeline.GetGraph()
	nodes := graph.Nodes()
	fmt.Println("\n节点状态:")
	for name, node := range nodes {
		runtimeStatus := node.GetRuntimeStatus()
		if runtimeStatus != nil {
			fmt.Printf("  - %s: %s\n", name, runtimeStatus.Status)
		} else {
			fmt.Printf("  - %s: UNKNOWN\n", name)
		}
	}

	// 打印元数据（如果有）
	metadata := pipeline.Metadata()
	if len(metadata) > 0 {
		fmt.Println("\n元数据:")
		for k, v := range metadata {
			fmt.Printf("  - %s: %v\n", k, v)
		}
	}

	fmt.Println("\n=== 示例执行完成 ===")
}
