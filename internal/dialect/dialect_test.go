// Package dialect 的测试：验证 FromValue 的解析与错误路径。
package dialect

import "testing"

// TestFromValue_OK 验证已知方言被正确解析。
func TestFromValue_OK(t *testing.T) {
	d, err := FromValue("mysql")
	if err != nil || d != MySQL {
		t.Errorf("FromValue(\"mysql\") = (%v, %v), want (mysql, nil)", d, err)
	}
}

// TestFromValue_Unknown 验证未知方言返回 error。
func TestFromValue_Unknown(t *testing.T) {
	if _, err := FromValue("oracle"); err == nil {
		t.Error("FromValue(\"oracle\") 应返回 error")
	}
}
