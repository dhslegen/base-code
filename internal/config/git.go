// Package config git helper functions.
package config

import (
	"os/exec"
	"strings"
)

// gitUserName 取 git config user.name 作为 author 缺省值；取不到返回空串。
// Go 小白知识点：exec.Command() 执行系统命令，.Output() 获得标准输出，error 可能来自命令执行失败。
func gitUserName() string {
	out, err := exec.Command("git", "config", "user.name").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
