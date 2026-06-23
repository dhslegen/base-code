// Package scanner 连接数据库读取表结构元数据。
//
// # 设计思路
//
// Java 版通过 JDBC 的 DatabaseMetaData 或直接执行 SQL 读表结构。
// Go 版用标准库 database/sql 执行相同 SQL，逻辑完全对称。
//
// # 接口 vs 实现
//
// Go 接口是隐式实现（implicit interface）：
// 只要某类型拥有接口声明的全部方法，就自动满足该接口，无需 implements 关键字。
// 这使得 TableScanner 可以在测试中被任意 mock 结构体替换。
package scanner

import (
	"database/sql"
	"fmt"

	"github.com/dahaoshen/base-code-go/internal/dialect"
	"github.com/dahaoshen/base-code-go/internal/model"
)

// TableScanner 扫表接口（隐式实现）。
// 调用方只依赖接口，不依赖具体的 mySQLScanner，便于后续扩展 PostgreSQL 实现。
type TableScanner interface {
	// ScanTable 读取指定表的列信息和表注释，返回结构化元数据。
	ScanTable(table string) (model.TableMetadata, error)
}

// For 按 SQL 方言返回对应的扫表器（M1 阶段仅支持 MySQL）。
//
// Go 小白知识点：switch 语句无需 break，默认不会 fall-through（和 Java/C 不同）。
// 对于未知方言，走 default 分支，用 fmt.Errorf 构造带上下文信息的 error 返回。
func For(d dialect.SqlDialect, db *sql.DB) (TableScanner, error) {
	switch d {
	case dialect.MySQL:
		return NewMySQL(db), nil
	default:
		// 将方言值嵌入错误信息，方便调用方诊断配置问题。
		return nil, fmt.Errorf("暂未实现方言 %q 的扫表器", d)
	}
}
