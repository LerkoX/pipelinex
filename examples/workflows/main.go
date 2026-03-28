package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenyingqiao/pipelinex"
	"github.com/chenyingqiao/pipelinex/logger"
)

// PipelineListener 监听流水线事件执行
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

// listWorkflows 列出当前目录下的所有工作流配置文件
func listWorkflows(dir string) ([]string, error) {
	var workflows []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yaml") && entry.Name() != "README.yaml" {
			workflows = append(workflows, entry.Name())
		}
	}

	return workflows, nil
}

// selectWorkflow 让用户选择工作流
func selectWorkflow(workflows []string) (string, error) {
	if len(workflows) == 0 {
		return "", fmt.Errorf("没有找到工作流配置文件")
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\n=== 可用的工作流 ===")
	for i, wf := range workflows {
		fmt.Printf("%d. %s\n", i+1, wf)
	}
	fmt.Println("0. 退出")
	fmt.Printf("\n请选择要运行的工作流 (0-%d): ", len(workflows))

	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("读取输入失败: %w", err)
	}

	input = strings.TrimSpace(input)
	if input == "0" {
		return "", nil
	}

	var choice int
	_, err = fmt.Sscanf(input, "%d", &choice)
	if err != nil || choice < 1 || choice > len(workflows) {
		return "", fmt.Errorf("无效的选择")
	}

	return workflows[choice-1], nil
}

// runPipeline 运行指定的流水线
func runPipeline(configPath string) error {
	fmt.Println("\n=== 运行工作流 ===")
	fmt.Printf("配置文件: %s\n", configPath)
	fmt.Println("----------------------------------------")

	// 创建上下文
	ctx := context.Background()

	// 创建控制台日志推送器
	consolePusher := logger.NewConsolePusher()

	// 创建 Runtime
	runtime := pipelinex.NewRuntime(ctx)
	runtime.SetPusher(consolePusher)

	// 创建监听器
	listener := &PipelineListener{
		pusher: consolePusher,
		ctx:    ctx,
	}

	// 读取配置文件
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}
	configYAML := string(configData)

	// 生成流水线 ID
	pipelineID := fmt.Sprintf("workflow-%s-%s",
		strings.TrimSuffix(filepath.Base(configPath), ".yaml"),
		time.Now().Format("20060102150405"))

	fmt.Printf("Pipeline ID: %s\n", pipelineID)
	fmt.Println("----------------------------------------")

	// 运行流水线
	pipeline, err := runtime.RunSync(ctx, pipelineID, configYAML, listener)
	if err != nil {
		return fmt.Errorf("流水线执行失败: %w", err)
	}

	fmt.Println("----------------------------------------")
	fmt.Println("流水线执行成功!")
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

	return nil
}

func main() {
	fmt.Println("=== PipelineX 工作流运行器 ===")
	fmt.Println()

	// 获取当前目录
	dir, err := os.Getwd()
	if err != nil {
		fmt.Printf("获取当前目录失败: %v\n", err)
		os.Exit(1)
	}

	// 列出工作流
	workflows, err := listWorkflows(dir)
	if err != nil {
		fmt.Printf("列出工作流失败: %v\n", err)
		os.Exit(1)
	}

	// 循环选择工作流
	for {
		selected, err := selectWorkflow(workflows)
		if err != nil {
			fmt.Printf("选择错误: %v\n", err)
			continue
		}

		if selected == "" {
			fmt.Println("退出工作流运行器")
			break
		}

		// 运行工作流
		err = runPipeline(filepath.Join(dir, selected))
		if err != nil {
			fmt.Printf("\n错误: %v\n", err)
		}

		// 询问是否继续
		fmt.Println("\n----------------------------------------")
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("是否继续运行其他工作流? (y/n): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "y" && input != "yes" {
			fmt.Println("退出工作流运行器")
			break
		}
	}
}
