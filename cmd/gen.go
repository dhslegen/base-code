package cmd

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql" // 空白导入：仅为注册 mysql 驱动到 database/sql，不直接调用其 API
	// Go 小白知识点：_ 是空白标识符（blank identifier）。
	// `_ "pkg"` 表示「导入该包，但不使用其导出符号」。
	// database/sql 通过 init() 注册驱动机制工作：go-sql-driver/mysql 的 init() 会调用
	// sql.Register("mysql", &MySQLDriver{})，这样 sql.Open("mysql", dsn) 就能找到驱动。
	// 若不导入，sql.Open 会报 "unknown driver mysql"。
	"github.com/spf13/cobra"

	"github.com/dahaoshen/base-code-go/internal/config"
	"github.com/dahaoshen/base-code-go/internal/dialect"
	"github.com/dahaoshen/base-code-go/internal/generator"
	"github.com/dahaoshen/base-code-go/internal/scanner"
)

// flag 变量：cobra 会把命令行 --config=xxx 等解析结果写入这些变量。
// 包级变量（package-level var）在命令执行前已由 init() 绑定到对应 flag。
var (
	flagConfig          string // --config：配置文件路径
	flagTables          string // --tables：逗号分隔的表名（必填）
	flagDialect         string // --dialect：覆盖配置文件中的方言
	flagDryRun          bool   // --dry-run：只打印到终端，不落盘
	flagOnlyTableModify bool   // --only-table-modify：仅生成改表影响的层（po/req-dto/resp-dto/mapper-xml/query/query-req-dto）
	flagWithoutApi      bool   // --without-api：不生成 API 相关层，保留后端内部层（service/service-impl/po/query/mapper/mapper-xml）
)

// genCmd 是 `base-code gen` 子命令。
//
// cobra.Command 的 RunE 字段（Run with Error）是「有返回值的执行函数」：
//   - 返回 nil   → 命令成功，cobra 正常退出（exit 0）
//   - 返回 error → cobra 打印错误并以非零退出码退出（exit 1）
//
// 和 Run（不返回 error）相比，RunE 让错误传播更自然——遵循 Go 惯用的「error as value」风格。
var genCmd = &cobra.Command{
	Use:   "gen",
	Short: "生成代码",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. 加载配置文件
		cfg, err := config.Load(flagConfig)
		if err != nil {
			return err
		}
		// 2. --dialect flag 覆盖配置文件中的方言（让 CI 流水线可以通过 flag 切换数据库）
		if flagDialect != "" {
			cfg.Datasource.Dialect = flagDialect
		}
		// 3. 注入当前日期到 generator，用于生成文件的 @since 注释
		// Go 小白知识点（重点）：Go 的日期格式使用「参考时间」而非占位符：
		//   - Java:   yyyy-MM-dd
		//   - Python: %Y-%m-%d
		//   - Go:     2006-01-02（Go 诞生时刻：Mon Jan 2 15:04:05 MST 2006）
		// 规律：月=01，日=02，时=15，分=04，秒=05，年=06——Go 用真实时间点代替 YYYY/DD 占位符。
		generator.SetSince(time.Now().Format("2006-01-02"))

		// 4. 解析方言
		d, err := dialect.FromValue(cfg.Datasource.Dialect)
		if err != nil {
			return err
		}
		// 5. 打开数据库连接
		// sql.Open 不会立即连接，只构造连接池——实际连接在 ScanTable 首次查询时建立。
		db, err := sql.Open("mysql", dsn(cfg))
		if err != nil {
			return err
		}
		defer db.Close() // 函数返回时关闭连接池（Go 惯用：defer 确保资源释放不遗漏）

		// 6. 获取扫表器
		sc, err := scanner.For(d, db)
		if err != nil {
			return err
		}

		// 7. 遍历表，逐表生成代码层。
		// M2-B-2：默认生成全 14 层；--without-api / --only-table-modify 按交集过滤
		// Go 小白知识点：SelectLayers 已在 generator 包导出，这里直接调用——
		// 两个 flag 默认均为 false，此时 SelectLayers(false,false) 返回全 14 层，
		// 完全替代原来硬编码的 7 层切片，保持向后兼容同时支持新过滤模式。
		layers := generator.SelectLayers(flagOnlyTableModify, flagWithoutApi)
		for _, t := range splitTables(flagTables) {
			meta, err := sc.ScanTable(t)
			if err != nil {
				return err
			}
			// dryRun=true → 代码写入 os.Stdout，不落盘；dryRun=false → 落盘
			if err := generator.Generate(cfg, meta, layers, flagDryRun, os.Stdout); err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	// StringVar / BoolVar 把命令行 flag 绑定到包级变量。
	// 参数：目标变量指针、flag 名称、默认值、帮助说明
	genCmd.Flags().StringVar(&flagConfig, "config", "base-code.yaml", "配置文件路径")
	genCmd.Flags().StringVar(&flagTables, "tables", "", "逗号分隔的表名（必填）")
	genCmd.Flags().StringVar(&flagDialect, "dialect", "", "覆盖配置中的方言")
	genCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "只打印不落盘")
	genCmd.Flags().BoolVar(&flagOnlyTableModify, "only-table-modify", false, "仅生成改表影响的层")
	genCmd.Flags().BoolVar(&flagWithoutApi, "without-api", false, "不生成 API 相关层")

	// MarkFlagRequired 标记 --tables 为必填。
	// 若用户未提供 --tables，cobra 在 RunE 调用前就打印错误并退出（不进入 RunE）。
	// _ = 忽略返回值：MarkFlagRequired 只在 flag 名不存在时才返回 error，此处 flag 刚注册故不会出错。
	_ = genCmd.MarkFlagRequired("tables")
}

// dsn 拼接 MySQL DSN（Data Source Name）连接串。
//
// 格式：username:password@tcp(host:port)/database?params
// Go 的 database/sql 不规定 DSN 格式，由各驱动自己定义；go-sql-driver/mysql 用此格式。
func dsn(c config.Config) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true",
		c.Datasource.Username, c.Datasource.Password, c.Datasource.Host, c.Datasource.Port, c.Datasource.Database)
}

// splitTables 将逗号分隔的表名字符串拆分为切片，并过滤空白项。
//
// 例：" sys_user , sys_role " → ["sys_user", "sys_role"]
//
// Go 小白知识点：
//   - strings.Split 不会自动去掉空白，需配合 strings.TrimSpace。
//   - var out []string 声明 nil 切片；append 到 nil 切片合法（Go 自动分配底层数组）。
func splitTables(s string) []string {
	var out []string
	for _, t := range strings.Split(s, ",") {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
	}
	return out
}
