// Package scanner 的 PostgreSQL 测试：用 sqlmock 验证三段查询（列、主键、表注释）映射。
//
// # Go 小白知识点：$1 占位符
//
// PostgreSQL 驱动使用 $1, $2... 作为查询参数占位符，
// 而 MySQL 驱动使用 ?。sqlmock 通过 QueryMatcherRegexp 匹配 SQL 片段，
// 所以测试里只需保证 ExpectQuery 包含能唯一标识该查询的子串即可。
//
// # Go 小白知识点：sql.NullInt64 / sql.NullString
//
// information_schema 里有些列（如 character_maximum_length）可能为 NULL。
// Go 的 rows.Scan 遇到 NULL 赋给普通 int64 会报错，
// 必须用 sql.NullInt64（含 Valid bool + Int64 int64）来承接可空整数列。
// 类似地，sql.NullString 承接可空字符串列。
package scanner

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/dahaoshen/base-code-go/internal/dialect"
)

// TestPostgreSQL_ScanTable 验证 PG 扫表：列类型拼装、主键标记、注释。
//
// 三段查询顺序：
//  1. information_schema.columns（列元数据，含可空字段）
//  2. PRIMARY KEY 约束查询（确定哪些列是主键）
//  3. obj_description（取表注释，取不到不致命）
func TestPostgreSQL_ScanTable(t *testing.T) {
	// sqlmock.New() 返回假数据库句柄 db 和控制器 mock。
	// 测试结束后 defer db.Close() 自动释放资源。
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// 1) 列查询（information_schema.columns 拼接）
	// 返回 8 列：column_name, data_type, character_maximum_length,
	//           numeric_precision, numeric_scale, is_nullable,
	//           column_default, column_comment
	// id 列：bigint 类型，无最大字符长度（NULL），精度 64，精度 0，非空，无默认值
	// name 列：character varying，最大长度 64，无精度（NULL），可空，无默认值
	mock.ExpectQuery("information_schema.columns").WillReturnRows(
		sqlmock.NewRows([]string{
			"column_name", "data_type", "character_maximum_length",
			"numeric_precision", "numeric_scale", "is_nullable",
			"column_default", "column_comment",
		}).
			AddRow("id", "bigint", nil, 64, 0, "NO", nil, "主键").
			AddRow("name", "character varying", 64, nil, nil, "YES", nil, "姓名"))

	// 2) 主键查询：返回 id 列名，表示 id 是主键
	mock.ExpectQuery("PRIMARY KEY").WillReturnRows(
		sqlmock.NewRows([]string{"column_name"}).AddRow("id"))

	// 3) 表注释查询：obj_description 是 PG 内置函数，返回对象注释
	mock.ExpectQuery("obj_description").WillReturnRows(
		sqlmock.NewRows([]string{"table_comment"}).AddRow("用户表"))

	s := NewPostgreSQL(db)
	meta, err := s.ScanTable("sys_user")
	if err != nil {
		t.Fatal(err)
	}

	// 断言 1：列数正确
	if len(meta.Columns) != 2 {
		t.Fatalf("列数 = %d, want 2", len(meta.Columns))
	}

	// 断言 2：id 列被标记为主键
	if !meta.Columns[0].IsPrimaryKey {
		t.Error("id 应为主键")
	}

	// 断言 3：name 列的 character varying(64) 应被 buildFullDataType 转换为 varchar(64)
	if meta.Columns[1].ColumnType != "varchar(64)" {
		t.Errorf("name 类型 = %q, want varchar(64)", meta.Columns[1].ColumnType)
	}

	// 断言 4：表注释正确读取
	if meta.TableComment != "用户表" {
		t.Errorf("表注释 = %q", meta.TableComment)
	}
}

// TestFor_PostgreSQL_Scanner 验证 scanner.For 现支持 PostgreSQL（M3 已实现）。
//
// 注意：mysql_test.go 中原 TestFor_PostgreSQL 测试旧行为（期望 error）已更新为成功。
func TestFor_PostgreSQL_Scanner(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := For(dialect.PostgreSQL, db); err != nil {
		t.Errorf("For(PostgreSQL) 不应报错: %v", err)
	}
}
