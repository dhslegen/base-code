// Package typemap 把数据库列类型映射为 Java 类型与 JDBC 类型。
// 核心概念：TypeMapper 接口允许不同数据库方言提供各自的类型映射实现。
package typemap

import (
	"fmt"

	"github.com/dahaoshen/base-code-go/internal/dialect"
)

// TypeMapper 是类型映射接口。
// Go 小白知识点：接口是隐式实现——任何拥有这两个方法的结构体「自动」满足 TypeMapper，
// 无需写 implements 关键字（Java/C# 那种显式继承在 Go 中不存在）。
type TypeMapper interface {
	// MapToJavaType 把数据库列类型映射为 Java 标准类型名，如 "varchar" -> "String"。
	MapToJavaType(dbType string) string
	// MapToJdbcType 把数据库列类型映射为 JDBC 标准类型名，如 "varchar" -> "VARCHAR"。
	MapToJdbcType(dbType string) string
}

// For 按方言返回 TypeMapper，dateType 控制日期类型映射（modern/legacy）。M3 起支持 PostgreSQL。
// Go 小白知识点：dateType 透传给具体 mapper（如 NewMySQL），
// 而不在 For 内硬编码，遵循「单一职责」——For 只负责分发，不决定映射细节。
func For(d dialect.SqlDialect, dateType string) (TypeMapper, error) {
	switch d {
	case dialect.MySQL:
		return NewMySQL(dateType), nil
	default:
		return nil, fmt.Errorf("暂未实现方言 %q 的类型映射", d)
	}
}
