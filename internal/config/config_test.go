// Package config 测试配置加载与约定默认值填充。
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoad_DefaultsApplied 验证配置加载后约定默认值被正确填充。
func TestLoad_DefaultsApplied(t *testing.T) {
	cfg, err := Load("../../testdata/base-code.yaml")
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if cfg.BasePackage != "com.dahaoshen.demo" {
		t.Errorf("BasePackage = %q", cfg.BasePackage)
	}
	if cfg.UseJakarta == nil || !*cfg.UseJakarta { // 缺省 true
		t.Error("UseJakarta 缺省应为 true")
	}
	if cfg.DateType != "modern" { // 缺省 modern
		t.Errorf("DateType = %q, want modern", cfg.DateType)
	}
	// autoFill 缺省约定
	if len(cfg.AutoFill.InsertColumns) == 0 {
		t.Error("autoFill insert 列应有约定缺省值")
	}
	if len(cfg.AutoFill.UpdateColumns) == 0 {
		t.Error("autoFill update 列应有约定缺省值")
	}
}

// TestLoad_EmptyBasePackage 验证缺少 base-package 时返回 error。
func TestLoad_EmptyBasePackage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	if err := os.WriteFile(path, []byte("base-code:\n  output-root: ./x\n  datasource:\n    host: h\n    username: u\n    database: d\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("缺少 base-package 应返回 error")
	}
	if !strings.Contains(err.Error(), "base-package") {
		t.Errorf("错误信息应含 base-package，实际: %v", err)
	}
}

// TestLoad_ExplicitUseJakartaFalse 验证显式 use-jakarta: false 不被默认值 true 覆盖（*bool 的意义）。
func TestLoad_ExplicitUseJakartaFalse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	if err := os.WriteFile(path, []byte("base-code:\n  base-package: com.x\n  output-root: ./x\n  use-jakarta: false\n  datasource:\n    host: h\n    username: u\n    database: d\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UseJakarta == nil || *cfg.UseJakarta != false {
		t.Errorf("显式 false 不应被覆盖，得到: %v", cfg.UseJakarta)
	}
}

// TestLoad_ApiDefaults 验证未配置 api: 时从 base-package 末段派生 service-name 与 base-path。
func TestLoad_ApiDefaults(t *testing.T) {
	cfg, err := Load("../../testdata/base-code.yaml") // base-package: com.dahaoshen.demo，无 api 节
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if cfg.Api.ServiceName != "demo" {
		t.Errorf("ServiceName = %q, want demo（base-package 末段）", cfg.Api.ServiceName)
	}
	if cfg.Api.BasePath != "/demo" {
		t.Errorf("BasePath = %q, want /demo", cfg.Api.BasePath)
	}
}

// TestLoad_ApiExplicit 验证显式 api: 配置优先于派生。
func TestLoad_ApiExplicit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	yaml := `base-code:
  base-package: com.example.hello
  output-root: ./x
  api:
    service-name: hello-service
    base-path: /admin-api/hello
  datasource:
    dialect: mysql
    host: h
    username: u
    database: d
`
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if cfg.Api.ServiceName != "hello-service" {
		t.Errorf("ServiceName = %q, want hello-service", cfg.Api.ServiceName)
	}
	if cfg.Api.BasePath != "/admin-api/hello" {
		t.Errorf("BasePath = %q, want /admin-api/hello", cfg.Api.BasePath)
	}
}

// strPtr / intPtr / boolPtr / colsPtr 是测试用的取址辅助。
// Go 小白知识点：Go 不能对字面量直接取址（&"x" 非法），需借助辅助函数或局部变量。
func strPtr(s string) *string      { return &s }
func intPtr(i int) *int            { return &i }
func boolPtr(b bool) *bool         { return &b }
func colsPtr(c ...string) *[]string { return &c }

// fullOverrides 返回一份可通过必填校验的最小完整内联配置。
func fullOverrides() Overrides {
	return Overrides{
		BasePackage: strPtr("com.example.hello"),
		OutputRoot:  strPtr("./src/main/java"),
		Dialect:     strPtr("mysql"),
		DbHost:      strPtr("127.0.0.1"),
		DbUser:      strPtr("root"),
		DbPassword:  strPtr("pw"),
		DbName:      strPtr("hello"),
	}
}

// TestLoadWithOverrides_PureFlags 验证无配置文件时纯 flag 可完整运行（agent 一行直达）。
func TestLoadWithOverrides_PureFlags(t *testing.T) {
	cfg, err := LoadWithOverrides("no-such-dir/base-code.yaml", false, fullOverrides())
	if err != nil {
		t.Fatalf("纯 flag 模式应成功: %v", err)
	}
	if cfg.BasePackage != "com.example.hello" {
		t.Errorf("BasePackage = %q", cfg.BasePackage)
	}
	if cfg.Datasource.Port != 3306 {
		t.Errorf("mysql 未设端口应派生 3306，得到 %d", cfg.Datasource.Port)
	}
	if cfg.Api.ServiceName != "hello" || cfg.Api.BasePath != "/hello" {
		t.Errorf("api 派生错误: %q %q", cfg.Api.ServiceName, cfg.Api.BasePath)
	}
}

// TestLoadWithOverrides_ExplicitConfigMissing 验证显式 --config 指向缺失文件必须报错。
func TestLoadWithOverrides_ExplicitConfigMissing(t *testing.T) {
	if _, err := LoadWithOverrides("no-such-dir/base-code.yaml", true, fullOverrides()); err == nil {
		t.Fatal("显式指定的配置文件缺失应报错")
	}
}

// TestLoadWithOverrides_FlagOverridesFile 验证 flag 逐项覆盖文件值（含 bool 显式 false）。
func TestLoadWithOverrides_FlagOverridesFile(t *testing.T) {
	cfg, err := LoadWithOverrides("../../testdata/base-code.yaml", true, Overrides{
		UseJakarta: boolPtr(false),
		DbHost:     strPtr("db.prod"),
		DbPort:     intPtr(3307),
		AutoFillInsert: colsPtr("created_at"),
	})
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if *cfg.UseJakarta != false {
		t.Error("--use-jakarta=false 应覆盖缺省 true")
	}
	if cfg.Datasource.Host != "db.prod" || cfg.Datasource.Port != 3307 {
		t.Errorf("datasource 覆盖失败: %s:%d", cfg.Datasource.Host, cfg.Datasource.Port)
	}
	if len(cfg.AutoFill.InsertColumns) != 1 || cfg.AutoFill.InsertColumns[0] != "created_at" {
		t.Errorf("auto-fill 覆盖失败: %v", cfg.AutoFill.InsertColumns)
	}
	// 未覆盖项沿用文件值
	if cfg.Datasource.Database != "demo" {
		t.Errorf("未覆盖项应沿用文件值，得到 %q", cfg.Datasource.Database)
	}
}

// TestLoadWithOverrides_PgPortDerived 验证 postgresql 未设端口派生 5432。
func TestLoadWithOverrides_PgPortDerived(t *testing.T) {
	ov := fullOverrides()
	ov.Dialect = strPtr("postgresql")
	cfg, err := LoadWithOverrides("no-such-dir/base-code.yaml", false, ov)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if cfg.Datasource.Port != 5432 {
		t.Errorf("postgresql 应派生 5432，得到 %d", cfg.Datasource.Port)
	}
}

// TestLoadWithOverrides_MissingRequiredHint 验证缺必填项时错误含缺失 flag 名与命令样例。
func TestLoadWithOverrides_MissingRequiredHint(t *testing.T) {
	_, err := LoadWithOverrides("no-such-dir/base-code.yaml", false, Overrides{
		BasePackage: strPtr("com.example.hello"),
	})
	if err == nil {
		t.Fatal("缺 output-root/db 必填项应报错")
	}
	for _, want := range []string{"--output-root", "--db-host", "--db-user", "--db-name", "base-code gen"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("错误信息应含 %q，实际: %v", want, err)
		}
	}
}
