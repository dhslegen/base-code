// Package typemap 的 MySQL 实现（本文件）。
// MySQL 类型映射使用 modern 日期类型（datetime/timestamp -> LocalDateTime）。
package typemap

import "strings"

// mySQLTypeMapper 实现 TypeMapper 接口（小写 = 包私有，外部通过 NewMySQL 拿到接口）。
// Go 小白知识点：结构体拥有 MapToJavaType 和 MapToJdbcType 方法，即隐式满足 TypeMapper。
type mySQLTypeMapper struct{}

// NewMySQL 返回 MySQL 类型映射器。
// 返回接口类型而非具体类型（TypeMapper 不是 *mySQLTypeMapper），
// 便于日后替换实现而无需改动调用侧——遵循「依赖倒转原则」。
func NewMySQL() TypeMapper { return mySQLTypeMapper{} }

// javaTypes 是「MySQL 类型 → Java 类型」查表。
// key 是标准化的 MySQL 列类型名（小写），value 是对应的 Java 标准类型。
// modern 日期类型：datetime/timestamp 都映射到 LocalDateTime（vs. legacy java.util.Date）。
var javaTypes = map[string]string{
	"varchar": "String", "text": "String", "char": "String",
	"int": "Integer", "integer": "Integer", "tinyint": "Integer", "smallint": "Integer",
	"bigint": "Long",
	"decimal": "BigDecimal", "numeric": "BigDecimal",
	"datetime": "LocalDateTime", "timestamp": "LocalDateTime",
	"date": "LocalDate", "time": "LocalTime",
	"double": "Double", "float": "Float",
	"bit": "Boolean", "boolean": "Boolean",
}

// jdbcTypes 是「MySQL 类型 → JDBC 类型」查表。
// key 同上，value 是 java.sql.Types 的常数名（如 VARCHAR、INTEGER）。
var jdbcTypes = map[string]string{
	"varchar": "VARCHAR", "text": "LONGVARCHAR", "char": "CHAR",
	"int": "INTEGER", "integer": "INTEGER", "tinyint": "TINYINT", "smallint": "SMALLINT",
	"bigint": "BIGINT",
	"decimal": "DECIMAL", "numeric": "NUMERIC",
	"datetime": "TIMESTAMP", "timestamp": "TIMESTAMP",
	"date": "DATE", "time": "TIME",
	"double": "DOUBLE", "float": "REAL",
	"bit": "BIT", "boolean": "BOOLEAN",
}

// MapToJavaType 把 MySQL 列类型映射为 Java 标准类型。
// 兜底策略：未知类型返回 "String"（宽松兼容）。
func (mySQLTypeMapper) MapToJavaType(dbType string) string {
	if v, ok := javaTypes[normalize(dbType)]; ok {
		return v
	}
	return "String" // 兜底
}

// MapToJdbcType 把 MySQL 列类型映射为 JDBC 标准类型。
// 兜底策略：未知类型返回 "VARCHAR"（宽松兼容）。
func (mySQLTypeMapper) MapToJdbcType(dbType string) string {
	if v, ok := jdbcTypes[normalize(dbType)]; ok {
		return v
	}
	return "VARCHAR"
}

// normalize 取列类型主词并小写。
// 例：
//   - "VARCHAR(255)" -> "varchar"
//   - "INT UNSIGNED" -> "int"
//   - "  BigInt  " -> "bigint"
// 实现机制：先小写+修剪，再在第一个「(」或「 」处截断。
func normalize(dbType string) string {
	s := strings.ToLower(strings.TrimSpace(dbType))
	if i := strings.IndexAny(s, "( "); i >= 0 {
		s = s[:i]
	}
	return s
}
