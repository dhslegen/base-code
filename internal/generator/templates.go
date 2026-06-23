// Package generator 负责把元数据渲染成 Java 代码字符串（po/mapper/service 三层）。
// 本文件通过 //go:embed 把模板目录编进二进制，实现"单文件可执行"——发布时无需附带 templates/ 目录。
package generator

import "embed"

// Go 小白知识点：//go:embed 是编译指令（directive），必须紧贴 var 声明、中间不能有空行。
// "all:templates" 表示嵌入 templates/ 目录下的全部文件（含以 . 开头的隐藏文件）。
// 变量类型必须是 embed.FS（虚拟文件系统），后续用 ParseFS 读取其中的模板文件。
//
//go:embed all:templates
var templateFS embed.FS
