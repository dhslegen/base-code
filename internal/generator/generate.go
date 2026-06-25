// Package generator generate.go 是编排器：把「表元数据 + 配置」组装成 TemplateData，再按约定路径渲染落盘。
// 与 render.go（模板渲染）和 templates.go（模板嵌入）处于同一包 generator 内，共享 TemplateData 定义。
//
// 核心流程：
//
//	TableMetadata + Config
//	    ↓ BuildTemplateData（类型映射、主键推导、autoFill 判定）
//	  TemplateData
//	    ↓ Render（text/template 执行）
//	  Java 代码字符串
//	    ↓ Generate（落盘 or dry-run）
//	  *.java 文件 / io.Writer 输出
//
// Go 小白知识点（全文重点）：
//   - io.Writer 接口：只要实现了 Write(p []byte) (n int, err error) 方法，就是 Writer。
//     os.File、bytes.Buffer、os.Stdout 都是 Writer。Generate 只依赖接口，
//     测试时传 bytes.Buffer，main 传 os.Stdout，代码一字不改——这是"依赖接口"的典型好处。
//   - filepath.Join：跨平台路径拼接（Windows 用 \，Unix 用 /），不要手动拼字符串。
//   - os.MkdirAll：等同于 mkdir -p，目录已存在不报错。
//   - os.WriteFile：覆盖写文件（Go 1.16+，非原子：中途失败可能留下截断文件），自动创建或覆盖。
//   - 包级变量 since：由 main 注入，避免库代码调用 time.Now()——库内取系统时间会让测试非确定性。
package generator

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dahaoshen/base-code-go/internal/config"
	"github.com/dahaoshen/base-code-go/internal/dialect"
	"github.com/dahaoshen/base-code-go/internal/model"
	"github.com/dahaoshen/base-code-go/internal/naming"
	"github.com/dahaoshen/base-code-go/internal/typemap"
)

// LayerSpec 描述一层的输出约定：包后缀、文件名在 ModelUpperCamel 后追加的后缀、扩展名、是否落 resources 根。
//
// Go 小白知识点：结构体（struct）是 Go 中聚合多字段的基本方式，类似 Java 的 POJO。
// 这里用值类型（非指针），因为 LayerSpec 是不可变的配置描述，复制开销极低。
type LayerSpec struct {
	PkgSuffix  string // 包后缀，如 model.po；resource 层用作子目录（mapper → resources/mapper/）
	NameSuffix string // 文件名后缀，如 ServiceImpl；po 层为空（文件名即 ModelUpperCamel）
	Ext        string // 文件扩展名：.java 或 .xml
	Resource   bool   // true 表示落 resources 根（如 mapper-xml），false 落 java 根
}

// Layers 是层 → 输出约定的全局约定表（M2-A 含非 API 的 7 层；M2-B 再补 api/dto）。
//
// 设计原则（开闭原则）：新增层只需在此追加一条 LayerSpec 记录，
// Generate/OutputPath 等编排代码无需任何修改。
//
// Go 小白知识点：var xxx = map[K]V{...} 是包级变量初始化，
// 整个程序生命周期内共享同一份 map。约定上只读；但它是导出的包级 map，
// 语言层面 importer 仍可改写——约定不可变即可，无需额外防护。
var Layers = map[string]LayerSpec{
	"po":           {PkgSuffix: "model.po", NameSuffix: "", Ext: ".java"},           // 实体类，文件名无后缀（如 SysUser.java）
	"mapper":       {PkgSuffix: "mapper", NameSuffix: "Mapper", Ext: ".java"},        // MyBatis-Plus Mapper 接口
	"service":      {PkgSuffix: "service", NameSuffix: "Service", Ext: ".java"},      // Service 接口
	"service-impl": {PkgSuffix: "service.impl", NameSuffix: "ServiceImpl", Ext: ".java"}, // Service 实现类
	"query":        {PkgSuffix: "model.query", NameSuffix: "Query", Ext: ".java"},    // 分页/条件查询对象
	"converter":    {PkgSuffix: "converter", NameSuffix: "Converter", Ext: ".java"},  // DTO ↔ PO 转换器
	"mapper-xml":   {PkgSuffix: "mapper", NameSuffix: "Mapper", Ext: ".xml", Resource: true}, // MyBatis XML，落 resources/mapper/
	"req-dto":                 {PkgSuffix: "model.dto.req", NameSuffix: "ReqDto", Ext: ".java"},                 // 请求 DTO
	"resp-dto":                {PkgSuffix: "model.dto.resp", NameSuffix: "RespDto", Ext: ".java"},               // 响应 DTO
	"query-req-dto":           {PkgSuffix: "model.dto.req", NameSuffix: "QueryReqDto", Ext: ".java"},           // 查询请求 DTO
	"page-query-req-dto":      {PkgSuffix: "model.dto.req", NameSuffix: "PageQueryReqDto", Ext: ".java"},      // 分页查询请求 DTO
	"update-by-query-req-dto": {PkgSuffix: "model.dto.req", NameSuffix: "UpdateByQueryReqDto", Ext: ".java"}, // 按条件更新请求 DTO
	"api":                      {PkgSuffix: "api", NameSuffix: "Api", Ext: ".java"},  // Feign RPC 接口
	"api-impl":                 {PkgSuffix: "api.impl", NameSuffix: "ApiImpl", Ext: ".java"}, // Feign RPC 实现
}

// ResolveResourcesRoot 由 java 源根派生 resources 根：把末段 src/main/java 换成 src/main/resources。
// 若 configured 非空则直接用它（配置优先于约定，便于非标准项目结构覆盖）。
//
// Go 小白知识点：filepath.Join 跨平台拼路径；strings.HasSuffix 检查字符串是否以某子串结尾。
// 拼路径永远用 filepath.Join，不要手写 "/" 或 "\\"——Windows/Unix 共用同一套代码。
func ResolveResourcesRoot(javaRoot, configured string) string {
	if configured != "" {
		return configured
	}
	// 标准 Maven 结构：src/main/java → src/main/resources
	javaSuffix := filepath.Join("src", "main", "java")
	resSuffix := filepath.Join("src", "main", "resources")
	if strings.HasSuffix(javaRoot, javaSuffix) {
		return javaRoot[:len(javaRoot)-len(javaSuffix)] + resSuffix
	}
	// 非标准结构兜底：与 javaRoot 同级的 resources（尽力而为）
	return filepath.Join(filepath.Dir(javaRoot), "resources")
}

// OutputPath 由约定推导落盘路径。
//   - java 层（Resource=false）：javaRoot + 包路径 + 文件名（含包子目录）
//   - resource 层（Resource=true）：resourcesRoot + PkgSuffix（仅一级，不按完整包名建子目录）+ 文件名
//
// 返回 (path, error)：未知层（不在 Layers 表）返回 error，避免静默写错位置。
//
// Go 小白知识点：多返回值 (string, error) 是 Go 惯用的错误传递方式；
// 调用方必须检查 error（编译器不强制，但 go vet/lint 会警告忽略 error）。
func OutputPath(layer, basePackage, javaRoot, resourcesRoot, modelUpperCamel string) (string, error) {
	spec, ok := Layers[layer]
	if !ok {
		// fmt.Errorf 创建带格式的 error，%q 在字符串两侧加引号，便于 debug
		return "", fmt.Errorf("未知代码层: %q", layer)
	}
	file := modelUpperCamel + spec.NameSuffix + spec.Ext
	if spec.Resource {
		// resource 文件直接落 resourcesRoot/<PkgSuffix>/ —— 不按完整包名建层级子目录
		// 例：mapper-xml → /proj/src/main/resources/mapper/SysUserMapper.xml
		return filepath.Join(resourcesRoot, spec.PkgSuffix, file), nil
	}
	// java 文件按完整包名建子目录
	// 例：po → com.dahaoshen.demo.model.po → com/dahaoshen/demo/model/po
	pkg := basePackage + "." + spec.PkgSuffix
	rel := strings.ReplaceAll(pkg, ".", string(filepath.Separator))
	return filepath.Join(javaRoot, rel, file), nil
}

// since 是当天日期字符串（格式 yyyy-MM-dd），填入生成的 Java 文件 @since 注释。
// Go 小白知识点：包级变量（package-level variable）在整个包生命周期内共享。
// 这里不在库内调用 time.Now()，而是让调用方（main）通过 SetSince 注入，
// 好处是：测试中可不注入（since="" 也合法），不会因系统时间漂移导致测试非确定性。
var since = "" // 缺省空串；main 启动时通过 SetSince 赋值

// SetSince 供 main 包调用，设置生成文件的日期戳（yyyy-MM-dd）。
// 测试时无需调用——since="" 时模板里 @since 注释为空白，不影响功能正确性。
func SetSince(s string) { since = s }

// BuildTemplateData 把表元数据 + 配置组装成模板上下文（TemplateData）。
//
// 步骤：
//  1. 解析方言，获取 TypeMapper（类型映射器）
//  2. 遍历 Columns，映射 Java/JDBC 类型、转换字段名、判定 autoFill
//  3. 查找单列主键（无主键 / 复合主键 → error，快速失败）
//  4. 判定 IsWithAutoFill（任意字段有 autoFill 注解）
//  5. 填充 TemplateData 所有字段
//
// 错误情形：
//   - 未知方言
//   - 方言无对应 TypeMapper
//   - 无主键 / 复合主键（框架 @TableId 仅支持单列主键）
func BuildTemplateData(meta model.TableMetadata, cfg config.Config) (TemplateData, error) {
	// 1. 解析方言：字符串 "mysql" → dialect.MySQL 常量，未知方言返回 error
	d, err := dialect.FromValue(cfg.Datasource.Dialect)
	if err != nil {
		return TemplateData{}, err
	}
	// 2. 获取类型映射器（Go 接口：TypeMapper 的具体实现由方言决定）
	// cfg.DateType 透传给 typemap.For，控制日期列映射为 modern（java.time.*）还是 legacy（java.util.Date）
	mapper, err := typemap.For(d, cfg.DateType)
	if err != nil {
		return TemplateData{}, err
	}

	// 3. 构建 autoFill 快速查找集（小写，忽略大小写差异）
	insert := toLowerSet(cfg.AutoFill.InsertColumns)
	update := toLowerSet(cfg.AutoFill.UpdateColumns)

	// 4. 遍历列，构建字段元数据（数据库视角 → Java 视角）
	var fields []model.FieldMetadata
	for _, c := range meta.Columns {
		fields = append(fields, model.FieldMetadata{
			JavaType:     mapper.MapToJavaType(c.ColumnType), // 数据库类型 → Java 类型（如 bigint→Long）
			JdbcType:     mapper.MapToJdbcType(c.ColumnType), // 数据库类型 → JDBC 类型（如 bigint→BIGINT）
			Name:         naming.Camel(c.ColumnName),         // 下划线列名 → 小驼峰（user_name→userName）
			TableField:   c.ColumnName,                       // 保留原始列名，填入 @TableField(value=...)
			Comment:      c.ColumnComment,
			AutoFill:     autoFill(c.ColumnName, insert, update), // "" / "insert" / "update" / "insertUpdate"
			IsPrimaryKey: c.IsPrimaryKey,
		})
	}

	// 5. 定位单列主键——无主键或复合主键时快速失败（model.FindSinglePrimaryKey 返回 error）
	pk, err := model.FindSinglePrimaryKey(fields, meta.TableName)
	if err != nil {
		return TemplateData{}, err
	}

	// 6. 判定 IsWithAutoFill：只要有任一字段的 AutoFill 非空，即需要 import FieldFill
	withAutoFill := false
	for _, f := range fields {
		if f.AutoFill != "" {
			withAutoFill = true
			break
		}
	}

	// 7. 组装 TemplateData（模板上下文）
	modelUpper := naming.UpperCamel(meta.TableName) // sys_user → SysUser
	return TemplateData{
		Author:      cfg.Author,
		Since:       since, // 由 SetSince 注入；测试中为 ""，不影响编译/渲染
		TableName:   meta.TableName,
		BasePackage: cfg.BasePackage,

		ModelUpperCamel: modelUpper,
		ModelCamel:      naming.Camel(meta.TableName),               // sys_user → sysUser
		ModelKebab:      naming.Kebab(modelUpper),                   // SysUser → sys-user
		ModelComment:    strings.TrimSuffix(meta.TableComment, "表"), // "用户表" → "用户"

		PkFieldUpperCamel: naming.Capitalize(pk.Name), // id → Id；userName → UserName
		// IdType 从主键字段的 JavaType 动态推导（不读配置），保证类型与列定义一致
		// Go 小白知识点：这里直接用 pk.JavaType（已由 mapper.MapToJavaType 填好），
		// 避免配置写错（如把 varchar 主键配置成 Long）导致的类型不匹配
		IdType: pk.JavaType,

		UseJakarta:     cfg.UseJakarta != nil && *cfg.UseJakarta, // *bool 解引用，nil 视为 false
		IsWithAutoFill: withAutoFill,
		Fields:         fields,
	}, nil
}

// Generate 渲染指定层并落盘（dryRun=false）或写入 out（dryRun=true）。
//
// 参数说明：
//   - cfg：生成器全局配置（包名、输出根目录、数据源等）
//   - meta：单张表的元数据（列名、类型、注释、主键标记）
//   - layers：要生成的层列表，如 []string{"po","mapper","service"}
//   - dryRun：true=只打印到 out，不创建目录/文件；false=落盘
//   - out：dry-run 时的输出目标（io.Writer 接口）；落盘模式传 nil 即可
//
// io.Writer 接口抽象的价值（Go 小白重点）：
//
//	Generate 内部只调用 fmt.Fprintf(out, ...)，不关心 out 是 bytes.Buffer 还是 os.Stdout。
//	测试：传 &bytes.Buffer{}，事后检查 buf.Len() 和 buf.String()——完全内存操作，无副作用。
//	main：传 os.Stdout，内容直接打印到终端。
//	两个场景共用同一套代码，零重复。
func Generate(cfg config.Config, meta model.TableMetadata, layers []string, dryRun bool, out io.Writer) error {
	// 1. 组装模板上下文（类型映射、主键推导、autoFill 判定全在这里完成）
	data, err := BuildTemplateData(meta, cfg)
	if err != nil {
		return err
	}

	// 2. 预先计算 resources 根（只算一次，避免每层重复计算）
	// cfg.OutputRoot 是 java 根；cfg.ResourcesRoot 是可选的显式配置
	// Go 小白知识点：对不变的值做"一次计算、多次使用"是常见的性能意识（虽然这里开销微乎其微）
	resourcesRoot := ResolveResourcesRoot(cfg.OutputRoot, cfg.ResourcesRoot)

	// 3. 按层逐一渲染并输出
	for _, layer := range layers {
		// 调用 render.go 的 Render，执行 text/template 渲染，返回 Java 代码字符串
		code, err := Render(layer, data)
		if err != nil {
			return err
		}

		if dryRun {
			// dry-run：把代码写入 io.Writer（不落盘）
			// fmt.Fprintf 返回写入字节数和 error，这里忽略写入数（_），但应检查 error
			if _, err := fmt.Fprintf(out, "// ==== %s ====\n%s\n", layer, code); err != nil {
				return fmt.Errorf("dry-run 写出 %s 失败: %w", layer, err)
			}
			continue
		}

		// 落盘：推导目标路径 → 创建目录 → 写文件
		// 新签名：按层决定使用 java 根（cfg.OutputRoot）还是 resources 根（resourcesRoot）
		path, err := OutputPath(layer, cfg.BasePackage, cfg.OutputRoot, resourcesRoot, data.ModelUpperCamel)
		if err != nil {
			return err
		}

		// os.MkdirAll 等价于 mkdir -p：递归创建所有中间目录，目录已存在不报错
		// 0o755 是 Unix 权限八进制：rwxr-xr-x（所有人可读，仅 owner 可写）
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("创建目录 %s 失败: %w", filepath.Dir(path), err)
		}

		// os.WriteFile 覆盖写入文件（非原子：中途失败可能留下截断文件；Go 1.16+ 提供）；0o644 = rw-r--r--
		if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
			return fmt.Errorf("写文件 %s 失败: %w", path, err)
		}
	}
	return nil
}

// AllLayers 返回全部 14 层，固定生成顺序（po 优先、api 收尾，便于阅读产物）。
//
// 设计原则：顺序固定（而非遍历 Layers map），因为 map 遍历在 Go 中是随机序——
// 如果直接 range Layers，每次调用的输出顺序不同，对用户体验（diff、日志）不友好。
// 用切片显式定义顺序，保证稳定输出。
func AllLayers() []string {
	return []string{
		"po", "mapper", "mapper-xml", "service", "service-impl", "query", "converter",
		"req-dto", "resp-dto", "query-req-dto", "page-query-req-dto", "update-by-query-req-dto",
		"api", "api-impl",
	}
}

// onlyTableModifySet / withoutApiSet 复现 Java BaseCodeApplication 的层过滤集合。
//
// Go 小白知识点：map[string]bool 当集合用（O(1) 成员判断），比 []string 线性扫描快；
// 包级 var 只初始化一次，整个程序生命周期共享，不会重复 alloc。
var onlyTableModifySet = map[string]bool{
	"po": true, "req-dto": true, "resp-dto": true,
	"mapper-xml": true, "query": true, "query-req-dto": true,
}
var withoutApiSet = map[string]bool{
	"service": true, "service-impl": true, "po": true,
	"query": true, "mapper": true, "mapper-xml": true,
}

// SelectLayers 按两个开关对全集做交集过滤：
//   - 默认（两者 false）：返回全 14 层。
//   - onlyTableModify=true：仅保留"改表影响层"（po/req-dto/resp-dto/mapper-xml/query/query-req-dto）。
//   - withoutApi=true：仅保留"非 API 层"（service/service-impl/po/query/mapper/mapper-xml）。
//   - 两者同时 true：取交集（po/query/mapper-xml 这 3 层）。
//
// Go 小白知识点：用 map 做集合（值恒 true）做 O(1) 成员判断，按 AllLayers() 全集顺序过滤
// 以保持稳定输出顺序（map 遍历是随机序，切片遍历是稳定序）。
func SelectLayers(onlyTableModify, withoutApi bool) []string {
	var out []string
	for _, layer := range AllLayers() {
		// 任一过滤集拒绝该层，则跳过；两者都开启则同时检查（取交集）
		if onlyTableModify && !onlyTableModifySet[layer] {
			continue
		}
		if withoutApi && !withoutApiSet[layer] {
			continue
		}
		out = append(out, layer)
	}
	return out
}

// toLowerSet 把字符串切片转成小写集合（map[string]bool），用于 O(1) 大小写不敏感查找。
// Go 小白知识点：Go 没有 Set 类型，惯用 map[KeyType]bool 模拟；value 用 true 表示"存在"。
func toLowerSet(xs []string) map[string]bool {
	m := make(map[string]bool, len(xs))
	for _, x := range xs {
		m[strings.ToLower(x)] = true
	}
	return m
}

// autoFill 根据列名判断自动填充类型：
//   - 既在 insert 又在 update 列表 → "insertUpdate"
//   - 仅在 insert 列表             → "insert"
//   - 仅在 update 列表             → "update"
//   - 不在任何列表                 → ""（无自动填充）
//
// 对应 MyBatis-Plus 的 @TableField(fill = FieldFill.INSERT) / UPDATE / INSERT_UPDATE。
func autoFill(col string, insert, update map[string]bool) string {
	c := strings.ToLower(col)
	switch {
	case insert[c] && update[c]:
		return "insertUpdate"
	case insert[c]:
		return "insert"
	case update[c]:
		return "update"
	default:
		return ""
	}
}
