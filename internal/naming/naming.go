// Package naming 提供数据库命名（下划线/中划线）与 Java 命名（驼峰/中划线）之间的转换。
// 对应 Java 版 BaseCodeApplication 里的 toCamelCase/toUpperCamelCase/toKebabCase/capitalize。
package naming

import "strings"

// Camel 把 user_name / user-name 转成小驼峰 userName。
func Camel(s string) string { return camel(s, false) }

// UpperCamel 把 user_name 转成大驼峰 UserName。
func UpperCamel(s string) string { return camel(s, true) }

// camel 是内部实现：upperFirst 决定首字母是否大写。
// Go 小白知识点：小写开头的函数名 = 包私有（未导出），不会被外部包引用。
// 语义对齐 Java toCamelCase：非分隔符字符一律按 nextUpper 决定大写、否则强制小写，
// 这样大写/混合大小写的列名（如 USER_NAME）也能规整为小驼峰 userName。
func camel(s string, upperFirst bool) string {
	nextUpper := upperFirst
	var b strings.Builder
	for _, c := range s { // range string 按 rune（Unicode 码点）遍历
		if c == '_' || c == '-' {
			nextUpper = true
			continue
		}
		if nextUpper {
			b.WriteRune(toUpper(c))
			nextUpper = false
		} else {
			b.WriteRune(toLower(c))
		}
	}
	return b.String()
}

// Kebab 把大驼峰 UserRole 转成中划线 user-role。
// 契约：仅接受大驼峰输入（与 Java toKebabCase 一致）；下划线/中划线不会被处理，
// 调用方应传入 UpperCamel 结果（如 naming.Kebab(naming.UpperCamel(name))）。
func Kebab(s string) string {
	var b strings.Builder
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			if i != 0 {
				b.WriteByte('-')
			}
			b.WriteRune(toLower(c))
		} else {
			b.WriteRune(c)
		}
	}
	return b.String()
}

// Capitalize 仅把首字符大写，其余字符不做任何大小写转换（区别于 UpperCamel 会整体规整）。
// 用于由小驼峰字段名拼 getter，如 userName -> UserName。
func Capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = toUpper(r[0])
	return string(r)
}

// toUpper/toLower 只处理 ASCII 字母：手动实现而非用 unicode 包，是为零依赖且够用（列名均为 ASCII）。
//
// toUpper 将单个 rune 的小写字母转大写，非字母原样返回。
func toUpper(c rune) rune {
	if c >= 'a' && c <= 'z' {
		return c - 32
	}
	return c
}

// toLower 将单个 rune 的大写字母转小写，非字母原样返回。
func toLower(c rune) rune {
	if c >= 'A' && c <= 'Z' {
		return c + 32
	}
	return c
}
