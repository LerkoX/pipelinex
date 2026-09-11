package template

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/flosch/pongo2/v6"
	"gopkg.in/yaml.v3"
)

// rePongo2Identifiers 与 pongo2 context.checkForValidIdentifiers 相同的校验规则
var rePongo2Identifiers = regexp.MustCompile("^[a-zA-Z0-9_]+$")

// sanitizeContext 过滤 pongo2 不接受的非法 context 键（如 NodeID.key 扁平点键）。
// 这类键在 pongo2 中本就无法被引用（模板中 a.b 永远解析为对 map a 的属性访问），
// 而 pongo2 的键校验是致命的——一个坏键会导致整个求值失败，这里做最后兜底
func sanitizeContext(ctx map[string]any) map[string]any {
	for k := range ctx {
		if !rePongo2Identifiers.MatchString(k) {
			filtered := make(map[string]any, len(ctx))
			for key, val := range ctx {
				if rePongo2Identifiers.MatchString(key) {
					filtered[key] = val
				}
			}
			return filtered
		}
	}
	return ctx
}

// 预检查Pongo2TemplateEngine是否实现了TemplateEngine接口
var _ TemplateEngine = (*Pongo2TemplateEngine)(nil)

// 注册自定义过滤器
func init() {
	// flowx 是流水线引擎而非 HTML 模板场景：关闭全局 autoescape，
	// 否则参数值中的引号会被转义为 &#39;/&quot;（且在多次渲染间累积成 &amp;amp;）
	pongo2.SetAutoescape(false)
	pongo2.RegisterFilter("toJson", filterToJSON)
	pongo2.RegisterFilter("toYaml", filterToYaml)
	pongo2.RegisterFilter("toBase64", filterToBase64)
	pongo2.RegisterFilter("fromBase64", filterFromBase64)
	pongo2.RegisterFilter("urlencode", filterURLEncode)
	pongo2.RegisterFilter("urldecode", filterURLDecode)
}

// filterToJSON 将值转换为 JSON 字符串
func filterToJSON(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	var result string

	switch v := in.Interface().(type) {
	case string:
		// 字符串类型，直接返回
		result = v
	case []interface{}:
		// 切片/数组，转换为 JSON 数组
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return nil, &pongo2.Error{
				Sender:    "filter:toJson",
				OrigError: err,
			}
		}
		result = string(jsonBytes)
	case map[string]interface{}:
		// Map，转换为 JSON 对象
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return nil, &pongo2.Error{
				Sender:    "filter:toJson",
				OrigError: err,
			}
		}
		result = string(jsonBytes)
	default:
		// 其他类型，先转换为 interface{} 再 Marshal
		jsonBytes, err := json.Marshal(in.Interface())
		if err != nil {
			return nil, &pongo2.Error{
				Sender:    "filter:toJson",
				OrigError: err,
			}
		}
		result = string(jsonBytes)
	}

	return pongo2.AsSafeValue(result), nil
}

// filterToYaml 将值转换为 YAML 字符串
func filterToYaml(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	yamlBytes, err := yaml.Marshal(in.Interface())
	if err != nil {
		return nil, &pongo2.Error{
			Sender:    "filter:toYaml",
			OrigError: err,
		}
	}
	return pongo2.AsSafeValue(string(yamlBytes)), nil
}

// filterToBase64 将字符串进行 Base64 编码
func filterToBase64(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	str, ok := in.Interface().(string)
	if !ok {
		return nil, &pongo2.Error{
			Sender:    "filter:toBase64",
			OrigError: fmt.Errorf("expected string, got %T", in.Interface()),
		}
	}
	return pongo2.AsSafeValue(base64.StdEncoding.EncodeToString([]byte(str))), nil
}

// filterFromBase64 将 Base64 字符串进行解码
func filterFromBase64(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	str, ok := in.Interface().(string)
	if !ok {
		return nil, &pongo2.Error{
			Sender:    "filter:fromBase64",
			OrigError: fmt.Errorf("expected string, got %T", in.Interface()),
		}
	}
	decoded, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return nil, &pongo2.Error{
			Sender:    "filter:fromBase64",
			OrigError: err,
		}
	}
	return pongo2.AsSafeValue(string(decoded)), nil
}

// filterURLEncode 对字符串进行 URL 编码
func filterURLEncode(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	str, ok := in.Interface().(string)
	if !ok {
		return nil, &pongo2.Error{
			Sender:    "filter:urlencode",
			OrigError: fmt.Errorf("expected string, got %T", in.Interface()),
		}
	}
	return pongo2.AsSafeValue(url.QueryEscape(str)), nil
}

// filterURLDecode 对 URL 编码的字符串进行解码
func filterURLDecode(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	str, ok := in.Interface().(string)
	if !ok {
		return nil, &pongo2.Error{
			Sender:    "filter:urldecode",
			OrigError: fmt.Errorf("expected string, got %T", in.Interface()),
		}
	}
	decoded, err := url.QueryUnescape(str)
	if err != nil {
		return nil, &pongo2.Error{
			Sender:    "filter:urldecode",
			OrigError: err,
		}
	}
	return pongo2.AsSafeValue(decoded), nil
}

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
	result, err := template.Execute(sanitizeContext(ctx))
	if err != nil {
		return false, fmt.Errorf("failed to execute expression '%s': %w", expression, err)
	}

	// 将结果转换为布尔值
	result = strings.TrimSpace(result)
	return strings.ToLower(result) == "true", nil
}

// EvaluateString 评估模板表达式，返回字符串
func (e *Pongo2TemplateEngine) EvaluateString(expression string, ctx map[string]any) (string, error) {
	tmpl, err := pongo2.FromString(expression)
	if err != nil {
		return "", fmt.Errorf("failed to parse expression '%s': %w", expression, err)
	}

	result, err := tmpl.Execute(sanitizeContext(ctx))
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
