package executor

import (
	"sort"
	"strings"
)

// ShellQuote 用单引号转义字符串，使其成为安全的 shell 字面量
// （值在调用前已完成模板渲染，转义后即可安全拼入命令文本）。
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// EnvExportPrefix 生成 export 行命令前缀，供不支持进程级环境变量注入的
// 执行器（如 kubernetes exec）兜底使用。键排序保证输出稳定。
func EnvExportPrefix(env map[string]string) string {
	if len(env) == 0 {
		return ""
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString("export ")
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(ShellQuote(env[k]))
		sb.WriteString("\n")
	}
	return sb.String()
}
