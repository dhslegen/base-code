# base-code-go 全流水线教学文档

> 本文档面向 **Go 小白**（有 Java 后端经验）。按代码生成的**实际执行顺序**逐站串讲整个代码库，每站附「Java 对照」与「Go 小白知识点」。

---

## 1. 总览：生成流水线一图

```
用户执行 base-code gen --config base-code.yaml --tables sys_user
          │
          ▼
    ┌────────────┐
    │  config    │  加载 base-code.yaml，填充约定默认值
    └─────┬──────┘
          │
          ▼
    ┌────────────┐
    │  dialect   │  "mysql" / "postgresql" → SqlDialect 常量
    └─────┬──────┘
          │
          ▼
    ┌────────────┐
    │  scanner   │  database/sql 执行 SQL，扫描列 / 主键 / 表注释
    └─────┬──────┘
          │ TableMetadata（原始数据库视角）
          ▼
    ┌────────────┐
    │  typemap   │  dbType → JavaType / JdbcType（接口隐式实现）
    └─────┬──────┘
          │
          ▼
    ┌────────────┐
    │  naming    │  snake_case → 驼峰 / kebab 转换
    └─────┬──────┘
          │
          ▼
    ┌────────────┐
    │  model     │  FindSinglePrimaryKey：无主键/复合主键快速失败
    └─────┬──────┘
          │ TemplateData（Java 视角）
          ▼
    ┌────────────────────────────────────────┐
    │  generator                             │
    │  BuildTemplateData → Render（embed.FS  │
    │  + text/template + FuncMap）→ Generate │
    │  → OutputPath → 落盘 / dry-run         │
    └────────────────────────────────────────┘
          │ *.java / *.xml  ×  14 层
          ▼
       目标工程 src/main/java（或 resources）
```

---

## 2. 逐站讲解

### 站 1 — config：YAML 加载 + 约定默认值

**它做什么**：读取 `base-code.yaml`，解析为 `Config` 结构体，对缺省字段填入约定值，确保后续阶段不需要做 nil 判断。

**关键文件**：`internal/config/config.go`

```go
// internal/config/config.go:14-23
type Config struct {
    BasePackage   string     `yaml:"base-package"`
    OutputRoot    string     `yaml:"output-root"`
    ResourcesRoot string     `yaml:"resources-root"` // 可选；mapper-xml 输出根
    Author        string     `yaml:"author"`
    UseJakarta    *bool      `yaml:"use-jakarta"` // 指针：区分「未配置」与「配置为 false」
    DateType      string     `yaml:"date-type"`
    Datasource    Datasource `yaml:"datasource"`
    AutoFill      AutoFill   `yaml:"auto-fill"`
}
```

```go
// internal/config/config.go:47-62  Load 函数
func Load(path string) (Config, error) {
    data, err := os.ReadFile(path)
    ...
    var r root
    if err := yaml.Unmarshal(data, &r); err != nil { ... }
    cfg := r.BaseCode
    applyDefaults(&cfg)
    if cfg.BasePackage == "" {
        return Config{}, fmt.Errorf("base-package 为必填项")
    }
    return cfg, nil
}
```

**约定默认值**（`applyDefaults`）：
- `UseJakarta`：nil → `true`（Spring Boot 3+ 默认 jakarta）
- `DateType`：空 → `"modern"`（使用 `java.time.*`）
- `Author`：空 → `gitUserName()`（读 git config）
- `AutoFill.InsertColumns`：空 → `["created_at","updated_at","created_by","updated_by"]`
- `AutoFill.UpdateColumns`：空 → `["updated_at","updated_by"]`

> **Java 对照**
>
> Java 版通过 Spring Boot `@ConfigurationProperties` + `application.yml` 加载配置，由 Spring 容器注入。Go 版没有 IoC 框架，手动用 `gopkg.in/yaml.v3` 解析 YAML 并调用 `applyDefaults` 填充默认值，等价于 Java 的 `@PostConstruct` 初始化逻辑。
>
> Java `Optional<T>` 区分「无值」与「false」；Go 用 `*bool` 指针——`nil` 表示未配置，非 nil 表示明确设置（包括 `false`）。

> **Go 小白知识点**
>
> - 结构体标签 `` `yaml:"base-package"` `` 告诉 yaml 库把 YAML 键 `base-package` 映射到 `BasePackage` 字段。这类标签是反引号包裹的字符串，在运行时通过反射读取。
> - `*bool` 是指向 bool 的指针。Go 无原生「三态 bool」，用指针模拟：`nil` 表示缺省，`&true/&false` 表示显式设置。

---

### 站 2 — dialect：具名类型 + 常量

**它做什么**：把配置文件里的字符串方言值（`"mysql"` / `"postgresql"`）转为类型安全的常量，防止后续代码直接比较裸字符串。

**关键文件**：`internal/dialect/dialect.go`

```go
// internal/dialect/dialect.go:8-15
type SqlDialect string   // 具名字符串类型（named type），底层是 string

const (
    MySQL      SqlDialect = "mysql"
    PostgreSQL SqlDialect = "postgresql"
)

// FromValue 解析配置字符串为方言，未知值返回 error
func FromValue(s string) (SqlDialect, error) { ... }
```

> **Java 对照**
>
> Java 版用枚举：
> ```java
> // com.wanji.software.basecode.enums.SqlDialect
> public enum SqlDialect {
>     MYSQL("mysql", "MySQL数据库"),
>     POSTGRESQL("postgresql", "PostgreSQL数据库");
>     
>     public static SqlDialect fromValue(String value) { ... }
> }
> ```
> Go 没有 `enum` 关键字，惯用模式是 `type X string` + `const` 块。与 Java 枚举相比，Go 方案同样类型安全，但不能像枚举那样穷举 switch（编译器不强制）。

> **Go 小白知识点**
>
> `type SqlDialect string` 是「具名类型」（named type）而非类型别名（`type X = string`）。具名类型与底层类型不能直接赋值，需显式转换：`SqlDialect("mysql")` 合法，但 `var d SqlDialect = "mysql"` 需要字面量兼容。这为编译期类型安全提供保障。

---

### 站 3 — scanner：database/sql 扫表

**它做什么**：连接数据库，执行 SQL 读取表的列信息、主键、表注释，返回 `model.TableMetadata`（数据库原始视角）。

**关键文件**：
- `internal/scanner/scanner.go`：`TableScanner` 接口 + `For` 工厂函数
- `internal/scanner/mysql.go`：MySQL 实现（`mySQLScanner`）
- `internal/scanner/postgresql.go`：PostgreSQL 实现（`postgreSQLScanner`）

#### 接口定义

```go
// internal/scanner/scanner.go:25-28
type TableScanner interface {
    ScanTable(table string) (model.TableMetadata, error)
}

func For(d dialect.SqlDialect, db *sql.DB) (TableScanner, error) { ... }
```

#### MySQL 扫表（两段查询）

```go
// internal/scanner/mysql.go:47-48  第一段：SHOW FULL COLUMNS
rows, err := s.db.Query(fmt.Sprintf("SHOW FULL COLUMNS FROM `%s`", table))
// SHOW FULL COLUMNS 9 列：Field, Type, Collation, Null, Key, Default, Extra, Privileges, Comment

// internal/scanner/mysql.go:86-88  第二段：查表注释
_ = s.db.QueryRow(
    "SELECT table_comment FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ? LIMIT 1", table,
).Scan(&tableComment)
```

#### PostgreSQL 扫表（三段查询）

PostgreSQL 把列信息、约束信息、对象注释分散在不同系统表里，需要三段独立查询：

```go
// internal/scanner/postgresql.go:42-47  第一段：列查询（含列注释）
const pgColumnSQL = `SELECT column_name, data_type, ...,
    COALESCE(col_description(pgc.oid, pa.attnum), '') as column_comment
FROM information_schema.columns c
LEFT JOIN pg_class pgc ON pgc.relname = c.table_name
...WHERE c.table_name = $1 AND c.table_schema = 'public'`

// 第二段：主键查询（information_schema.table_constraints）
// 第三段：表注释查询（obj_description）
```

> **Java 对照**
>
> Java 版用 `DatabaseTableScanner` 接口（`interface DatabaseTableScanner`），MySQL 实现为 `MysqlTableScanner`，PostgreSQL 实现为 `PostgreSqlTableScanner`，通过 `TableScannerFactory` 工厂按方言分发（对应 Go 的 `scanner.For`）。Java 使用 `ResultSet`（JDBC 游标），Go 使用 `*sql.Rows`（database/sql 游标），模式完全对称。

> **Go 小白知识点**
>
> - **隐式接口**：Go 接口是「隐式实现」——`mySQLScanner` 有 `ScanTable` 方法就自动满足 `TableScanner` 接口，无需 `implements` 关键字。
> - **`rows.Next()` 遍历**：`db.Query()` 返回 `*sql.Rows`（游标），必须调用 `defer rows.Close()` 归还连接；`rows.Next()` 移动游标，`rows.Scan(&v1, &v2...)` 读取当前行，必须传指针。
> - **MySQL `?` vs PostgreSQL `$1`**：MySQL 驱动用 `?` 占位符，pgx 驱动（`pgx/v5/stdlib`）用 `$1, $2...` 有序编号占位符。
> - **`sql.NullString` / `sql.NullInt64`**：接收数据库 NULL 值的标准类型，`Valid=false` 表示 NULL，`Valid=true` 时字段有意义。
> - **`go-sqlmock` 单元测试**：`github.com/DATA-DOG/go-sqlmock` 注册一个假 `sql.Driver`，无需启动真实数据库即可测试 `ScanTable`——预设期望 SQL 与返回行，断言结果（见 `internal/scanner/mysql_test.go` / `postgresql_test.go`）。类比 Java 的 Mockito mock `ResultSet` 或 H2 内存库。

---

### 站 4 — typemap：隐式接口 + FQN 免 import

**它做什么**：把数据库列类型字符串（如 `"bigint"`）映射为 Java 类型名（`"Long"`）和 JDBC 类型名（`"BIGINT"`），供模板渲染使用。

**关键文件**：
- `internal/typemap/typemap.go`：`TypeMapper` 接口 + `For` 工厂
- `internal/typemap/mysql.go`：MySQL 实现，含 `modern` / `legacy` 日期类型分支
- `internal/typemap/postgresql.go`：PostgreSQL 实现

```go
// internal/typemap/typemap.go:14-19
type TypeMapper interface {
    MapToJavaType(dbType string) string
    MapToJdbcType(dbType string) string
}

func For(d dialect.SqlDialect, dateType string) (TypeMapper, error) { ... }
```

**dateType 分支**（`mysql.go:60-68`）：
- `"modern"`（默认）：`date` → `java.time.LocalDate`，`datetime/timestamp` → `java.time.LocalDateTime`
- `"legacy"`：`date` → `java.util.Date`，`datetime/timestamp` → `java.util.Date`

**FQN 免 import**：非 `java.lang` 类型使用全限定名（Full Qualified Name），模板直接展开无需额外 import：
- `decimal` → `java.math.BigDecimal`（FQN）
- `varchar` → `String`（`java.lang`，自动引入）

> **Java 对照**
>
> Java 版：`TypeMapper` 接口定义在 `com.wanji.software.basecode.mapper.TypeMapper`，方法签名为 `String mapToJavaType(String dbType)` 和 `JDBCType mapToJdbcType(String dbType)`（注意 Java 版 `mapToJdbcType` 返回 `java.sql.JDBCType` 枚举，Go 版返回字符串）。实现类为 `MySqlTypeMapper` 和 `PostgreSqlTypeMapper`，由 `TypeMapperFactory` 按方言分发。

> **Go 小白知识点**
>
> Go 接口通过「结构类型系统」（structural typing）实现：只要结构体有接口声明的全部方法签名，就自动满足接口，无需显式声明。这与 Java 的 `implements` 关键字（名义类型系统）完全不同，带来极大的灵活性——可以为任何已有类型补充接口实现，甚至为第三方库类型「伪实现」接口。

---

### 站 5 — naming：驼峰 / kebab 转换

**它做什么**：把数据库下划线命名（`sys_user`）转成 Java 驼峰命名（`sysUser` / `SysUser`）以及 URL 路径用的 kebab 命名（`sys-user`）。

**关键文件**：`internal/naming/naming.go`

```go
// internal/naming/naming.go:7-11  导出的命名转换函数
func Camel(s string) string      { return camel(s, false) }  // sys_user → sysUser
func UpperCamel(s string) string { return camel(s, true) }   // sys_user → SysUser
func Kebab(s string) string      { ... }                      // SysUser → sys-user（仅接受大驼峰输入）
func Capitalize(s string) string { ... }                      // userName → UserName（仅首字母）
```

`camel` 内部实现：遍历 rune，遇 `_` 或 `-` 则将下一字符大写；`upperFirst` 控制首字符。

> **Java 对照**
>
> Java 版在 `BaseCodeApplication` 中以私有实例方法实现（`toCamelCase(String input)`、`toUpperCamelCase(String input)`、`toKebabCase(String input)`、`capitalize(String input)`），散落在业务逻辑类里。Go 版提取为独立的 `naming` 包，函数首字母大写（导出），便于独立测试和跨包复用。

> **Go 小白知识点**
>
> - `range s`（s 为 string）按 **Unicode 码点（rune）**遍历，而非字节。这对中文等多字节字符安全。
> - Go 没有字符类型 `char`，字符用 `rune`（`int32` 的别名）表示。
> - `strings.Builder` 是 Go 1.10+ 高效字符串拼接工具，避免每次 `+=` 产生临时字符串（对应 Java `StringBuilder`）。
> - **表驱动测试**（table-driven test）：Go 惯用模式，用 `[]struct{input, want}` 切片集中组织测试用例，通过 `for range` 循环调用 `t.Run`，见 `internal/naming/naming_test.go`。

---

### 站 6 — model：结构体 + 单主键 fail-fast

**它做什么**：定义三个核心数据结构（数据库视角 `ColumnMetadata`、`TableMetadata`；Java 视角 `FieldMetadata`）以及单主键校验函数。

**关键文件**：`internal/model/model.go`

```go
// internal/model/model.go:10-15  数据库视角
type ColumnMetadata struct {
    ColumnName    string
    ColumnType    string
    ColumnComment string
    IsPrimaryKey  bool
}

// internal/model/model.go:19-27  Java 视角
type FieldMetadata struct {
    JavaType     string
    JdbcType     string
    Name         string   // 小驼峰字段名，如 userName
    TableField   string   // 原始列名，如 user_name
    Comment      string
    AutoFill     string   // "" / "insert" / "update" / "insertUpdate"
    IsPrimaryKey bool
}

// internal/model/model.go:38-56  单主键校验（快速失败）
func FindSinglePrimaryKey(fields []FieldMetadata, tableName string) (FieldMetadata, error) {
    // 无主键 → error；复合主键 → error；恰好一列 → 返回该列
}
```

> **Java 对照**
>
> Java 版：`ColumnMetadata`、`TableMetadata`、`FieldMetadata` 均位于 `com.wanji.software.basecode.model` 包，用 Lombok `@Data` 生成 getter/setter。Go 版结构体直接访问导出字段，无需 getter/setter（Go 惯用「扁平结构」，不封装简单数据载体）。
>
> Java 版主键校验抛 `IllegalStateException`，Go 版 `FindSinglePrimaryKey` 返回 `(FieldMetadata, error)`，调用方显式检查——这是 Go「error as value」哲学，错误是普通返回值，不是异常。

> **Go 小白知识点**
>
> - Go 结构体字段首字母**大写 = 导出**（`public`），小写 = 包私有（`package-private`）。`text/template` 通过反射访问字段，**只能访问导出字段**——这是为什么 `TemplateData` 和 `FieldMetadata` 的每个字段都首字母大写。
> - Go 没有 `throws` 声明，错误通过多返回值 `(T, error)` 传播，调用方必须在调用点检查 `error`（`go vet` 会警告忽略错误）。

---

### 站 7 — generator：embed.FS + text/template + 编排器

**它做什么**：这是流水线的核心枢纽，分为三个文件：
- `templates.go`：`//go:embed` 把 14 个 `.tmpl` 文件编进二进制
- `render.go`：`Render` 函数执行模板渲染，返回代码字符串
- `generate.go`：`BuildTemplateData`（组装上下文）、`Generate`（编排 + 落盘）、`Layers`（层定义表）、`SelectLayers`（层过滤）

#### 模板嵌入（templates.go）

```go
// internal/generator/templates.go:11-12
//go:embed all:templates
var templateFS embed.FS
```

#### 模板渲染（render.go）

```go
// internal/generator/render.go:60-74
func Render(layer string, data TemplateData) (string, error) {
    name := layer + ".tmpl"
    tmpl, err := template.New(name).Funcs(funcMap).
        ParseFS(templateFS, "templates/"+name)
    ...
    var buf bytes.Buffer
    tmpl.Execute(&buf, data)
    return buf.String(), nil
}

// FuncMap 注入命名转换函数到模板
var funcMap = template.FuncMap{
    "camel": naming.Camel, "upperCamel": naming.UpperCamel,
    "kebab": naming.Kebab, "capitalize": naming.Capitalize,
}
```

模板片段示例（`po.tmpl` 使用 `range .Fields`）：

```
{{range .Fields}}
    @TableField(value = "{{.TableField}}")
    private {{.JavaType}} {{.Name}};
{{end}}
    {{if $.UseJakarta}}@Serial{{end}}
```

（此处省略 `fill = FieldFill.INSERT/UPDATE/INSERT_UPDATE` 的 autoFill 条件分支，完整见 `templates/po.tmpl`）

> `range .Fields` 内部 `.` 指向当前 `FieldMetadata`；要访问父级上下文（`TemplateData`）字段，需用 `$.UseJakarta` 而非 `.UseJakarta`。

#### 层定义表（Layers）

```go
// internal/generator/generate.go:57-72
var Layers = map[string]LayerSpec{
    "po":           {PkgSuffix: "model.po",  NameSuffix: "",           Ext: ".java"},
    "mapper":       {PkgSuffix: "mapper",    NameSuffix: "Mapper",     Ext: ".java"},
    "service":      {PkgSuffix: "service",   NameSuffix: "Service",    Ext: ".java"},
    "service-impl": {PkgSuffix: "service.impl", NameSuffix: "ServiceImpl", Ext: ".java"},
    "query":        {PkgSuffix: "model.query", NameSuffix: "Query",    Ext: ".java"},
    "converter":    {PkgSuffix: "converter", NameSuffix: "Converter",  Ext: ".java"},
    "mapper-xml":   {PkgSuffix: "mapper",    NameSuffix: "Mapper",     Ext: ".xml", Resource: true},
    // ... 7 个 DTO/API 层
}
```

`LayerSpec.Resource=true` 表示 XML 文件落 `resources/mapper/` 而非 `src/main/java`。

#### 层过滤（SelectLayers）

```go
// internal/generator/generate.go:313-326
func SelectLayers(onlyTableModify, withoutApi bool) []string {
    // 从 AllLayers()（全 14 层稳定顺序）按两个开关过滤：
    // onlyTableModify=true → 仅保留 po/req-dto/resp-dto/mapper-xml/query/query-req-dto
    // withoutApi=true      → 仅保留 service/service-impl/po/query/mapper/mapper-xml
    // 两者同时 true        → 取交集（po/query/mapper-xml）
}
```

#### BuildTemplateData + idType 动态推导

```go
// internal/generator/generate.go:143-211
func BuildTemplateData(meta model.TableMetadata, cfg config.Config) (TemplateData, error) {
    // 1. 解析方言 → 2. 获取 TypeMapper → 3. 遍历列构建 FieldMetadata
    // 4. FindSinglePrimaryKey（快速失败）
    // 5. IdType = pk.JavaType（从主键列类型动态推导，如 bigint→Long）
    // 6. PkFieldUpperCamel = naming.Capitalize(pk.Name)（如 id→Id）
}
```

> **Java 对照**
>
> Java 版（`BaseCodeApplication`）使用 **Thymeleaf**（`org.thymeleaf.TemplateEngine`）作为模板引擎，模板以 `.tmpl` 后缀存放在 `resources/templates/` 下，Spring Boot 打包后通过 ClassPath 加载。模板语法：变量用 `[[${basePackageName}]]`，循环用 `[# th:each="field : ${fields}"]...[/]`，条件用 `[# th:if="${...}"]...[/]`。Go 版用 `//go:embed` 将 `.tmpl` 文件编译进二进制（单文件分发），模板引擎换为标准库 `text/template`，语法不同但设计意图完全对应。

> **Go 小白知识点**
>
> - **`//go:embed`**：编译指令，必须紧贴 `var` 声明（中间不能有空行）。`all:templates` 嵌入 `templates/` 目录下全部文件（包含隐藏文件）。
> - **`text/template`**：Go 标准库模板引擎，`{{.Field}}` 访问结构体字段，`{{range .Slice}}...{{end}}` 遍历切片，`{{if .Bool}}...{{end}}` 条件渲染。对应 Java Thymeleaf 的 `[[${field}]]` / `[# th:each ...]`，但两者语法不同——Go 用双花括号，Thymeleaf 用双方括号或属性标签。
> - **`io.Writer` 接口**：`Generate` 的 `out io.Writer` 参数。只要实现 `Write([]byte) (int, error)`，就是 `Writer`。测试传 `&bytes.Buffer{}`，CLI 传 `os.Stdout`，代码一字不改——这是「依赖接口，不依赖实现」的典型体现。
> - **`map[string]bool` 当集合**：Go 没有 `Set<T>`，惯用 `map[K]bool`，`O(1)` 成员判断（`onlyTableModifySet[layer]`）。

---

### 站 8 — cmd：cobra + 方言感知驱动 / DSN + 退出码

**它做什么**：CLI 入口，组织命令树（`base-code gen`），解析 flag，按方言选驱动和 DSN，调用前面所有站的函数完成流水线。

**关键文件**：
- `cmd/root.go`：`rootCmd`（`base-code`）+ `Execute()`
- `cmd/gen.go`：`genCmd`（`base-code gen`）+ flag 绑定 + 流水线调用

#### Flag 定义

```go
// cmd/gen.go:30-37
var (
    flagConfig          string // --config
    flagTables          string // --tables（必填）
    flagDialect         string // --dialect
    flagDryRun          bool   // --dry-run
    flagOnlyTableModify bool   // --only-table-modify
    flagWithoutApi      bool   // --without-api
)
```

#### 驱动 + DSN 选择（方言感知）

```go
// cmd/gen.go:134-148
func dbDriverAndDSN(d dialect.SqlDialect, cfg config.Config) (string, string) {
    switch d {
    case dialect.PostgreSQL:
        return "pgx", fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", ...)
    default: // MySQL
        return "mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true", ...)
    }
}
```

MySQL 和 PostgreSQL 驱动通过**空白导入**注册：

```go
// cmd/gen.go:9-16
import (
    _ "github.com/go-sql-driver/mysql"   // 注册 "mysql" 驱动
    _ "github.com/jackc/pgx/v5/stdlib"  // 注册 "pgx" 驱动
)
```

#### 退出码

`rootCmd.SilenceErrors = true`，由 `main.go` 统一处理：若 `Execute()` 返回 error，打印一次错误并以非零退出码退出（`os.Exit(1)`）。`genCmd` 使用 `RunE`（Run with Error），返回 `nil` 则正常退出（exit 0）。

> **Java 对照**
>
> Java 版 `BaseCodeApplication` 是 Spring Boot 应用，通过 `@SpringBootApplication` + `CommandLineRunner` 接收命令行参数，没有显式的命令树结构。参数解析依赖手工 `args[0]`、`args[1]` 拆分。Go 版用 cobra（类似 Java 的 picocli / JCommander），支持多级子命令、自动生成 `--help` 文档、flag 类型安全解析。

> **Go 小白知识点**
>
> - **`_ "pkg"`（空白导入）**：`database/sql` 使用注册机制——驱动包的 `init()` 调用 `sql.Register("mysql", ...)` 注册自身。空白导入触发 `init()` 但不引入任何符号。若不导入，`sql.Open("mysql", dsn)` 报 `unknown driver "mysql"`。
> - **日期格式**：Go 使用「参考时间」而非占位符：`time.Now().Format("2006-01-02")` 等价于 Java 的 `new SimpleDateFormat("yyyy-MM-dd")`。Go 的参考时间是 `Mon Jan 2 15:04:05 MST 2006`（Go 诞生时刻），月=01，日=02，年=06。
> - **`defer`**：函数返回时（包含所有 return 路径）必然执行，是 Go 确保资源释放（文件/连接关闭）的惯用方式，对应 Java 的 `try-finally` 或 `try-with-resources`。

---

## 3. 14 层产物速查表

以下层名与 `internal/generator/generate.go` 中 `Layers` map 的键完全一致，文件名格式为 `{ModelUpperCamel}{NameSuffix}{Ext}`。

| 层名 | 包后缀（PkgSuffix） | 文件名示例（`sys_user` 表） | 扩展名 | 落盘根 |
|------|---------------------|----------------------------|--------|--------|
| `po` | `model.po` | `SysUser.java` | `.java` | java 根 |
| `mapper` | `mapper` | `SysUserMapper.java` | `.java` | java 根 |
| `mapper-xml` | `mapper` | `SysUserMapper.xml` | `.xml` | **resources 根** |
| `service` | `service` | `SysUserService.java` | `.java` | java 根 |
| `service-impl` | `service.impl` | `SysUserServiceImpl.java` | `.java` | java 根 |
| `query` | `model.query` | `SysUserQuery.java` | `.java` | java 根 |
| `converter` | `converter` | `SysUserConverter.java` | `.java` | java 根 |
| `req-dto` | `model.dto.req` | `SysUserReqDto.java` | `.java` | java 根 |
| `resp-dto` | `model.dto.resp` | `SysUserRespDto.java` | `.java` | java 根 |
| `query-req-dto` | `model.dto.req` | `SysUserQueryReqDto.java` | `.java` | java 根 |
| `page-query-req-dto` | `model.dto.req` | `SysUserPageQueryReqDto.java` | `.java` | java 根 |
| `update-by-query-req-dto` | `model.dto.req` | `SysUserUpdateByQueryReqDto.java` | `.java` | java 根 |
| `api` | `api` | `SysUserApi.java` | `.java` | java 根 |
| `api-impl` | `api.impl` | `SysUserApiImpl.java` | `.java` | java 根 |

**层过滤快速参考**（`SelectLayers` 函数）：

| flag 组合 | 输出层 |
|-----------|--------|
| 默认（无 flag） | 全 14 层 |
| `--only-table-modify` | `po` / `req-dto` / `resp-dto` / `mapper-xml` / `query` / `query-req-dto` |
| `--without-api` | `service` / `service-impl` / `po` / `query` / `mapper` / `mapper-xml` |
| 两者同时 | `po` / `query` / `mapper-xml`（取交集） |

---

## 4. 工程级前置依赖

`api` / `api-impl` 层引用以下**目标工程需自行预置**的公共类（base-code 不生成它们）：

- **`{base-package}.constants.ApiConstants`**：提供 `NAME`（Feign 服务名）与 `PREFIX`（API 路径前缀），用于 `@FeignClient` 注解及各端点 URL 拼接。
- **`{base-package}.model.dto.req.PageQueryReqDto`**：非表相关的通用分页请求 DTO（仅含 `current`/`size`），是 `pageAll` 端点的入参。注意：各表生成的 `XxxPageQueryReqDto` 继承自 `XxxQueryReqDto`，与这个通用基类是两回事。

未预置上述类时，`api`/`api-impl` 层产物无法编译（与源工程既有契约一致）。

详情参见 [README.md](../README.md) 的"工程级前置依赖"节。

---

## 5. 进一步

- 使用 `code-learner` 技能可生成「带注释的逐文件 Go 讲解」。
- 使用 `codetour-teacher` 技能可生成「交互式代码导览（CodeTour 格式）」，在 VS Code 中按站点点击高亮代码。
- 阅读 `*_test.go` 文件——每个包都有测试，是了解函数契约的最佳方式。
