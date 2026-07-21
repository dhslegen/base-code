package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// flagInitForce 对应 --force：覆盖已存在的 base-code.yaml。
var flagInitForce bool

// configTemplate 是 `base-code init` 写出的教学型全量配置模版。
// 必填三项给可运行示例值，其余全部注释并标注真实约定默认值——用户「看着改」。
// 与 README「最小配置文件」样例同源同风格（README 侧已声明同源关系）。
//
// Go 小白知识点：反引号包裹的是「原始字符串字面量」（raw string literal），
// 内部不转义（\n 就是两个字符、换行原样保留），最适合内嵌多行模版文本。
// 注意原始字符串里不能出现反引号本身。
const configTemplate = `# base-code 配置文件 —— 由 base-code init 生成
# 必填三项（tables / base-package / datasource.database）为示例值，请按工程实际修改；
# 其余均为可选项：保持注释即走约定默认值，需要定制时取消注释修改。
# 同名 flag 优先于本文件（如 --tables 会整体覆盖 tables）。
base-code:
  # ── 必填（flag 与配置文件二选一；此处填好后可零 flag 执行：base-code gen）──
  tables: [sys_user, sys_role]            # 生成目标表；也可 --tables sys_user,sys_role 内联
  base-package: com.example.demo          # Java 基础包名

  # with-api: true                        # 默认不生成 API 层；配 true 生成全 14 层（api/api-impl + DTO/converter）

  # ── 可选（缺省即约定默认值）──
  # output-root: ./src/main/java          # Java 源文件输出根目录
  # resources-root: ./src/main/resources  # mapper-xml 输出根；缺省由 output-root 推导
  # author: yourname                      # 代码 @author；缺省读 git config user.name
  # use-jakarta: true                     # true=jakarta 包（Spring Boot 3+），false=javax 包
  # date-type: modern                     # modern=java.time.*，legacy=java.util.Date

  # API 层服务标识（仅 with-api: true 时生效；缺省从 base-package 末段派生）
  # api:
  #   service-name: demo-service          # @FeignClient 的服务名（注册中心应用名）
  #   base-path: /admin-api/demo          # 所有 API 端点的基础路径前缀

  datasource:
    database: demo                        # 数据库名（必填）
    # dialect: mysql                      # mysql 或 postgresql；缺省 mysql
    # host: 127.0.0.1                     # 缺省 127.0.0.1
    # port: 3306                          # 缺省按方言派生：mysql→3306，postgresql→5432
    # username: root                      # 缺省 root
    # password: secret                    # 缺省空；建议改用 --db-password 内联传入，避免密码落盘

  # 自动填充列（@TableField(fill=...)），缺省约定如下
  # auto-fill:
  #   insert-columns: [created_at, updated_at, created_by, updated_by]
  #   update-columns: [updated_at, updated_by]
`

// initCmd 是 `base-code init` 子命令：在当前目录生成 base-code.yaml 配置模版。
// 设计动机：从零手写配置门槛高，「改」比「写」容易——给一份必填项带示例值、
// 可选项全注释的教学型模版，init → 编辑 → gen 三步上手。
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "在当前目录生成 base-code.yaml 配置模版",
	Long: `在当前目录生成教学型 base-code.yaml 配置模版。

模版中必填三项（tables / base-package / datasource.database）已填示例值，
其余配置项以注释形式列出并标注约定默认值——按需取消注释修改即可。
编辑完成后直接运行 base-code gen（无需 --config，gen 默认读取 ./base-code.yaml）。`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		const target = "base-code.yaml"
		// os.Stat 探测文件是否已存在：err == nil 即存在。
		// 防误覆盖是默认行为，覆盖必须显式 --force——不可逆操作要用户明示。
		if _, err := os.Stat(target); err == nil && !flagInitForce {
			return fmt.Errorf("%s 已存在，未做任何改动；确认要覆盖请加 --force", target)
		}
		if err := os.WriteFile(target, []byte(configTemplate), 0o644); err != nil {
			return fmt.Errorf("写入 %s 失败: %w", target, err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), `已生成 %s
下一步：
  1. 编辑必填三项：tables / base-package / datasource.database
  2. 运行 base-code gen
`, target)
		return nil
	},
}

func init() {
	initCmd.Flags().BoolVar(&flagInitForce, "force", false, "覆盖已存在的 base-code.yaml（默认: false）")
}
