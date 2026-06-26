// Package scanner — PostgreSQL 扫表器实现。
// 移植自 Java PostgreSqlTableScanner，用 information_schema 三段查询读取列/主键/表注释。
//
// # 为什么用三段查询而不是一段？
//
// PostgreSQL 把"列信息"、"约束信息"、"对象注释"存在不同的系统表/视图里：
//  - information_schema.columns：列的基础属性（类型、可空、默认值等）
//  - information_schema.table_constraints + key_column_usage：约束（PRIMARY KEY/UNIQUE 等）
//  - pg_class + obj_description：表/对象级别注释
//
// 三段查询能各自独立失败/重试，逻辑更清晰。
//
// # Go 小白知识点：$1 占位符
//
// PostgreSQL 驱动（如 lib/pq）要求用 $1, $2... 作为查询参数占位符。
// 与 MySQL 的 ? 不同，PostgreSQL 的占位符是有序编号的，
// 同一参数可多次引用（例如 $1 出现两次即传入同一个值两次）。
package scanner

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/dahaoshen/base-code-go/internal/model"
)

// postgreSQLScanner 持有数据库连接，实现 TableScanner 接口。
// Go 小白知识点：结构体嵌入 *sql.DB 作为字段，是 Go 惯用的"组合"模式，等价于 Java 的依赖注入。
type postgreSQLScanner struct{ db *sql.DB }

// NewPostgreSQL 用已打开的 *sql.DB 构造 PostgreSQL 扫表器。
// 返回 TableScanner 接口而非具体类型，调用方不需要知道底层实现。
func NewPostgreSQL(db *sql.DB) TableScanner { return postgreSQLScanner{db: db} }

// pgColumnSQL 查询指定表的列元数据。
// 使用 information_schema.columns（标准 SQL 视图，跨数据库兼容），
// 同时 LEFT JOIN pg_class/pg_attribute 获取 PostgreSQL 特有的列注释。
//
// COALESCE(expr, '') 等价于 Java 的 Objects.toString(x, "")，把 NULL 换成空串。
// col_description(oid, attnum) 是 PG 内置函数，按 (表oid, 列序号) 返回列注释。
//
// pg_class JOIN 上的 relkind = 'r' 不可省：pg_class 是 PG 的"万物表"（表/视图/索引/序列同居一表），
// 若只按 relname 匹配，碰到同名视图/索引会让每列重复出现（笛卡尔积"列膨胀"），
// 生成的 PO 出现重复字段、编译失败。'r' 锁定"普通表"，与下方 pgTableCommentSQL 的过滤保持一致。
const pgColumnSQL = `SELECT column_name, data_type, character_maximum_length, numeric_precision, numeric_scale, is_nullable, column_default, COALESCE(col_description(pgc.oid, pa.attnum), '') as column_comment
FROM information_schema.columns c
LEFT JOIN pg_class pgc ON pgc.relname = c.table_name AND pgc.relkind = 'r'
LEFT JOIN pg_attribute pa ON pa.attrelid = pgc.oid AND pa.attname = c.column_name
WHERE c.table_name = $1 AND c.table_schema = 'public'
ORDER BY c.ordinal_position`

// pgPrimaryKeySQL 查询指定表的主键列名列表。
// 通过 information_schema.table_constraints（约束表）JOIN key_column_usage（约束列表）
// 筛选 constraint_type = 'PRIMARY KEY' 的行。
const pgPrimaryKeySQL = `SELECT kcu.column_name
FROM information_schema.table_constraints tc
JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
WHERE tc.constraint_type = 'PRIMARY KEY' AND tc.table_name = $1 AND tc.table_schema = 'public'`

// pgTableCommentSQL 查询表注释。
// obj_description(oid, catalog_name) 是 PG 内置函数，返回 pg_class 里对象的注释。
// relkind = 'r' 表示只匹配普通表（relation），排除视图、索引等。
const pgTableCommentSQL = `SELECT obj_description(pgc.oid, 'pg_class') as table_comment
FROM pg_class pgc WHERE pgc.relname = $1 AND pgc.relkind = 'r'`

// ScanTable 用 information_schema 三段查询读列/主键/表注释，拼装为 TableMetadata。
//
// 执行顺序：
//  1. 查列（information_schema.columns），可空整数字段用 sql.NullInt64 承接
//  2. 查主键列名，构建集合，回写到已扫描的列上
//  3. 查表注释（取不到不致命，用 _ 忽略错误）
func (s postgreSQLScanner) ScanTable(table string) (model.TableMetadata, error) {
	// ── 第一段：列查询 ──────────────────────────────────────────────────────────
	rows, err := s.db.Query(pgColumnSQL, table)
	if err != nil {
		return model.TableMetadata{}, fmt.Errorf("查询列 %s 失败: %w", table, err)
	}
	// defer 确保无论函数从哪里返回，rows 都被关闭，防止连接泄漏。
	defer rows.Close()

	var cols []model.ColumnMetadata
	for rows.Next() {
		var name, dataType, isNullable, comment string
		// Go 小白知识点：sql.NullInt64 用于接收可能为 NULL 的整数列。
		// 字段 Valid = false 表示数据库返回了 NULL；Valid = true 时 Int64 才有意义。
		var maxLen, precision, scale sql.NullInt64
		// sql.NullString 同理，用于接收可能为 NULL 的字符串列（如 column_default）。
		var def sql.NullString
		if err := rows.Scan(&name, &dataType, &maxLen, &precision, &scale, &isNullable, &def, &comment); err != nil {
			return model.TableMetadata{}, fmt.Errorf("扫描列失败: %w", err)
		}
		cols = append(cols, model.ColumnMetadata{
			ColumnName:    name,
			ColumnType:    buildFullDataType(dataType, maxLen, precision, scale),
			ColumnComment: comment,
		})
	}
	// rows.Err() 检查迭代过程中是否有网络/数据库错误（rows.Next() 遇错会静默停止）。
	if err := rows.Err(); err != nil {
		return model.TableMetadata{}, err
	}

	// ── 第二段：主键查询 ────────────────────────────────────────────────────────
	pkRows, err := s.db.Query(pgPrimaryKeySQL, table)
	if err != nil {
		return model.TableMetadata{}, fmt.Errorf("查询主键失败: %w", err)
	}
	defer pkRows.Close()

	// 用 map 存储主键列名集合，O(1) 查找。等价于 Java 的 Set<String>。
	pkSet := map[string]bool{}
	for pkRows.Next() {
		var c string
		if err := pkRows.Scan(&c); err != nil {
			return model.TableMetadata{}, err
		}
		pkSet[c] = true
	}
	if err := pkRows.Err(); err != nil {
		return model.TableMetadata{}, err
	}

	// 回写主键标记：遍历已扫描列，在 pkSet 里找到的设为主键。
	for i := range cols {
		if pkSet[cols[i].ColumnName] {
			cols[i].IsPrimaryKey = true
		}
	}

	// ── 第三段：表注释查询（取不到不致命）──────────────────────────────────────
	// QueryRow 只取一行，_ 忽略 Scan 错误（无注释时 tableComment.Valid = false）。
	var tableComment sql.NullString
	_ = s.db.QueryRow(pgTableCommentSQL, table).Scan(&tableComment)

	return model.TableMetadata{
		TableName:    table,
		TableComment: tableComment.String, // NullString.String：Valid=false 时为 ""
		Columns:      cols,
	}, nil
}

// buildFullDataType 把 information_schema 的拆分字段拼回完整类型串。
//
// 对齐 Java PostgreSqlTableScanner.buildFullDataType 的逻辑：
//   - character varying(64) → varchar(64)
//   - numeric(10,2)         → numeric(10,2)
//   - timestamp without time zone → timestamp
//   - 其余类型直接返回 dataType 原值
//
// Go 小白知识点：switch 无需 break，default 兜底未匹配的类型。
func buildFullDataType(dataType string, maxLen, precision, scale sql.NullInt64) string {
	switch strings.ToLower(dataType) {
	case "character varying", "varchar":
		// character varying(N) 是 SQL 标准写法；varchar(N) 是别名。
		if maxLen.Valid {
			return fmt.Sprintf("varchar(%d)", maxLen.Int64)
		}
		return "varchar"
	case "character", "char":
		if maxLen.Valid {
			return fmt.Sprintf("char(%d)", maxLen.Int64)
		}
		return "char"
	case "numeric", "decimal":
		// numeric(精度, 小数位数)；精度和小数位数都可能为 NULL（如 numeric 不带参数时）。
		if precision.Valid && scale.Valid {
			return fmt.Sprintf("numeric(%d,%d)", precision.Int64, scale.Int64)
		} else if precision.Valid {
			return fmt.Sprintf("numeric(%d)", precision.Int64)
		}
		return "numeric"
	case "timestamp without time zone":
		// PG 默认时间戳类型，省略时区部分简化为 timestamp。
		return "timestamp"
	case "timestamp with time zone":
		return "timestamptz"
	case "time without time zone":
		return "time"
	case "time with time zone":
		return "timetz"
	default:
		// bigint、boolean、text、uuid 等直接返回，无需特殊处理。
		return dataType
	}
}
