// Package typemap 的 MySQL 实现（本文件）。
// MySQL 类型映射支持 modern（java.time.*）与 legacy（java.util.Date 等）两种日期类型风格。
package typemap

import "strings"

// mySQLTypeMapper 实现 TypeMapper 接口（小写 = 包私有，外部通过 NewMySQL 拿到接口）。
// Go 小白知识点：结构体拥有 MapToJavaType 和 MapToJdbcType 方法，即隐式满足 TypeMapper。
// dateType 字段控制日期类型风格："modern" → java.time.*，"legacy" → java.util.Date 等。
type mySQLTypeMapper struct{ dateType string }

// NewMySQL 返回 MySQL 类型映射器。
// dateType 传 "modern" 或 "legacy"，控制日期/时间列的 Java 类型。
// 返回接口类型而非具体类型，便于日后替换实现而无需改动调用侧——遵循「依赖倒转原则」。
func NewMySQL(dateType string) TypeMapper { return mySQLTypeMapper{dateType: dateType} }

// javaBase 是与 dateType 无关的固定映射（非 java.lang 类型使用 FQN）。
// Go 小白知识点：FQN（全限定名，如 java.math.BigDecimal）让 Java 模板直接使用，
// 无需 import 语句——Java 编译器能通过完整包路径找到类型定义。
// String/Long/Integer/Boolean 等 java.lang 类型无需 FQN（Java 自动 import java.lang.*）。
var javaBase = map[string]string{
	"varchar": "String", "text": "String", "char": "String", "enum": "String",
	"longtext": "String", "mediumtext": "String", "tinytext": "String",
	"int": "Integer", "integer": "Integer",
	"bigint": "Long", "smallint": "Short",
	"float": "Float", "double": "Double",
	"boolean": "Boolean", "bit": "Boolean",
	"decimal": "java.math.BigDecimal", "numeric": "java.math.BigDecimal", "real": "java.math.BigDecimal",
	"blob": "byte[]", "longblob": "byte[]", "mediumblob": "byte[]", "tinyblob": "byte[]",
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
	"blob": "BLOB", "longblob": "BLOB", "mediumblob": "BLOB", "tinyblob": "BLOB",
	"real": "REAL",
}

// MapToJavaType 把 MySQL 列类型映射为 Java 标准类型（非 java.lang 类型返回 FQN）。
// 兜底策略：未知类型返回 "Object"（对齐原 Java mapper 的 default 分支）。
func (m mySQLTypeMapper) MapToJavaType(dbType string) string {
	norm := normalize(dbType)
	// tinyint(1) 约定为 Boolean，裸 tinyint 为 Integer
	// Go 小白知识点：先特判再 switch，保证 "tinyint(1)" 在 normalize 截断前被正确识别
	if norm == "tinyint" {
		if strings.Contains(dbType, "(1)") {
			return "Boolean"
		}
		return "Integer"
	}
	// 日期时间按 dateType 分支
	switch norm {
	case "date":
		return m.dateOrLegacy("java.time.LocalDate", "java.util.Date")
	case "datetime", "timestamp":
		return m.dateOrLegacy("java.time.LocalDateTime", "java.util.Date")
	case "time":
		return m.dateOrLegacy("java.time.LocalTime", "java.sql.Time")
	}
	if v, ok := javaBase[norm]; ok {
		return v
	}
	return "Object" // 兜底（对齐原 Java mapper 的 default 分支）
}

// dateOrLegacy 按 dateType 在 modern/legacy 之间二选一。
// Go 小白知识点：方法定义在 mySQLTypeMapper 上（值接收者），读取 dateType 字段。
func (m mySQLTypeMapper) dateOrLegacy(modern, legacy string) string {
	if m.dateType == "legacy" {
		return legacy
	}
	return modern
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
