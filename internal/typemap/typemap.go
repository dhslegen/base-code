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

// For 按方言返回对应的 TypeMapper。
// M1 仅实现 MySQL，PostgreSQL 留待 M3 返回 error。
func For(d dialect.SqlDialect) (TypeMapper, error) {
	switch d {
	case dialect.MySQL:
		return NewMySQL(), nil
	default:
		return nil, fmt.Errorf("暂未实现方言 %q 的类型映射", d)
	}
}
