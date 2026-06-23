// Package generator 负责把元数据渲染成 Java 代码字符串（po/mapper/service 三层）。
// 核心流程：embed.FS 加载模板 → text/template 解析 → Execute 写入 bytes.Buffer → 返回字符串。
package generator

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/dahaoshen/base-code-go/internal/model"
	"github.com/dahaoshen/base-code-go/internal/naming"
)

// TemplateData 是模板渲染上下文，所有字段首字母大写（导出），才能被 text/template 的 {{.Field}} 访问。
// Go 小白知识点：text/template 通过反射读取结构体字段，未导出（小写）字段对模板不可见。
type TemplateData struct {
	// Author 代码作者，填入 @author 注释。
	Author string
	// Since 生成日期，填入 @since 注释。
	Since string
	// TableName 数据库表名（原始下划线命名），填入 @TableName。
	TableName string
	// BasePackage Java 基础包名，如 com.dahaoshen.demo。
	BasePackage string
	// ModelUpperCamel 大驼峰实体类名，如 SysUser。
	ModelUpperCamel string
	// ModelCamel 小驼峰变量名，如 sysUser。
	ModelCamel string
	// ModelKebab 中划线名，如 sys-user，用于 URL 路径等场景。
	ModelKebab string
	// ModelComment 中文表注释，填入 Javadoc 与 @Schema。
	ModelComment string
	// PkFieldUpperCamel 主键字段的大驼峰名（如 Id），用于拼 getter 方法名。
	PkFieldUpperCamel string
	// IdType 主键 Java 类型（如 Long / String），模板里直接展开。
	IdType string
	// UseJakarta 为 true 时引入 jakarta 包并生成 @Serial 注解（Spring Boot 3+ 用 jakarta 而非 javax）。
	UseJakarta bool
	// IsWithAutoFill 为 true 时 import 整个 annotation 包（含 FieldFill），用于自动填充场景。
	IsWithAutoFill bool
	// Fields 表的全部字段元数据，模板中用 range .Fields 遍历。
	// Go 小白知识点：range 内 "." 指向当前元素（FieldMetadata），父级字段要用 "$." 访问（如 $.UseJakarta）。
	Fields []model.FieldMetadata
}

// funcMap 把 naming 包的命名转换函数注册为模板函数。
// Go 小白知识点：text/template 的 FuncMap 允许在模板里调用 Go 函数，如 {{.Name | upperCamel}}。
// 必须在 template.New(...).Funcs(funcMap) 之后再调用 ParseFS，否则模板解析时找不到函数。
var funcMap = template.FuncMap{
	"camel":      naming.Camel,
	"upperCamel": naming.UpperCamel,
	"kebab":      naming.Kebab,
	"capitalize": naming.Capitalize,
}

// Render 渲染指定层的 Java 代码模板，返回生成的代码字符串。
// layer 取值："po" / "mapper" / "service"，对应 templates/<layer>.tmpl。
// Go 小白知识点：Go 惯例用 (value, error) 双返回值替代异常；调用方必须检查 error。
func Render(layer string, data TemplateData) (string, error) {
	name := layer + ".tmpl"
	// ParseFS 从嵌入的 embed.FS 中按路径读取并解析模板。
	// 注意：必须先 .Funcs(funcMap) 再 .ParseFS，顺序颠倒会报"函数未定义"错误。
	tmpl, err := template.New(name).Funcs(funcMap).
		ParseFS(templateFS, "templates/"+name)
	if err != nil {
		return "", fmt.Errorf("解析模板 %s 失败: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("渲染模板 %s 失败: %w", name, err)
	}
	return buf.String(), nil
}
