// base-code 是数据库代码生成器的 CLI 入口。
//
// Go 小白知识点：
//   - 可执行程序必须在 package main 中，且必须有 func main()。
//   - main 包的 import 路径无法被其他包导入，只能作为程序入口。
//   - os.Exit(1) 立即终止进程并返回退出码 1（非零）。
//     Unix 约定：0=成功，非零=失败。CI/CD 脚本通过 $? 检查退出码。
//     注意：os.Exit 不执行 defer，因此不在 main 中 defer，而由 cmd 层管理资源。
package main

import (
	"fmt"
	"os"

	"github.com/dhslegen/base-code/cmd"
)

func main() {
	// cmd.Execute() 启动 cobra 命令树，解析 os.Args 并执行对应子命令。
	// 若命令执行失败（RunE 返回 error），cobra 已打印 "Error: ..." 到 stderr，
	// Execute() 也会返回该 error。
	if err := cmd.Execute(); err != nil {
		// 再次将错误信息写到 stderr（格式化为中文，方便用户阅读）。
		// fmt.Fprintln(os.Stderr, ...) vs fmt.Println(...)：
		//   - fmt.Println 写 stdout（标准输出，用于正常程序输出）
		//   - fmt.Fprintln(os.Stderr, ...) 写 stderr（标准错误，用于错误/诊断信息）
		//   脚本/管道通常把 stdout 和 stderr 分开处理，错误应写 stderr。
		fmt.Fprintln(os.Stderr, "错误:", err)
		// os.Exit(1) 以非零退出码退出。
		// 非零退出码的价值：
		//   - Shell 脚本：`base-code gen ... || exit 1`（出错时终止后续命令）
		//   - CI/CD：构建系统检测到非零退出码，自动标记构建失败
		//   - AI Agent：可读取退出码判断命令是否成功
		os.Exit(1)
	}
}
