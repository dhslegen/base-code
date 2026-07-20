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
	_ "github.com/jackc/pgx/v5/stdlib" // 空白导入：注册 pgx 的 database/sql 兼容驱动，驱动名为 "pgx"。
	// pgx 是 Go 生态最主流的 PostgreSQL 驱动。`pgx/v5/stdlib` 子包封装了原生 pgx 连接池，
	// 使其符合标准库 database/sql 接口（*sql.DB），从而与 sql.Open 无缝配合。
	// 注意：pgx 的原生 API（pgxpool.Pool）性能更高，但这里统一用 database/sql 便于复用扫表逻辑。
	"github.com/spf13/cobra"

	"github.com/dhslegen/base-code/internal/config"
	"github.com/dhslegen/base-code/internal/dialect"
	"github.com/dhslegen/base-code/internal/generator"
	"github.com/dhslegen/base-code/internal/scanner"
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

	// 内联配置 flag：所有 yaml 配置项的命令行等价物（agent 可一行构造完整命令）。
	// 注意：是否「显式提供」由 Flags().Changed 判定，这些变量的零值不承担缺省语义——
	// 缺省统一由 config.applyDefaults 补齐（优先级：flag > 配置文件 > 约定默认值）。
	flagBasePackage    string
	flagOutputRoot     string
	flagResourcesRoot  string
	flagAuthor         string
	flagUseJakarta     bool
	flagDateType       string
	flagDbHost         string
	flagDbPort         int
	flagDbUser         string
	flagDbPassword     string
	flagDbName         string
	flagServiceName    string
	flagBasePath       string
	flagAutoFillInsert string
	flagAutoFillUpdate string
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
		// 1. 装配命令行内联覆盖（仅收集用户显式提供的 flag）。
		// Go 小白知识点：fs.Changed(name) 报告该 flag 是否在命令行出现过——
		// 这与「值 == 零值」不同：--use-jakarta=false / --db-port 0 也算显式提供。
		fs := cmd.Flags()
		ov := config.Overrides{}
		if fs.Changed("base-package") {
			ov.BasePackage = &flagBasePackage
		}
		if fs.Changed("output-root") {
			ov.OutputRoot = &flagOutputRoot
		}
		if fs.Changed("resources-root") {
			ov.ResourcesRoot = &flagResourcesRoot
		}
		if fs.Changed("author") {
			ov.Author = &flagAuthor
		}
		if fs.Changed("use-jakarta") {
			ov.UseJakarta = &flagUseJakarta
		}
		if fs.Changed("date-type") {
			ov.DateType = &flagDateType
		}
		if fs.Changed("dialect") {
			ov.Dialect = &flagDialect
		}
		if fs.Changed("db-host") {
			ov.DbHost = &flagDbHost
		}
		if fs.Changed("db-port") {
			ov.DbPort = &flagDbPort
		}
		if fs.Changed("db-user") {
			ov.DbUser = &flagDbUser
		}
		if fs.Changed("db-password") {
			ov.DbPassword = &flagDbPassword
		}
		if fs.Changed("db-name") {
			ov.DbName = &flagDbName
		}
		if fs.Changed("service-name") {
			ov.ServiceName = &flagServiceName
		}
		if fs.Changed("base-path") {
			ov.BasePath = &flagBasePath
		}
		if fs.Changed("auto-fill-insert") {
			cols := splitCSV(flagAutoFillInsert)
			ov.AutoFillInsert = &cols
		}
		if fs.Changed("auto-fill-update") {
			cols := splitCSV(flagAutoFillUpdate)
			ov.AutoFillUpdate = &cols
		}

		// 2. 加载配置并叠加覆盖。requireFile 语义：用户显式 --config → 文件必须存在；
		// 未显式 → 默认文件缺席时进入纯 flag 模式。
		cfg, err := config.LoadWithOverrides(flagConfig, fs.Changed("config"), ov)
		if err != nil {
			return err
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
		// 5. 按方言选择驱动名与 DSN，再打开数据库连接。
		// sql.Open 不会立即连接，只构造连接池——实际连接在 ScanTable 首次查询时建立。
		// Go 小白知识点：不同数据库的 DSN 格式由各自驱动定义，彼此不通用：
		//   - MySQL  DSN：user:pass@tcp(host:port)/db?charset=utf8mb4&parseTime=true
		//   - PG DSN：  host=… port=… user=… password=… dbname=… sslmode=disable（key=value 风格）
		// 这里用 dbDriverAndDSN 把「选驱动」和「拼 DSN」封装到一起，避免 if-else 散落在业务逻辑里。
		driverName, dataSourceName := dbDriverAndDSN(d, cfg)
		db, err := sql.Open(driverName, dataSourceName)
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
		for _, t := range splitCSV(flagTables) {
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

	// 内联配置 flag（与 base-code.yaml 各键一一对应；提供后覆盖文件值）
	genCmd.Flags().StringVar(&flagBasePackage, "base-package", "", "Java 基础包名（内联，覆盖配置文件）")
	genCmd.Flags().StringVar(&flagOutputRoot, "output-root", "", "Java 源文件输出根目录（内联）")
	genCmd.Flags().StringVar(&flagResourcesRoot, "resources-root", "", "mapper-xml 输出根目录（内联，缺省由 output-root 推导）")
	genCmd.Flags().StringVar(&flagAuthor, "author", "", "代码 @author（内联，缺省读 git config user.name）")
	genCmd.Flags().BoolVar(&flagUseJakarta, "use-jakarta", true, "true=jakarta 包（Spring Boot 3+），false=javax 包（内联）")
	genCmd.Flags().StringVar(&flagDateType, "date-type", "", "modern=java.time.*，legacy=java.util.Date（内联）")
	genCmd.Flags().StringVar(&flagDbHost, "db-host", "", "数据库主机（内联）")
	genCmd.Flags().IntVar(&flagDbPort, "db-port", 0, "数据库端口（内联，缺省按方言 3306/5432）")
	genCmd.Flags().StringVar(&flagDbUser, "db-user", "", "数据库用户名（内联）")
	genCmd.Flags().StringVar(&flagDbPassword, "db-password", "", "数据库密码（内联）")
	genCmd.Flags().StringVar(&flagDbName, "db-name", "", "数据库名（内联）")
	genCmd.Flags().StringVar(&flagServiceName, "service-name", "", "@FeignClient 服务名（内联，缺省 base-package 末段）")
	genCmd.Flags().StringVar(&flagBasePath, "base-path", "", "API 基础路径前缀（内联，缺省 /+base-package 末段）")
	genCmd.Flags().StringVar(&flagAutoFillInsert, "auto-fill-insert", "", "插入自动填充列，逗号分隔（内联）")
	genCmd.Flags().StringVar(&flagAutoFillUpdate, "auto-fill-update", "", "更新自动填充列，逗号分隔（内联）")

	// MarkFlagRequired 标记 --tables 为必填。
	// 若用户未提供 --tables，cobra 在 RunE 调用前就打印错误并退出（不进入 RunE）。
	// _ = 忽略返回值：MarkFlagRequired 只在 flag 名不存在时才返回 error，此处 flag 刚注册故不会出错。
	_ = genCmd.MarkFlagRequired("tables")
}

// dbDriverAndDSN 按方言返回 database/sql 驱动名与连接串（DSN）。
//
// Go 小白知识点：database/sql 把「驱动注册」与「连接串格式」都交给第三方包自行定义。
//   - MySQL  驱动名 "mysql"，DSN 格式：user:pass@tcp(host:port)/db?charset=utf8mb4&parseTime=true
//   - pgx    驱动名 "pgx"，  DSN 格式：host=… port=… user=… password=… dbname=… sslmode=disable
//
// switch 默认分支兜底 MySQL，保持向后兼容；新增方言只需追加 case。
func dbDriverAndDSN(d dialect.SqlDialect, cfg config.Config) (string, string) {
	ds := cfg.Datasource
	switch d {
	case dialect.PostgreSQL:
		// pgx 的 stdlib 驱动名为 "pgx"（由 pgx/v5/stdlib 的 init() 注册）。
		// PostgreSQL DSN 使用 key=value 格式（libpq 风格），各字段以空格分隔。
		// sslmode=disable：本地/内网环境通常不启用 TLS，生产环境可改为 require/verify-full。
		return "pgx", fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			ds.Host, ds.Port, ds.Username, ds.Password, ds.Database)
	default: // mysql（以及未来尚未支持的方言均兜底 mysql）
		// go-sql-driver/mysql 驱动名 "mysql"，DSN 采用 URI 风格：user:pass@tcp(host:port)/db?params。
		// parseTime=true：让驱动把 DATE/DATETIME 列自动扫描为 time.Time（否则需手动解析字符串）。
		return "mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true",
			ds.Username, ds.Password, ds.Host, ds.Port, ds.Database)
	}
}

// splitCSV 将逗号分隔字符串拆分为切片，并过滤空白项（表名、自动填充列清单共用）。
//
// 例：" sys_user , sys_role " → ["sys_user", "sys_role"]
//
// Go 小白知识点：
//   - strings.Split 不会自动去掉空白，需配合 strings.TrimSpace。
//   - var out []string 声明 nil 切片；append 到 nil 切片合法（Go 自动分配底层数组）。
func splitCSV(s string) []string {
	var out []string
	for _, t := range strings.Split(s, ",") {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
	}
	return out
}
