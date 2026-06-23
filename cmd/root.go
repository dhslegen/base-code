// Package cmd 用 cobra 组织命令行。
//
// Go 小白知识点：
//   - cobra 是 Go 生态最流行的命令行库（kubectl、hugo、gh 都用它）。
//   - cobra.Command 代表一条命令；Use 是命令名称，Short 是单行简介，Long 是 --help 全文。
//   - rootCmd 是「根命令」，所有子命令（如 gen）都挂在它下面，形成命令树。
//   - init() 是 Go 的包初始化函数，在 main 运行前自动调用，不可手动调用。
//     每个 .go 文件可以有自己的 init()；同一包内多个 init() 按文件名字母序执行。
package cmd

import "github.com/spf13/cobra"

// rootCmd 是顶层命令 base-code。
// 当用户输入 `base-code gen ...` 时，cobra 先命中 rootCmd，再路由到 genCmd。
var rootCmd = &cobra.Command{
	Use:   "base-code",
	Short: "数据库代码生成器（Go 版）",
	Long:  "连接数据库扫描表结构，按约定生成 MyBatis-Plus 分层 Java 代码。",
	// SilenceErrors=true：让 cobra 不自行打印 error（改由 main 统一打印一次中文"错误: ..."，避免重复）。
	SilenceErrors: true,
}

// Execute 由 main 调用，启动整个命令树的解析与执行。
// cobra.Command.Execute() 内部会：
//  1. 解析 os.Args（命令行参数）
//  2. 找到对应子命令
//  3. 绑定 flags
//  4. 调用 RunE / Run 回调
func Execute() error { return rootCmd.Execute() }

func init() {
	// 把 gen 子命令注册到根命令，让 `base-code gen` 可用。
	// genCmd 定义在 gen.go（同一包 cmd），Go 包内跨文件可以直接引用。
	rootCmd.AddCommand(genCmd)
}
