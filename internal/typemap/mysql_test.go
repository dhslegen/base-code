// Package typemap 的 MySQL 类型映射单元测试。
// 同时覆盖 MapToJavaType、MapToJdbcType 的正确性及兜底行为，以及 For 按方言取 TypeMapper 的逻辑。
package typemap

import (
	"testing"

	"github.com/dahaoshen/base-code-go/internal/dialect"
)

// TestMySQL_JavaType 验证类型映射：非 java.lang 类型返回全限定名（FQN），按 dateType 分支。
// Go 小白知识点：FQN（全限定名，如 java.time.LocalDateTime）让 Java 模板无需手动 import——
// 直接在代码里写 java.time.LocalDateTime，Java 编译器就能找到，不需要 import 语句。
func TestMySQL_JavaType(t *testing.T) {
	m := NewMySQL("modern")
	cases := map[string]string{
		"varchar":      "String",
		"bigint":       "Long",
		"int":          "Integer",
		"datetime":     "java.time.LocalDateTime", // modern → FQN
		"decimal":      "java.math.BigDecimal",
		"VARCHAR(255)": "String",
		"INT UNSIGNED": "Integer",
		"unknown":      "Object",
	}
	for in, want := range cases {
		if got := m.MapToJavaType(in); got != want {
			t.Errorf("MapToJavaType(%q) = %q, want %q", in, got, want)
		}
	}
	// legacy 分支：java.util.Date（旧版 Java 日期类型）
	if got := NewMySQL("legacy").MapToJavaType("datetime"); got != "java.util.Date" {
		t.Errorf("legacy datetime = %q, want java.util.Date", got)
	}
	// tinyint(1) 约定为 Boolean，裸 tinyint 为 Integer
	if got := m.MapToJavaType("tinyint(1)"); got != "Boolean" {
		t.Errorf("tinyint(1) = %q, want Boolean", got)
	}
	if got := m.MapToJavaType("tinyint"); got != "Integer" {
		t.Errorf("tinyint = %q, want Integer", got)
	}
}

// TestMySQL_JdbcType 验证数据库类型到 JDBC 类型的映射与兜底。
// dateType 对 JDBC 映射无影响，传 "modern" 即可。
func TestMySQL_JdbcType(t *testing.T) {
	m := NewMySQL("modern")
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

// TestFor 验证按方言取 TypeMapper：MySQL 与 PostgreSQL 都应成功。
// dateType 传 "modern"（正常值）验证两个分支。
func TestFor(t *testing.T) {
	if _, err := For(dialect.MySQL, "modern"); err != nil {
		t.Errorf("For(MySQL) 不应报错: %v", err)
	}
	if _, err := For(dialect.PostgreSQL, "modern"); err != nil {
		t.Errorf("For(PostgreSQL) 不应报错: %v", err)
	}
}
