// Package typemap 的 PostgreSQL 实现（本文件）。
// PostgreSQL 类型映射支持 modern（java.time.*）与 legacy（java.util.Date 等）两种日期类型风格。
package typemap

// postgreSQLTypeMapper 实现 TypeMapper 接口（小写 = 包私有，外部通过 NewPostgreSQL 拿到接口）。
// Go 小白知识点：结构体拥有 MapToJavaType 和 MapToJdbcType 方法，即隐式满足 TypeMapper。
// dateType 字段控制日期类型风格："modern" → java.time.*，"legacy" → java.util.Date 等。
type postgreSQLTypeMapper struct{ dateType string }

// NewPostgreSQL 返回 PostgreSQL 类型映射器。
// dateType 传 "modern" 或 "legacy"，控制日期/时间列的 Java 类型。
// 返回接口类型而非具体类型，便于日后替换实现而无需改动调用侧——遵循「依赖倒转原则」。
func NewPostgreSQL(dateType string) TypeMapper { return postgreSQLTypeMapper{dateType: dateType} }

// pgJavaBase 是与 dateType 无关的 PG 类型 → Java 类型映射（非 java.lang 用 FQN）。
var pgJavaBase = map[string]string{
	"varchar": "String", "character": "String", "char": "String", "text": "String", "citext": "String", "uuid": "String",
	"int2": "Short", "smallint": "Short",
	"int4": "Integer", "integer": "Integer", "int": "Integer",
	"int8": "Long", "bigint": "Long", "serial": "Long", "bigserial": "Long",
	"float4": "Float", "real": "Float",
	"float8": "Double", "double": "Double",
	"numeric": "java.math.BigDecimal", "decimal": "java.math.BigDecimal", "money": "java.math.BigDecimal",
	"bool": "Boolean", "boolean": "Boolean",
	"bytea": "byte[]",
	"json": "String", "jsonb": "String", "array": "String[]",
	"point": "String", "line": "String", "lseg": "String", "box": "String", "path": "String", "polygon": "String", "circle": "String",
	"cidr": "String", "inet": "String", "macaddr": "String",
	"bit": "String", "varbit": "String",
}

var pgJdbc = map[string]string{
	"varchar": "VARCHAR", "character": "VARCHAR", "char": "VARCHAR", "text": "VARCHAR", "citext": "VARCHAR",
	"uuid": "OTHER",
	"int2": "SMALLINT", "smallint": "SMALLINT",
	"int4": "INTEGER", "integer": "INTEGER", "int": "INTEGER",
	"int8": "BIGINT", "bigint": "BIGINT", "serial": "BIGINT", "bigserial": "BIGINT",
	"float4": "REAL", "real": "REAL",
	"float8": "DOUBLE", "double": "DOUBLE",
	"numeric": "NUMERIC", "decimal": "NUMERIC", "money": "OTHER",
	"bool": "BOOLEAN", "boolean": "BOOLEAN",
	"date": "DATE", "time": "TIME", "timetz": "TIME", "timestamp": "TIMESTAMP", "timestamptz": "TIMESTAMP",
	"bytea": "BINARY", "json": "OTHER", "jsonb": "OTHER", "array": "ARRAY",
}

// MapToJavaType 把 PostgreSQL 列类型映射为 Java 标准类型（非 java.lang 类型返回 FQN）。
// 兜底策略：未知类型返回 "Object"（对齐原 Java mapper 的 default 分支）。
func (m postgreSQLTypeMapper) MapToJavaType(dbType string) string {
	switch norm := normalize(dbType); norm {
	case "date":
		return dateOrLegacyPG(m.dateType, "java.time.LocalDate", "java.util.Date")
	case "time":
		return dateOrLegacyPG(m.dateType, "java.time.LocalTime", "java.sql.Time")
	case "timetz":
		return dateOrLegacyPG(m.dateType, "java.time.OffsetTime", "java.sql.Time")
	case "timestamp":
		return dateOrLegacyPG(m.dateType, "java.time.LocalDateTime", "java.util.Date")
	case "timestamptz":
		return dateOrLegacyPG(m.dateType, "java.time.OffsetDateTime", "java.util.Date")
	default:
		if v, ok := pgJavaBase[norm]; ok {
			return v
		}
		return "Object"
	}
}

// MapToJdbcType 把 PostgreSQL 列类型映射为 JDBC 标准类型。
// 兜底策略：未知类型返回 "OTHER"（宽松兼容）。
func (postgreSQLTypeMapper) MapToJdbcType(dbType string) string {
	if v, ok := pgJdbc[normalize(dbType)]; ok {
		return v
	}
	return "OTHER"
}

// dateOrLegacyPG 按 dateType 二选一（包级函数，PG 与 MySQL 都可复用此模式）。
func dateOrLegacyPG(dateType, modern, legacy string) string {
	if dateType == "legacy" {
		return legacy
	}
	return modern
}
