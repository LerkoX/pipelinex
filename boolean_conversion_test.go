package pipelinex

import (
	"testing"
)

// TestConvertBoolToString_Simple 测试简单布尔值转换
func TestConvertBoolToString_Simple(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected any
	}{
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"string", "hello", "hello"},
		{"int", 42, 42},
		{"float", 3.14, 3.14},
		{"nil", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertBoolToString(tt.input)
			if result != tt.expected {
				t.Errorf("convertBoolToString(%v) = %v (type %T), want %v (type %T)", tt.input, result, result, tt.expected, tt.expected)
			}
		})
	}
}

// TestConvertBoolToString_NestedMap 测试嵌套 map 中的布尔值转换
func TestConvertBoolToString_NestedMap(t *testing.T) {
	input := map[string]any{
		"enabled":  true,
		"disabled": false,
		"name":     "test",
		"count":    10,
		"nested": map[string]any{
			"innerBool": true,
			"innerName": "inner",
		},
	}

	result := convertBoolToString(input)
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatal("Result should be a map")
	}

	if resultMap["enabled"] != "true" {
		t.Errorf("Expected enabled='true', got '%v'", resultMap["enabled"])
	}
	if resultMap["disabled"] != "false" {
		t.Errorf("Expected disabled='false', got '%v'", resultMap["disabled"])
	}
	if resultMap["name"] != "test" {
		t.Errorf("Expected name='test', got '%v'", resultMap["name"])
	}

	nested, ok := resultMap["nested"].(map[string]any)
	if !ok {
		t.Fatal("nested should be a map")
	}
	if nested["innerBool"] != "true" {
		t.Errorf("Expected innerBool='true', got '%v'", nested["innerBool"])
	}
	if nested["innerName"] != "inner" {
		t.Errorf("Expected innerName='inner', got '%v'", nested["innerName"])
	}
}

// TestConvertBoolToString_MixedInterfaceMap 测试混合类型的 map interface{}
func TestConvertBoolToString_MixedInterfaceMap(t *testing.T) {
	input := map[interface{}]interface{}{
		"success": true,
		"failed":  false,
		"text":    "value",
	}

	result := convertBoolToString(input)
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatal("Result should be a map[string]any")
	}

	if resultMap["success"] != "true" {
		t.Errorf("Expected success='true', got '%v'", resultMap["success"])
	}
	if resultMap["failed"] != "false" {
		t.Errorf("Expected failed='false', got '%v'", resultMap["failed"])
	}
}

// TestDGAEvaluationContext_All_BooleanConversion 测试 All() 方法中的布尔值转换
func TestDGAEvaluationContext_All_BooleanConversion(t *testing.T) {
	ctx := NewEvaluationContext().WithParams(map[string]any{
		"isActive":   true,
		"isDeleted":  false,
		"normalText": "value",
	})

	all := ctx.All()

	if all["isActive"] != "true" {
		t.Errorf("Expected isActive='true', got '%v' (type %T)", all["isActive"], all["isActive"])
	}
	if all["isDeleted"] != "false" {
		t.Errorf("Expected isDeleted='false', got '%v' (type %T)", all["isDeleted"], all["isDeleted"])
	}
	if all["normalText"] != "value" {
		t.Errorf("Expected normalText='value', got '%v'", all["normalText"])
	}
}

// TestDGAEvaluationContext_All_NestedBooleanConversion 测试嵌套结构中的布尔值转换
func TestDGAEvaluationContext_All_NestedBooleanConversion(t *testing.T) {
	ctx := NewEvaluationContext().WithParams(map[string]any{
		"TestResult": map[string]any{
			"passed":       true,
			"failed":       false,
			"total":        100,
			"coverage":     85.5,
			"nestedStatus": map[string]any{
				"ready": true,
			},
		},
	})

	all := ctx.All()

	testResult, ok := all["TestResult"].(map[string]any)
	if !ok {
		t.Fatal("TestResult should be a map")
	}

	if testResult["passed"] != "true" {
		t.Errorf("Expected TestResult.passed='true', got '%v'", testResult["passed"])
	}
	if testResult["failed"] != "false" {
		t.Errorf("Expected TestResult.failed='false', got '%v'", testResult["failed"])
	}

	nestedStatus, ok := testResult["nestedStatus"].(map[string]any)
	if !ok {
		t.Fatal("nestedStatus should be a map")
	}
	if nestedStatus["ready"] != "true" {
		t.Errorf("Expected nestedStatus.ready='true', got '%v'", nestedStatus["ready"])
	}
}

// TestPongo2TemplateEngine_EvaluateBool_StringBooleanComparison 测试字符串布尔值与布尔字面量的比较
func TestPongo2TemplateEngine_EvaluateBool_StringBooleanComparison(t *testing.T) {
	engine := NewPongo2TemplateEngine()

	tests := []struct {
		name     string
		expr     string
		ctx      map[string]any
		expected bool
	}{
		{
			name:     "string true equals 'true'",
			expr:     "{{ value == 'true' }}",
			ctx:      map[string]any{"value": "true"},
			expected: true,
		},
		{
			name:     "string true equals boolean true literal",
			expr:     "{{ value == true }}",
			ctx:      map[string]any{"value": "true"},
			expected: true,
		},
		{
			name:     "string false equals boolean false literal",
			expr:     "{{ value == false }}",
			ctx:      map[string]any{"value": "false"},
			expected: true,
		},
		{
			name:     "string true not equals boolean false literal",
			expr:     "{{ value == false }}",
			ctx:      map[string]any{"value": "true"},
			expected: false,
		},
		{
			name:     "nested object boolean comparison",
			expr:     "{{ test.passed == true }}",
			ctx:      map[string]any{"test": map[string]any{"passed": "true"}},
			expected: true,
		},
		{
			name:     "boolean literal in and condition",
			expr:     "{{ env == 'staging' and check.passed == true }}",
			ctx:      map[string]any{"env": "staging", "check": map[string]any{"passed": "true"}},
			expected: true,
		},
		{
			name:     "boolean literal with not equals",
			expr:     "{{ value != true }}",
			ctx:      map[string]any{"value": "false"},
			expected: true,
		},
		{
			name:     "boolean literal with or condition",
			expr:     "{{ value == true or other == 'yes' }}",
			ctx:      map[string]any{"value": "false", "other": "yes"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.EvaluateBool(tt.expr, tt.ctx)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestPongo2TemplateEngine_EvaluateBool_ComplexCICDCondition 测试类似 CI/CD 流水线的复杂条件
func TestPongo2TemplateEngine_EvaluateBool_ComplexCICDCondition(t *testing.T) {
	engine := NewPongo2TemplateEngine()

	ctx := map[string]any{
		"Param": map[string]any{
			"environment":           "staging",
			"skipTests":             "false",
			"codeCoverageThreshold": 80,
		},
		"QualityCheck": map[string]any{
			"allTestsPassed": "true",
			"codeCoverage":    85,
		},
		"Build": map[string]any{
			"success": "true",
		},
	}

	tests := []struct {
		name     string
		expr     string
		expected bool
	}{
		{
			name:     "staging deployment with passing tests and coverage",
			expr:     "{{ Param.environment == 'staging' and QualityCheck.allTestsPassed == true and QualityCheck.codeCoverage >= Param.codeCoverageThreshold }}",
			expected: true,
		},
		{
			name:     "staging deployment but low coverage",
			expr:     "{{ Param.environment == 'staging' and QualityCheck.allTestsPassed == true and QualityCheck.codeCoverage >= 90 }}",
			expected: false,
		},
		{
			name:     "skip tests is false",
			expr:     "{{ Param.skipTests == false }}",
			expected: true,
		},
		{
			name:     "build success and tests passed",
			expr:     "{{ Build.success == true and QualityCheck.allTestsPassed == true }}",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.EvaluateBool(tt.expr, ctx)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
