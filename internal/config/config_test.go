// Package config 测试配置加载与约定默认值填充。
package config

import "testing"

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
}
