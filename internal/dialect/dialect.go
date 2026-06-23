// Package dialect 定义 SQL 方言。
// Go 没有枚举，用「具名字符串类型 + 常量」表达，避免裸 string 满天飞且类型不安全。
package dialect

import "fmt"

// SqlDialect 是底层类型为 string 的「具名类型」（named type，非 type alias：用 type X string 而非 type X = string）。用具名类型而非裸 string 可获得编译期类型安全。
type SqlDialect string

const (
	// MySQL 表示 MySQL 数据库方言。
	MySQL SqlDialect = "mysql"
	// PostgreSQL 表示 PostgreSQL 数据库方言。
	PostgreSQL SqlDialect = "postgresql"
)

// FromValue 解析配置字符串为方言，未知值报错。
// 对应 Java 的 SqlDialect.fromValue 静态工厂方法。
func FromValue(s string) (SqlDialect, error) {
	switch SqlDialect(s) {
	case MySQL:
		return MySQL, nil
	case PostgreSQL:
		return PostgreSQL, nil
	default:
		return "", fmt.Errorf("不支持的 SQL 方言: %q（可选 mysql / postgresql）", s)
	}
}
