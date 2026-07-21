// init_test.go 是 `base-code init` 的 CLI 冒烟测试。
//
// Go 小白知识点：
//   - init 命令写「当前工作目录」，测试用 os.Chdir 切到 t.TempDir() 隔离，
//     t.Cleanup 注册切回原目录（Go 同一包内测试默认串行执行，Chdir 不会相互踩踏）。
//   - 质量闸门：模版产出必须能被真实 config.Load 解析并通过必填校验——
//     未来 schema 演进（改键名/加必填项）导致模版失效时，这里会第一时间红灯。
package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	"github.com/dhslegen/base-code/internal/config"
)

// resetInitFlags 把 initCmd 的所有 flag 重置为刚注册时的状态（同 resetGenFlags 的动机：
// 包级共享命令在多次 Execute 间 Changed/Value 不自动清零，需手动复位防子测试互相污染）。
func resetInitFlags() {
	initCmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	})
}

// chdirTemp 切到临时目录并注册测试结束时切回，返回该目录路径。
func chdirTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	return dir
}

// TestInitCreatesLoadableTemplate 质量闸门：init 产出的模版必须能被真实 config.Load
// 解析并通过 validate（必填三项示例值有效），且 stdout 给出下一步指引。
func TestInitCreatesLoadableTemplate(t *testing.T) {
	resetInitFlags()
	dir := chdirTemp(t)
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"init"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("init 应成功: %v", err)
	}
	cfg, err := config.Load(filepath.Join(dir, "base-code.yaml"))
	if err != nil {
		t.Fatalf("模版必须能被 config.Load 通过（质量闸门）: %v", err)
	}
	if len(cfg.Tables) == 0 || cfg.BasePackage == "" || cfg.Datasource.Database == "" {
		t.Errorf("模版必填三项应有示例值: tables=%v base-package=%q database=%q",
			cfg.Tables, cfg.BasePackage, cfg.Datasource.Database)
	}
	if out := buf.String(); !strings.Contains(out, "base-code gen") {
		t.Errorf("成功输出应含下一步指引 base-code gen，实际:\n%s", out)
	}
}

// TestInitRefusesOverwrite 已存在 base-code.yaml 时应报错（提示 --force）且不改动原文件。
func TestInitRefusesOverwrite(t *testing.T) {
	resetInitFlags()
	dir := chdirTemp(t)
	original := []byte("base-code:\n  base-package: com.keep.me\n")
	if err := os.WriteFile(filepath.Join(dir, "base-code.yaml"), original, 0o644); err != nil {
		t.Fatal(err)
	}
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"init"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("已存在文件时 init 应报错，实际返回 nil")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("错误信息应提示 --force，实际: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "base-code.yaml"))
	if string(got) != string(original) {
		t.Errorf("拒绝覆盖时原文件不得被改动，实际内容:\n%s", got)
	}
}

// TestInitForceOverwrites --force 应覆盖已存在文件为模版内容。
func TestInitForceOverwrites(t *testing.T) {
	resetInitFlags()
	dir := chdirTemp(t)
	if err := os.WriteFile(filepath.Join(dir, "base-code.yaml"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"init", "--force"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("init --force 应成功: %v", err)
	}
	if _, err := config.Load(filepath.Join(dir, "base-code.yaml")); err != nil {
		t.Fatalf("覆盖后的模版应能被 config.Load 通过: %v", err)
	}
}
