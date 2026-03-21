package pipelinex

import (
	"strings"
	"github.com/google/uuid"
)

// NewUUID 生成新的UUID（去除连字符格式）
func NewUUID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}

// ValidateUUID 验证UUID格式（支持带连字符和不带连字符两种格式）
func ValidateUUID(id string) bool {
	if len(id) == 32 {
		// 无连字符格式，重新添加连字符后验证
		formatted := id[:8] + "-" + id[8:12] + "-" + id[12:16] + "-" + id[16:20] + "-" + id[20:]
		_, err := uuid.Parse(formatted)
		return err == nil
	}
	// 标准带连字符格式
	_, err := uuid.Parse(id)
	return err == nil
}

// FormatUUID 为已有UUID添加连字符（如果是无连字符格式）
func FormatUUID(id string) string {
	if len(id) != 32 {
		return id
	}
	return id[:8] + "-" + id[8:12] + "-" + id[12:16] + "-" + id[16:20] + "-" + id[20:]
}
