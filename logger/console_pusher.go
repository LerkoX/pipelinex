package logger

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/fatih/color"
)

// ConsolePusher 控制台日志推送器实现
type ConsolePusher struct {
	mu       sync.RWMutex
	colors   map[Level]*color.Color
	showTime bool
	showNode bool
}

// NewConsolePusher 创建新的控制台日志推送器
func NewConsolePusher() *ConsolePusher {
	return &ConsolePusher{
		showTime: true,
		showNode: true,
		colors: map[Level]*color.Color{
			LevelDebug: color.New(color.FgHiBlack, color.Faint),
			LevelInfo:  color.New(color.FgCyan),
			LevelWarn:  color.New(color.FgYellow),
			LevelError: color.New(color.FgRed, color.Bold),
		},
	}
}

// SetShowTime 设置是否显示时间
func (c *ConsolePusher) SetShowTime(show bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.showTime = show
}

// SetShowNode 设置是否显示节点信息
func (c *ConsolePusher) SetShowNode(show bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.showNode = show
}

// Push 推送单条日志
func (c *ConsolePusher) Push(ctx context.Context, entry Entry) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 设置时间
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// 获取颜色
	colored, ok := c.colors[entry.Level]
	if !ok {
		colored = color.New()
	}

	// 构建前缀
	prefix := c.buildPrefix(entry)

	// 打印日志消息
	if entry.Message != "" {
		msg := colored.Sprintf("%s%s: %s", prefix, entry.Level, entry.Message)
		fmt.Println(msg)
	}

	// 打印输出内容
	if entry.Output != "" {
		output := colored.Sprintf("%sOutput:\n%s", prefix, c.indentOutput(entry.Output))
		fmt.Println(output)
	}

	return nil
}

// PushBatch 批量推送
func (c *ConsolePusher) PushBatch(ctx context.Context, entries []Entry) error {
	for _, entry := range entries {
		if err := c.Push(ctx, entry); err != nil {
			return err
		}
	}
	return nil
}

// Close 关闭连接，刷新缓冲
func (c *ConsolePusher) Close() error {
	// 控制台推送器无需清理
	return nil
}

// buildPrefix 构建日志前缀
func (c *ConsolePusher) buildPrefix(entry Entry) string {
	var prefix string

	if c.showTime {
		prefix += fmt.Sprintf("[%s] ", entry.Timestamp.Format("15:04:05.000"))
	}

	if c.showNode {
		if entry.Pipeline != "" {
			prefix += fmt.Sprintf("[%s]", entry.Pipeline)
		}
		if entry.Node != "" {
			prefix += fmt.Sprintf("[%s]", entry.Node)
		}
		if entry.Step != "" {
			prefix += fmt.Sprintf("[%s]", entry.Step)
		}
		prefix += " "
	}

	return prefix
}

// indentOutput 缩进输出内容
func (c *ConsolePusher) indentOutput(output string) string {
	return "  " + output
}
