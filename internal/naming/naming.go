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
// 转换规则：
//   - 遇到分隔符（_ 或 -）时跳过，并标记下一字符要大写
//   - 首字母按 upperFirst 决定大/小写
//   - 其余字符（非分隔符触发位置）原样写入，保留原有大小写
func camel(s string, upperFirst bool) string {
	// isFirst 标记当前处理的是否为第一个有效字符（决定首字母大小写）
	isFirst := true
	nextUpper := false
	var b strings.Builder
	for _, c := range s { // range string 按 rune（Unicode 码点）遍历
		if c == '_' || c == '-' {
			nextUpper = true
			continue
		}
		if isFirst {
			// 首字母：按 upperFirst 决定大写或小写
			if upperFirst {
				b.WriteRune(toUpper(c))
			} else {
				b.WriteRune(toLower(c))
			}
			isFirst = false
			nextUpper = false
		} else if nextUpper {
			// 分隔符后的第一个字符强制大写
			b.WriteRune(toUpper(c))
			nextUpper = false
		} else {
			// 其余字符原样保留（保持输入的大小写）
			b.WriteRune(c)
		}
	}
	return b.String()
}

// Kebab 把大驼峰 UserRole 转成中划线 user-role。
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

// Capitalize 仅首字母大写，其余原样（用于由驼峰字段名拼 getter，如 userName -> UserName）。
func Capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = toUpper(r[0])
	return string(r)
}

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
