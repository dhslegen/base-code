// generate_test.go 测试编排器的三个核心行为：
//  1. TestOutputPath       — 路径约定是否正确（po/mapper/service 三层）
//  2. TestGenerate_WritesFiles — 非 dry-run 时是否落盘
//  3. TestGenerate_DryRunNoWrite — dry-run 时只写 out、不落盘
//  4. TestBuildTemplateData_NoKey — 无主键/复合主键时 BuildTemplateData 应返回 error
//
// Go 小白知识点：
//   - bytes.Buffer 实现了 io.Writer 接口，测试时可用它"截获"写出内容，无需真正的文件或 os.Stdout。
//   - t.TempDir() 在测试结束后自动清理临时目录，省去手动 defer os.RemoveAll 的麻烦。
//   - os.ReadDir 读目录条目，len==0 证明没有任何文件被创建，用于验证 dry-run 不落盘。
package generator

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/dahaoshen/base-code-go/internal/config"
	"github.com/dahaoshen/base-code-go/internal/model"
)

// sampleMeta 构造 sys_user 表的最小化元数据：一个 bigint 主键 + 一个 varchar 字段。
func sampleMeta() model.TableMetadata {
	return model.TableMetadata{
		TableName: "sys_user", TableComment: "用户表",
		Columns: []model.ColumnMetadata{
			{ColumnName: "id", ColumnType: "bigint", ColumnComment: "主键", IsPrimaryKey: true},
			{ColumnName: "user_name", ColumnType: "varchar(64)", ColumnComment: "用户名"},
		},
	}
}

// sampleCfg 构造一个指向临时目录 out 的最小化配置。
// UseJakarta 用指针是因为 config.Config.UseJakarta 为 *bool，以区分"未配置"和"明确 false"。
func sampleCfg(out string) config.Config {
	jakarta := true
	return config.Config{
		BasePackage: "com.dahaoshen.demo", OutputRoot: out, Author: "tester",
		UseJakarta: &jakarta, DateType: "modern",
		Datasource: config.Datasource{Dialect: "mysql"},
		AutoFill:   config.AutoFill{InsertColumns: []string{"created_at"}, UpdateColumns: []string{"updated_at"}},
	}
}

// TestOutputPath 验证按约定推导落盘路径：po 文件名无后缀，mapper/service 加首字母大写层名后缀。
func TestOutputPath(t *testing.T) {
	cases := []struct {
		layer string
		want  string
	}{
		{"po", "/root/com/dahaoshen/demo/model/po/SysUser.java"},
		{"mapper", "/root/com/dahaoshen/demo/mapper/SysUserMapper.java"},
		{"service", "/root/com/dahaoshen/demo/service/SysUserService.java"},
	}
	for _, tc := range cases {
		got := OutputPath(tc.layer, "com.dahaoshen.demo", "/root", "SysUser")
		want := filepath.FromSlash(tc.want)
		if got != want {
			t.Errorf("layer=%s: OutputPath = %q, want %q", tc.layer, got, want)
		}
	}
}

// TestGenerate_WritesFiles 验证 dryRun=false 时三层文件均落盘。
// 只检查 po 文件是否存在（作为代表性断言；mapper/service 同理生成）。
func TestGenerate_WritesFiles(t *testing.T) {
	dir := t.TempDir()
	cfg := sampleCfg(dir)
	// dryRun=false, out=nil — 不需要 io.Writer，传 nil 即可
	if err := Generate(cfg, sampleMeta(), []string{"po", "mapper", "service"}, false, nil); err != nil {
		t.Fatal(err)
	}
	// po 文件名无后缀：SysUser.java（不是 SysUserPo.java）
	poPath := filepath.Join(dir, "com/dahaoshen/demo/model/po/SysUser.java")
	if _, err := os.Stat(poPath); err != nil {
		t.Errorf("未生成 po 文件: %v", err)
	}
}

// TestGenerate_DryRunNoWrite 验证 dryRun=true 时：
//  1. 内容写入 out（buf.Len() > 0）
//  2. 临时目录保持空（不落盘）
//  3. 约定路径上确实未创建文件
//
// 这展示了 io.Writer 接口的核心价值：Generate 内部对 out 只调用 fmt.Fprintf，
// 测试传 bytes.Buffer、main 传 os.Stdout，代码一字不改。
func TestGenerate_DryRunNoWrite(t *testing.T) {
	dir := t.TempDir()
	cfg := sampleCfg(dir)
	var buf bytes.Buffer
	if err := Generate(cfg, sampleMeta(), []string{"po"}, true, &buf); err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Error("dry-run 应把内容写到 out")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Error("dry-run 不应落盘")
	}
	// 精准验证：po 层约定路径上确实未创建文件
	poPath := filepath.Join(dir, "com/dahaoshen/demo/model/po/SysUser.java")
	if _, err := os.Stat(poPath); !os.IsNotExist(err) {
		t.Error("dry-run 不应在约定路径落盘")
	}
}

// TestBuildTemplateData_NoPrimaryKey 验证无主键时 BuildTemplateData 返回 error。
// Go 中"快速失败"（fail-fast）是惯用手法：非法状态尽早 return error，
// 避免后续代码对 TemplateData 的零值做出错误假设。
func TestBuildTemplateData_NoPrimaryKey(t *testing.T) {
	meta := model.TableMetadata{
		TableName:    "no_pk_table",
		TableComment: "无主键表",
		Columns: []model.ColumnMetadata{
			{ColumnName: "name", ColumnType: "varchar(64)", ColumnComment: "名称"},
		},
	}
	jakarta := true
	cfg := config.Config{
		BasePackage: "com.test", Author: "tester",
		UseJakarta: &jakarta, DateType: "modern",
		Datasource: config.Datasource{Dialect: "mysql"},
	}
	_, err := BuildTemplateData(meta, cfg)
	if err == nil {
		t.Error("无主键时 BuildTemplateData 应返回 error，但返回了 nil")
	}
}

// TestBuildTemplateData_CompositePrimaryKey 验证复合主键时 BuildTemplateData 返回 error。
// 框架（MyBatis-Plus @TableId）只支持单列主键，复合主键必须快速失败。
func TestBuildTemplateData_CompositePrimaryKey(t *testing.T) {
	meta := model.TableMetadata{
		TableName:    "composite_pk_table",
		TableComment: "复合主键表",
		Columns: []model.ColumnMetadata{
			{ColumnName: "pk1", ColumnType: "bigint", IsPrimaryKey: true},
			{ColumnName: "pk2", ColumnType: "bigint", IsPrimaryKey: true},
		},
	}
	jakarta := true
	cfg := config.Config{
		BasePackage: "com.test", Author: "tester",
		UseJakarta: &jakarta, DateType: "modern",
		Datasource: config.Datasource{Dialect: "mysql"},
	}
	_, err := BuildTemplateData(meta, cfg)
	if err == nil {
		t.Error("复合主键时 BuildTemplateData 应返回 error，但返回了 nil")
	}
}
