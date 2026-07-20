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
	if err := os.WriteFile(path, []byte("base-code:\n  tables: [t]\n  base-package: com.x\n  output-root: ./x\n  use-jakarta: false\n  datasource:\n    dialect: mysql\n    host: h\n    username: u\n    database: d\n"), 0o644); err != nil {
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
  tables: [t]
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
func strPtr(s string) *string       { return &s }
func intPtr(i int) *int             { return &i }
func boolPtr(b bool) *bool          { return &b }
func colsPtr(c ...string) *[]string { return &c }

// fullOverrides 返回一份可通过必填校验的最小完整内联配置（tables + base-package + db-name 三项）。
func fullOverrides() Overrides {
	return Overrides{
		Tables:      colsPtr("it_user"),
		BasePackage: strPtr("com.example.hello"),
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
	if cfg.OutputRoot != "./src/main/java" {
		t.Errorf("OutputRoot = %q, want ./src/main/java", cfg.OutputRoot)
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
		UseJakarta:     boolPtr(false),
		DbHost:         strPtr("db.prod"),
		DbPort:         intPtr(3307),
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

// TestLoadWithOverrides_MissingRequiredHint 验证缺必填项时错误含缺失 flag 名与三参最短样例。
func TestLoadWithOverrides_MissingRequiredHint(t *testing.T) {
	_, err := LoadWithOverrides("no-such-dir/base-code.yaml", false, Overrides{
		OutputRoot: strPtr("./x"),
	})
	if err == nil {
		t.Fatal("缺 tables/base-package/db-name 应报错")
	}
	for _, want := range []string{"--tables", "--base-package", "--db-name", "base-code gen"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("错误信息应含 %q，实际: %v", want, err)
		}
	}
	// 已有约定默认值的项不应再出现在必填清单里
	for _, banned := range []string{"--db-host", "--db-user", "--output-root", "--dialect"} {
		if strings.Contains(err.Error(), banned) {
			t.Errorf("错误信息不应再含 %q（已有约定默认值），实际: %v", banned, err)
		}
	}
}

// TestLoadWithOverrides_DialectDefaultsMysql 验证未传 dialect 时缺省 mysql（含端口派生 3306）。
func TestLoadWithOverrides_DialectDefaultsMysql(t *testing.T) {
	ov := fullOverrides()
	ov.Dialect = nil
	cfg, err := LoadWithOverrides("no-such-dir/base-code.yaml", false, ov)
	if err != nil {
		t.Fatalf("dialect 缺省应为 mysql 而非报错: %v", err)
	}
	if cfg.Datasource.Dialect != "mysql" || cfg.Datasource.Port != 3306 {
		t.Errorf("缺省方言/端口 = %q/%d, want mysql/3306", cfg.Datasource.Dialect, cfg.Datasource.Port)
	}
}

// TestLoadWithOverrides_ExplicitZeroPortKept 验证显式 --db-port 0 不被方言派生覆盖（指针语义承诺）。
func TestLoadWithOverrides_ExplicitZeroPortKept(t *testing.T) {
	ov := fullOverrides()
	ov.DbPort = intPtr(0)
	cfg, err := LoadWithOverrides("no-such-dir/base-code.yaml", false, ov)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if cfg.Datasource.Port != 0 {
		t.Errorf("显式 --db-port 0 应保留，得到 %d", cfg.Datasource.Port)
	}
}

// TestLoadWithOverrides_ConnectionDefaults 验证连接参数的约定默认值全套下沉。
func TestLoadWithOverrides_ConnectionDefaults(t *testing.T) {
	cfg, err := LoadWithOverrides("no-such-dir/base-code.yaml", false, fullOverrides())
	if err != nil {
		t.Fatalf("最短两参应成功: %v", err)
	}
	ds := cfg.Datasource
	if ds.Host != "127.0.0.1" || ds.Username != "root" || ds.Password != "" {
		t.Errorf("连接默认值 = %s/%s/%q, want 127.0.0.1/root/\"\"", ds.Host, ds.Username, ds.Password)
	}
	if cfg.OutputRoot != "./src/main/java" {
		t.Errorf("OutputRoot = %q, want ./src/main/java", cfg.OutputRoot)
	}
}

// TestLoad_FileValuesBeatConventionDefaults 验证配置文件值优先于约定默认值（不被 127.0.0.1 等覆盖）。
func TestLoad_FileValuesBeatConventionDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	yaml := `base-code:
  tables: [t]
  base-package: com.x
  output-root: ./custom/java
  datasource:
    host: db.prod
    username: svc
    database: d
`
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if cfg.Datasource.Host != "db.prod" || cfg.Datasource.Username != "svc" || cfg.OutputRoot != "./custom/java" {
		t.Errorf("文件值被约定默认值覆盖: %s/%s/%s", cfg.Datasource.Host, cfg.Datasource.Username, cfg.OutputRoot)
	}
}

// TestLoadWithOverrides_WithApiDefaultFalse 验证 with-api 缺省 false（默认不生成 API 层）。
func TestLoadWithOverrides_WithApiDefaultFalse(t *testing.T) {
	cfg, err := LoadWithOverrides("no-such-dir/base-code.yaml", false, fullOverrides())
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if cfg.WithApi == nil || *cfg.WithApi != false {
		t.Errorf("WithApi 缺省应为 false，得到 %v", cfg.WithApi)
	}
}

// TestLoad_WithApiExplicitTrue 验证 yaml 显式 with-api: true 不被缺省 false 覆盖。
func TestLoad_WithApiExplicitTrue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	yaml := `base-code:
  tables: [t]
  base-package: com.x
  with-api: true
  datasource:
    database: d
`
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if cfg.WithApi == nil || *cfg.WithApi != true {
		t.Errorf("yaml 显式 true 应保留，得到 %v", cfg.WithApi)
	}
}

// TestLoad_TablesFromFile 验证配置文件可提供 tables（列表形态），无需 --tables flag。
func TestLoad_TablesFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	yaml := `base-code:
  tables: [sys_user, sys_role]
  base-package: com.x
  datasource:
    database: d
`
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("配置文件提供 tables 应通过校验: %v", err)
	}
	if len(cfg.Tables) != 2 || cfg.Tables[0] != "sys_user" || cfg.Tables[1] != "sys_role" {
		t.Errorf("Tables = %v, want [sys_user sys_role]", cfg.Tables)
	}
}

// TestLoadWithOverrides_TablesFlagOverridesFile 验证 --tables 整体覆盖文件里的 tables。
func TestLoadWithOverrides_TablesFlagOverridesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	yaml := `base-code:
  tables: [file_a, file_b]
  base-package: com.x
  datasource:
    database: d
`
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadWithOverrides(path, true, Overrides{Tables: colsPtr("flag_t")})
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if len(cfg.Tables) != 1 || cfg.Tables[0] != "flag_t" {
		t.Errorf("--tables 应整体覆盖文件值，得到 %v", cfg.Tables)
	}
}

// TestLoadWithOverrides_MissingTablesOnly 验证只缺 tables 时，错误精确指向 --tables。
func TestLoadWithOverrides_MissingTablesOnly(t *testing.T) {
	_, err := LoadWithOverrides("no-such-dir/base-code.yaml", false, Overrides{
		BasePackage: strPtr("com.x"),
		DbName:      strPtr("d"),
	})
	if err == nil {
		t.Fatal("缺 tables 应报错")
	}
	if !strings.Contains(err.Error(), "--tables") {
		t.Errorf("错误信息应含 --tables，实际: %v", err)
	}
	for _, banned := range []string{"--base-package", "--db-name"} {
		if strings.Contains(strings.SplitN(err.Error(), "\n", 2)[0], banned) {
			t.Errorf("缺失清单不应含已提供的 %q，实际: %v", banned, err)
		}
	}
}

// TestLoadWithOverrides_WithApiFlagOverridesFile 验证 flag 显式 true 覆盖缺省 false。
func TestLoadWithOverrides_WithApiFlagOverridesFile(t *testing.T) {
	ov := fullOverrides()
	ov.WithApi = boolPtr(true)
	cfg, err := LoadWithOverrides("no-such-dir/base-code.yaml", false, ov)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if *cfg.WithApi != true {
		t.Error("--with-api 应覆盖缺省 false")
	}
}
