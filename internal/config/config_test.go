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
	if err := os.WriteFile(path, []byte("base-code:\n  output-root: ./x\n"), 0o644); err != nil {
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
	if err := os.WriteFile(path, []byte("base-code:\n  base-package: com.x\n  use-jakarta: false\n"), 0o644); err != nil {
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
