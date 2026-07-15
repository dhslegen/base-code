// Package scanner 的 MySQL 实现。
package scanner

import (
	"database/sql"
	"fmt"

	"github.com/dhslegen/base-code/internal/model"
)

// mySQLScanner 是 TableScanner 接口的 MySQL 实现。
// 小写首字母 = 包内私有，外部只能通过 NewMySQL 工厂函数获取（封装原则）。
type mySQLScanner struct {
	// db 是 database/sql 提供的连接池句柄。
	// *sql.DB 是并发安全的，可在多协程间共享，无需加锁。
	db *sql.DB
}

// NewMySQL 用一个已打开的 *sql.DB 构造 MySQL 扫表器。
//
// 返回类型是接口 TableScanner，而非具体类型 mySQLScanner，
// 这样调用方无法直接访问内部字段，保持良好的封装性。
func NewMySQL(db *sql.DB) TableScanner {
	return mySQLScanner{db: db}
}

// ScanTable 读取指定表的列信息和表注释。
//
// # 实现步骤
//
//  1. SHOW FULL COLUMNS FROM `table` — 获取列名、类型、是否可空、键类型、默认值、扩展信息、注释。
//  2. information_schema.tables — 获取表注释。
//
// # Go 小白知识点：rows.Next() / rows.Scan() 遍历
//
//	rows, err := db.Query(sql)   // 执行查询，返回游标（cursor）
//	defer rows.Close()           // 必须关闭！否则连接不会归还连接池
//	for rows.Next() {            // rows.Next() 将游标移到下一行，无更多行时返回 false
//	    var col string
//	    rows.Scan(&col)          // 将当前行的列值填充到变量（必须传指针）
//	}
//	rows.Err()                   // 检查迭代过程中是否发生了 IO 错误
func (s mySQLScanner) ScanTable(table string) (model.TableMetadata, error) {
	// SHOW FULL COLUMNS 比 DESCRIBE 多返回 Privileges 和 Comment 两列。
	// 注意：表名来自 --tables 命令行参数（可信输入；如需更严可加白名单校验，正则 `^[a-zA-Z0-9_]+$`），
	//       这里用 Sprintf 拼接。
	rows, err := s.db.Query(fmt.Sprintf("SHOW FULL COLUMNS FROM `%s`", table))
	if err != nil {
		return model.TableMetadata{}, fmt.Errorf("SHOW COLUMNS %s 失败: %w", table, err)
	}
	// defer 确保函数返回时（包括中途 return）必然执行 rows.Close()。
	// 若不关闭，底层连接不会归还连接池，最终导致连接耗尽。
	defer rows.Close()

	var cols []model.ColumnMetadata
	for rows.Next() {
		// SHOW FULL COLUMNS 列顺序：Field, Type, Collation, Null, Key, Default, Extra, Privileges, Comment（共 9 列）。
		// sql.NullString 用于可能为 NULL 的列（Collation 在非字符串类型列上为 NULL，Default 列允许 NULL）：
		//   type NullString struct { String string; Valid bool }
		//   Valid=false 表示数据库返回 NULL；Valid=true 时 String 是实际值。
		var field, typ, null, key, extra, priv, comment string
		var collation, def sql.NullString // collation/def 在某些列上可能为 NULL，用 NullString 承接
		// SHOW FULL COLUMNS 9 列依次：Field, Type, Collation, Null, Key, Default, Extra, Privileges, Comment
		if err := rows.Scan(&field, &typ, &collation, &null, &key, &def, &extra, &priv, &comment); err != nil {
			return model.TableMetadata{}, fmt.Errorf("扫描列 [%s] 失败: %w", table, err)
		}
		cols = append(cols, model.ColumnMetadata{
			ColumnName:    field,
			ColumnType:    typ,
			ColumnComment: comment,
			// 简报约定：Key 字段值为 "PRI" 时视为主键（与 MySQL 规范一致）。
			IsPrimaryKey: key == "PRI",
		})
	}
	// rows.Err() 检查 rows.Next() 遍历过程中是否发生网络/IO 错误。
	// 必须在 rows.Close() 之前（这里 defer 会最后执行）、rows.Next() 循环之后调用。
	if err := rows.Err(); err != nil {
		return model.TableMetadata{}, fmt.Errorf("遍历列结果集出错: %w", err)
	}

	// 查询表注释（information_schema.tables）。
	// 用 QueryRow 代替 Query：只期望一行结果，简化代码。
	// 限定当前库 DATABASE() 避免跨库同名表歧义。
	// 忽略此查询的错误——取不到注释不影响代码生成主流程（注释只是辅助信息）。
	var tableComment string
	_ = s.db.QueryRow(
		"SELECT table_comment FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ? LIMIT 1", table,
	).Scan(&tableComment)

	return model.TableMetadata{
		TableName:    table,
		TableComment: tableComment,
		Columns:      cols,
	}, nil
}
