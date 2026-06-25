// Package scanner 的测试：用 sqlmock 模拟真实数据库，验证 MySQL 扫表器的行为。
//
// # Go 小白知识点：为什么用 sqlmock？
//
// 真实数据库测试需要启动 MySQL 服务、建表、清理数据，既慢又脆弱。
// go-sqlmock 提供一个实现了 database/sql/driver.Driver 接口的"假数据库"，
// 允许你预先设定期望的 SQL 查询和对应的返回结果，无需真实连接。
package scanner

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	"github.com/dahaoshen/base-code-go/internal/dialect"
)

// TestMySQL_ScanTable 验证 MySQL 扫表器在 happy path 下的完整行为：
//   - 正确解析 SHOW FULL COLUMNS 的两列返回；
//   - id 列因 Key="PRI" 被标记为主键；
//   - name 列注释正确读取；
//   - 表注释从 information_schema 正确读取。
func TestMySQL_ScanTable(t *testing.T) {
	// sqlmock.New() 返回三个值：
	//   db   — 实现了 *sql.DB 接口的假数据库句柄（对外表现和真实 DB 完全一样）
	//   mock — 控制器，用来注册"期望的查询 → 期望的结果"
	//   err  — 构造失败时的错误
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	// defer 关键字：函数返回时（无论正常还是 panic）自动执行，等价于 Java 的 try-with-resources。
	defer db.Close()

	// 模拟 SHOW FULL COLUMNS 返回两列：id（主键）、name。
	// sqlmock.NewRows 指定列名列表；AddRow 按顺序添加一行值。
	// SHOW FULL COLUMNS 返回 9 列：Field, Type, Collation, Null, Key, Default, Extra, Privileges, Comment
	rows := sqlmock.NewRows([]string{"Field", "Type", "Collation", "Null", "Key", "Default", "Extra", "Privileges", "Comment"}).
		AddRow("id", "bigint", nil, "NO", "PRI", nil, "auto_increment", "", "主键").
		AddRow("name", "varchar(64)", "utf8mb4_general_ci", "YES", "", nil, "", "", "姓名")
	// ExpectQuery 注册"期望收到一条 SQL 含此子串的查询"；WillReturnRows 指定返回值。
	mock.ExpectQuery("SHOW FULL COLUMNS FROM").WillReturnRows(rows)

	// 模拟表注释查询。
	mock.ExpectQuery("table_comment").
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow("用户表"))

	s := NewMySQL(db)
	meta, err := s.ScanTable("sys_user")
	if err != nil {
		t.Fatal(err)
	}
	if len(meta.Columns) != 2 {
		t.Fatalf("列数 = %d, want 2", len(meta.Columns))
	}
	if !meta.Columns[0].IsPrimaryKey {
		t.Error("id 应为主键")
	}
	if meta.Columns[1].ColumnComment != "姓名" {
		t.Errorf("name 注释 = %q", meta.Columns[1].ColumnComment)
	}
	if meta.TableComment != "用户表" {
		t.Errorf("表注释 = %q, want 用户表", meta.TableComment)
	}
}

// TestFor_MySQL 验证 For(MySQL, db) 成功返回扫表器（非 nil）。
func TestFor_MySQL(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	scanner, err := For(dialect.MySQL, db)
	if err != nil {
		t.Fatalf("For(MySQL) 意外返回 error: %v", err)
	}
	if scanner == nil {
		t.Error("For(MySQL) 应返回非 nil 扫表器")
	}
}

// TestFor_PostgreSQL 验证 For(PostgreSQL, db) 成功返回扫表器（M3 已实现 PostgreSQL 扫表器）。
//
// Go 小白知识点：测试"预期成功"路径同样重要——功能已实现时应验证无错误返回。
// 注：M1 阶段此测试断言返回 error；M3 实现 PostgreSQL 后更新为断言成功。
func TestFor_PostgreSQL(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	s, err := For(dialect.PostgreSQL, db)
	if err != nil {
		t.Errorf("For(PostgreSQL) 不应返回 error: %v", err)
	}
	if s == nil {
		t.Error("For(PostgreSQL) 应返回非 nil 扫表器")
	}
}
