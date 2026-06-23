// Package typemap 的 MySQL 类型映射单元测试。
// 同时覆盖 MapToJavaType、MapToJdbcType 的正确性及兜底行为，以及 For 按方言取 TypeMapper 的逻辑。
package typemap

import (
	"testing"

	"github.com/dahaoshen/base-code-go/internal/dialect"
)

// TestMySQL_JavaType 测试 MySQL 类型映射到 Java 类型的正确性。
func TestMySQL_JavaType(t *testing.T) {
	m := NewMySQL()
	cases := map[string]string{
		"varchar":      "String",
		"bigint":       "Long",
		"int":          "Integer",
		"datetime":     "LocalDateTime", // modern 日期类型
		"unknown":      "String",        // 兜底
		"VARCHAR(255)": "String",        // 带长度也能识别（normalize 截断）
		"INT UNSIGNED": "Integer",       // 带修饰也能识别
	}
	for in, want := range cases {
		if got := m.MapToJavaType(in); got != want {
			t.Errorf("MapToJavaType(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestMySQL_JdbcType 验证数据库类型到 JDBC 类型的映射与兜底。
func TestMySQL_JdbcType(t *testing.T) {
	m := NewMySQL()
	cases := map[string]string{
		"varchar":      "VARCHAR",
		"bigint":       "BIGINT",
		"datetime":     "TIMESTAMP",
		"VARCHAR(255)": "VARCHAR",  // 带长度也能识别（normalize 截断）
		"INT UNSIGNED": "INTEGER",  // 带修饰也能识别
		"unknown":      "VARCHAR",  // 兜底
	}
	for in, want := range cases {
		if got := m.MapToJdbcType(in); got != want {
			t.Errorf("MapToJdbcType(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestFor 验证按方言取 TypeMapper：MySQL 成功，PostgreSQL（暂未实现）返回 error。
func TestFor(t *testing.T) {
	if _, err := For(dialect.MySQL); err != nil {
		t.Errorf("For(MySQL) 不应报错: %v", err)
	}
	if _, err := For(dialect.PostgreSQL); err == nil {
		t.Error("For(PostgreSQL) 应返回 error（暂未实现）")
	}
}
