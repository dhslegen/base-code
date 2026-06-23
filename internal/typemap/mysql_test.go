// Package typemap 的 MySQL 类型映射单元测试。
// 测试 MapToJavaType 和 MapToJdbcType 方法的正确性及兜底行为。
package typemap

import "testing"

// TestMySQL_JavaType 测试 MySQL 类型映射到 Java 类型的正确性。
func TestMySQL_JavaType(t *testing.T) {
	m := NewMySQL()
	cases := map[string]string{
		"varchar":  "String",
		"bigint":   "Long",
		"int":      "Integer",
		"datetime": "LocalDateTime", // modern 日期类型
		"unknown":  "String",        // 兜底
	}
	for in, want := range cases {
		if got := m.MapToJavaType(in); got != want {
			t.Errorf("MapToJavaType(%q) = %q, want %q", in, got, want)
		}
	}
}
