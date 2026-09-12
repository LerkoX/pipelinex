package dag

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/LerkoX/flowx/core"
	"gopkg.in/yaml.v3"
)

// OutputExtractor 输出提取器接口
type OutputExtractor interface {
	// Extract 从命令输出中提取数据
	// output: 命令完整输出
	// 返回: 提取的键值对（值为 FieldItem）
	Extract(output string) (map[string]core.FieldItem, error)
}

// CodecBlockExtractor 代码块提取器
// 只识别 ```flowx-yaml 代码块，支持行尾注释提取 description
type CodecBlockExtractor struct {
	maxSize int
}

// NewCodecBlockExtractor 创建代码块提取器
func NewCodecBlockExtractor(maxSize int) *CodecBlockExtractor {
	if maxSize <= 0 {
		maxSize = 1024 * 1024 // 默认 1MB
	}
	return &CodecBlockExtractor{
		maxSize: maxSize,
	}
}

// Extract 从输出中提取代码块
// 解析 ```flowx-yaml 代码块，提取行尾注释作为 description
func (e *CodecBlockExtractor) Extract(output string) (map[string]core.FieldItem, error) {
	result := make(map[string]core.FieldItem)

	// 检查输出大小限制 - 截取末尾内容
	if len(output) > e.maxSize {
		output = output[len(output)-e.maxSize:]
		fmt.Printf("Warning: Output truncated to %d bytes (showing last %d bytes) due to size limit\n", len(output), e.maxSize)
	}

	// 查找 YAML 代码块。
	// 闭合围栏必须独占一行（前导为换行）：节点输出的值常是 json.dumps 单行字符串，
	// 内部可能含字面 Markdown 代码围栏 ```（如 vibe 节点的 summary/transcript），
	// 非贪婪匹配到「任意位置的 ```」会在值中间提前截断，导致整块 YAML 解析失败、
	// 节点输出全部丢失（exec 158/159 事故）。
	yamlPattern := regexp.MustCompile("(?s)```flowx-yaml[^\\n]*\\n(.*?)\\n```[^\\n]*(\\n|$)")
	yamlMatches := yamlPattern.FindAllStringSubmatch(output, -1)
	for _, match := range yamlMatches {
		if len(match) >= 2 {
			yamlContent := match[1]
			// 提取注释
			descriptions := e.extractComments(yamlContent)
			// 移除注释后解析 YAML
			cleanedYaml := e.removeComments(yamlContent)

			var data map[string]interface{}
			if err := yaml.Unmarshal([]byte(cleanedYaml), &data); err != nil {
				fmt.Printf("Warning: Failed to parse YAML code block: %v\n", err)
				continue
			}

			// 转换为 FieldItem
			for k, v := range data {
				fieldItem := core.FieldItem{
					Value:       v,
					Description: descriptions[k],
					SrcNode:     "", // 由调用方设置
				}
				result[k] = fieldItem
			}
		}
	}

	return result, nil
}

// extractComments 提取 YAML 中的行尾注释
// 匹配: key: value # comment
func (e *CodecBlockExtractor) extractComments(yamlStr string) map[string]string {
	descriptions := make(map[string]string)
	lines := strings.Split(yamlStr, "\n")

	linePattern := regexp.MustCompile(`^\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*:\s*.*?\s*#\s*(.+)$`)

	for _, line := range lines {
		match := linePattern.FindStringSubmatch(line)
		if match != nil && len(match) >= 3 {
			key := match[1]
			comment := strings.TrimSpace(match[2])
			descriptions[key] = comment
		}
	}

	return descriptions
}

// removeComments 移除 YAML 中的行尾注释
func (e *CodecBlockExtractor) removeComments(yamlStr string) string {
	pattern := regexp.MustCompile(`\s*#.*$`)
	return pattern.ReplaceAllString(yamlStr, "")
}

// RegexExtractor 正则表达式提取器
type RegexExtractor struct {
	patterns map[string]*regexp.Regexp
	maxSize  int
}

// NewRegexExtractor 创建正则表达式提取器
func NewRegexExtractor(patterns map[string]string, maxSize int) (*RegexExtractor, error) {
	if maxSize <= 0 {
		maxSize = 1024 * 1024 // 默认 1MB
	}

	compiledPatterns := make(map[string]*regexp.Regexp)
	for key, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile pattern for key %s: %w", key, err)
		}
		compiledPatterns[key] = re
	}

	return &RegexExtractor{
		patterns: compiledPatterns,
		maxSize:  maxSize,
	}, nil
}

// Extract 使用正则表达式从输出中提取数据
// 返回值为 FieldItem，Description 和 SrcNode 由调用方设置
func (e *RegexExtractor) Extract(output string) (map[string]core.FieldItem, error) {
	result := make(map[string]core.FieldItem)

	// 检查输出大小限制 - 截取末尾内容
	if len(output) > e.maxSize {
		output = output[len(output)-e.maxSize:]
		fmt.Printf("Warning: Output truncated to %d bytes (showing last %d bytes) due to size limit\n", len(output), e.maxSize)
	}

	// 遍历所有正则表达式模式
	for key, re := range e.patterns {
		matches := re.FindStringSubmatch(output)
		if len(matches) >= 2 {
			value := matches[1]
			// 使用第一个捕获组作为值
			result[key] = core.FieldItem{
				Value:       value,
				Description: "", // 正则提取没有注释
				SrcNode:     "", // 由调用方设置
			}
		} else if len(matches) == 1 {
			// 如果没有捕获组，使用整个匹配
			result[key] = core.FieldItem{
				Value:       matches[0],
				Description: "",
				SrcNode:     "",
			}
		}
	}

	return result, nil
}
