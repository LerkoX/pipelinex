package pipelinex

import (
	"fmt"
	"strings"

	"github.com/flosch/pongo2/v6"
)

// 预检查Pongo2TemplateEngine是否实现了TemplateEngine接口
var _ TemplateEngine = (*Pongo2TemplateEngine)(nil)

// Pongo2TemplateEngine 使用pongo2作为模板引擎的实现
type Pongo2TemplateEngine struct{}

// NewPongo2TemplateEngine 创建一个新的Pongo2模板引擎实例
func NewPongo2TemplateEngine() TemplateEngine {
	return &Pongo2TemplateEngine{}
}

// EvaluateBool 评估模板表达式，返回布尔值
func (e *Pongo2TemplateEngine) EvaluateBool(expression string, ctx map[string]any) (bool, error) {
	// 使用 if-else 形式评估布尔表达式
	// 如果表达式包含 {{ }}，则去除外层
	innerExpr := expression
	if strings.HasPrefix(expression, "{{") && strings.HasSuffix(expression, "}}") {
		innerExpr = strings.TrimSpace(expression[2 : len(expression)-2])
	}

	// 构造 if-else 模板
	// 将布尔字面量替换为字符串字面量
	boolProcessedExpr := strings.ReplaceAll(innerExpr, " true ", " 'true' ")
	boolProcessedExpr = strings.ReplaceAll(boolProcessedExpr, " false ", " 'false' ")
	boolProcessedExpr = strings.ReplaceAll(boolProcessedExpr, "== true", "== 'true'")
	boolProcessedExpr = strings.ReplaceAll(boolProcessedExpr, "== false", "== 'false'")
	boolProcessedExpr = strings.ReplaceAll(boolProcessedExpr, "!= true", "!= 'true'")
	boolProcessedExpr = strings.ReplaceAll(boolProcessedExpr, "!= false", "!= 'false'")
	boolProcessedExpr = strings.ReplaceAll(boolProcessedExpr, "> true", "> 'true'")
	boolProcessedExpr = strings.ReplaceAll(boolProcessedExpr, "< true", "< 'true'")
	boolProcessedExpr = strings.ReplaceAll(boolProcessedExpr, ">= true", ">= 'true'")
	boolProcessedExpr = strings.ReplaceAll(boolProcessedExpr, "<= true", "<= 'true'")

	ifTemplate := "{% if " + boolProcessedExpr + " %}true{% else %}false{% endif %}"
	template, err := pongo2.FromString(ifTemplate)
	if err != nil {
		return false, fmt.Errorf("failed to parse expression '%s': %w", expression, err)
	}

	// 执行模板
	result, err := template.Execute(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to execute expression '%s': %w", expression, err)
	}

	// 将结果转换为布尔值
	result = strings.TrimSpace(result)
	return strings.ToLower(result) == "true", nil
}

// EvaluateString 评估模板表达式，返回字符串
func (e *Pongo2TemplateEngine) EvaluateString(expression string, ctx map[string]any) (string, error) {
	template, err := pongo2.FromString(expression)
	if err != nil {
		return "", fmt.Errorf("failed to parse expression '%s': %w", expression, err)
	}

	result, err := template.Execute(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to execute expression '%s': %w", expression, err)
	}

	return strings.TrimSpace(result), nil
}

// Validate 验证表达式语法是否正确
func (e *Pongo2TemplateEngine) Validate(expression string) error {
	_, err := pongo2.FromString(expression)
	if err != nil {
		return fmt.Errorf("invalid expression syntax: %w", err)
	}
	return nil
}
