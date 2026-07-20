// gen_test.go 是 `base-code gen` 的 CLI 门面冒烟测试（不连数据库，只验证参数解析与帮助渲染）。
//
// Go 小白知识点：
//   - cobra.Command 的 Execute() 有个反直觉细节：不管你在哪个子命令上调用 Execute()，
//     内部都会跳到根命令重新执行（"Regardless of what command execute is called on, run on Root only"，
//     见 cobra command.go ExecuteC() 源码）。因此本文件测试时必须对 rootCmd（而非 genCmd）调用
//     SetArgs/SetOut/SetErr/Execute，否则 genCmd 上设置的 args 会被忽略，测试形同虚设。
//   - rootCmd/genCmd 是包级共享变量（跨整个 cmd 包生命周期只有一份），cobra 的 flag「是否被显式提供」
//     （Changed 字段）在多次 Execute 之间不会自动清零——上一个子测试设置过的 --help/--with-api 等
//     状态会残留，污染下一个子测试的解析结果。resetGenFlags 在每个子测试开始前把 genCmd 的所有 flag
//     重置为刚注册时的状态（Changed=false、Value=DefValue），避免子测试互相干扰。
package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

// resetGenFlags 把 genCmd 的所有 flag 重置为刚注册时的状态。
//
// 注意：cobra 的 --help/--version 内建 flag 是「懒初始化」的——只有命令第一次真正执行过
// （c.execute 内部调用 InitDefaultHelpFlag）才会出现在 genCmd.Flags() 里。VisitAll 在它们
// 尚未注册时自然不会遍历到，不会报错，所以本函数可以安全地在任意子测试之前调用。
func resetGenFlags() {
	genCmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	})
}

// TestGenHelpGroupedOutput 覆盖 F2/F3/F4/F6：`gen --help` 应按四个语义分组渲染，
// 且含"默认:"字样与关键 flag（--with-api / --base-package）。
func TestGenHelpGroupedOutput(t *testing.T) {
	resetGenFlags()
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"gen", "--help"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("gen --help 不应返回 error，got: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"生成目标", "数据库连接", "API 层", "生成行为", "默认:", "--with-api", "--base-package"} {
		if !strings.Contains(out, want) {
			t.Errorf("gen --help 输出应包含 %q，实际输出:\n%s", want, out)
		}
	}
}

// TestGenRejectsRemovedWithoutApiFlag 覆盖 F5 提到的旧 flag 已删场景：
// v0.4.0 把 --without-api 反转为 --with-api，旧 flag 名不应再被识别。
func TestGenRejectsRemovedWithoutApiFlag(t *testing.T) {
	resetGenFlags()
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"gen", "--tables", "t", "--without-api"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("gen --tables t --without-api 应返回 error（旧 flag 已删），实际返回 nil")
	}
	if !strings.Contains(err.Error(), "unknown flag") {
		t.Errorf("错误信息应含 \"unknown flag\"，实际: %v", err)
	}
}

// TestGenWithApiTrueRejectedAsPositionalArg 覆盖 F3：bool flag --with-api 不接值，
// `--with-api true` 里的 "true" 会被 cobra 当成位置参数——genCmd.Args=cobra.NoArgs
// 应拒绝它，而不是静默吞掉，让用户误以为传参生效了。
func TestGenWithApiTrueRejectedAsPositionalArg(t *testing.T) {
	resetGenFlags()
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"gen", "--tables", "t", "--with-api", "true"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal(`gen --tables t --with-api true 应返回 error（NoArgs 拒绝多余位置参数 "true"），实际返回 nil`)
	}
	msg := err.Error()
	if !strings.Contains(msg, `unknown command "true"`) && !strings.Contains(msg, "accepts 0 arg") {
		t.Errorf(`错误信息应含 unknown command "true" 或 accepts 0 arg(s)，实际: %v`, err)
	}
}
