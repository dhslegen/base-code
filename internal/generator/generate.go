// generate.go 是编排器：把「表元数据 + 配置」组装成 TemplateData，再按约定路径渲染落盘。
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

// LayerSuffix 是「层 → Java 包后缀」约定表。
// M1 仅三层；M2 补齐 14 层时在此表里继续追加即可，Generate 代码无需修改（开闭原则）。
//
// 例：basePackage="com.dahaoshen.demo"，layer="po" → 包 com.dahaoshen.demo.model.po
var LayerSuffix = map[string]string{
	"po":      "model.po", // 实体类（Plain Object）层，放在 model.po 子包
	"mapper":  "mapper",   // MyBatis-Plus Mapper 接口层
	"service": "service",  // Service 接口层
}

// since 是当天日期字符串（格式 yyyy-MM-dd），填入生成的 Java 文件 @since 注释。
// Go 小白知识点：包级变量（package-level variable）在整个包生命周期内共享。
// 这里不在库内调用 time.Now()，而是让调用方（main）通过 SetSince 注入，
// 好处是：测试中可不注入（since="" 也合法），不会因系统时间漂移导致测试非确定性。
var since = "" // 缺省空串；main 启动时通过 SetSince 赋值

// SetSince 供 main 包调用，设置生成文件的日期戳（yyyy-MM-dd）。
// 测试时无需调用——since="" 时模板里 @since 注释为空白，不影响功能正确性。
func SetSince(s string) { since = s }

// OutputPath 由约定推导文件落盘的绝对路径。
//
// 规则：
//  1. 包路径 = basePackage + "." + LayerSuffix[layer]，点号换成路径分隔符
//  2. 文件名 = modelUpperCamel + 层后缀（po 层无后缀；其余层首字母大写的层名）+ ".java"
//
// 示例：layer="po", basePackage="com.dahaoshen.demo", outputRoot="/root", model="SysUser"
//
//	→ /root/com/dahaoshen/demo/model/po/SysUser.java
//
// 示例：layer="mapper", basePackage="com.dahaoshen.demo", outputRoot="/root", model="SysUser"
//
//	→ /root/com/dahaoshen/demo/mapper/SysUserMapper.java
func OutputPath(layer, basePackage, outputRoot, modelUpperCamel string) string {
	// 1. 组装包名：com.dahaoshen.demo.model.po
	pkg := basePackage + "." + LayerSuffix[layer]
	// 2. 把包名里的点换成系统路径分隔符
	//    filepath.Separator 在 Unix 是 '/'，Windows 是 '\\'
	rel := strings.ReplaceAll(pkg, ".", string(filepath.Separator))
	// 3. 拼文件名：po 层无后缀（SysUser.java），其余层加首字母大写层名（SysUserMapper.java）
	file := modelUpperCamel + suffixForFile(layer) + ".java"
	// 4. filepath.Join 跨平台安全地把各段拼成完整路径
	return filepath.Join(outputRoot, rel, file)
}

// suffixForFile 决定文件名中的层后缀：
//   - po 层：无后缀（实体类名就是 ModelUpperCamel 本身，如 SysUser）
//   - 其他层：首字母大写的层名（naming.UpperCamel 处理驼峰，如 mapper→Mapper、service→Service）
func suffixForFile(layer string) string {
	if layer == "po" {
		return ""
	}
	// naming.UpperCamel("mapper") = "Mapper"；若层名本已驼峰，此处也能正确处理
	return naming.UpperCamel(layer)
}

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
	mapper, err := typemap.For(d)
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
		ModelCamel:      naming.Camel(meta.TableName),     // sys_user → sysUser
		ModelKebab:      naming.Kebab(modelUpper),          // SysUser → sys-user
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

	// 2. 按层逐一渲染并输出
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
		path := OutputPath(layer, cfg.BasePackage, cfg.OutputRoot, data.ModelUpperCamel)

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
