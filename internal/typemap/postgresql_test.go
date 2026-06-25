// Package typemap 的 PostgreSQL 测试：验证 PG 类型 → Java FQN 映射与 dateType 分支。
package typemap

import (
	"testing"

	"github.com/dahaoshen/base-code-go/internal/dialect"
)

// TestPostgreSQL_JavaType 验证 PG 类型映射（FQN、dateType、JDBC）。
func TestPostgreSQL_JavaType(t *testing.T) {
	m := NewPostgreSQL("modern")
	cases := map[string]string{
		"varchar":     "String",
		"uuid":        "String",
		"int2":        "Short",
		"int4":        "Integer",
		"int8":        "Long",
		"bigserial":   "Long",
		"numeric":     "java.math.BigDecimal",
		"bool":        "Boolean",
		"timestamp":   "java.time.LocalDateTime",
		"timestamptz": "java.time.OffsetDateTime",
		"jsonb":       "String",
		"bytea":       "byte[]",
		"weirdtype":   "Object", // 兜底
	}
	for in, want := range cases {
		if got := m.MapToJavaType(in); got != want {
			t.Errorf("MapToJavaType(%q) = %q, want %q", in, got, want)
		}
	}
	if got := NewPostgreSQL("legacy").MapToJavaType("timestamp"); got != "java.util.Date" {
		t.Errorf("legacy timestamp = %q, want java.util.Date", got)
	}
}

// TestPostgreSQL_JdbcType 验证 PG JDBC 类型映射代表项。
func TestPostgreSQL_JdbcType(t *testing.T) {
	m := NewPostgreSQL("modern")
	cases := map[string]string{
		"int4": "INTEGER", "int8": "BIGINT", "varchar": "VARCHAR",
		"timestamp": "TIMESTAMP", "uuid": "OTHER", "numeric": "NUMERIC",
	}
	for in, want := range cases {
		if got := m.MapToJdbcType(in); got != want {
			t.Errorf("MapToJdbcType(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestFor_PostgreSQL_OK 验证 For 现已支持 PostgreSQL。
func TestFor_PostgreSQL_OK(t *testing.T) {
	if _, err := For(dialect.PostgreSQL, "modern"); err != nil {
		t.Errorf("For(PostgreSQL) 不应报错: %v", err)
	}
}
