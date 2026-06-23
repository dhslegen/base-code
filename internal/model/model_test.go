// Package model 的测试：验证 FindSinglePrimaryKey 在单/无/复合主键三种路径下的行为。
package model

import (
	"strings"
	"testing"
)

// TestFindSinglePrimaryKey_OK 验证单主键时正确返回主键字段。
func TestFindSinglePrimaryKey_OK(t *testing.T) {
	fields := []FieldMetadata{
		{Name: "id", IsPrimaryKey: true},
		{Name: "name"},
	}
	pk, err := FindSinglePrimaryKey(fields, "user")
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	if pk.Name != "id" {
		t.Errorf("主键 = %q, want id", pk.Name)
	}
}

// TestFindSinglePrimaryKey_None 验证无主键时快速失败并返回 error。
func TestFindSinglePrimaryKey_None(t *testing.T) {
	_, err := FindSinglePrimaryKey([]FieldMetadata{{Name: "name"}}, "user")
	if err == nil {
		t.Fatal("无主键时应返回错误")
	}
}

// TestFindSinglePrimaryKey_Composite 验证复合主键时快速失败，且错误信息含列名。
func TestFindSinglePrimaryKey_Composite(t *testing.T) {
	fields := []FieldMetadata{
		{Name: "a", TableField: "a", IsPrimaryKey: true},
		{Name: "b", TableField: "b", IsPrimaryKey: true},
	}
	_, err := FindSinglePrimaryKey(fields, "user")
	if err == nil {
		t.Fatal("复合主键时应返回错误")
	}
	if !strings.Contains(err.Error(), "a") || !strings.Contains(err.Error(), "b") {
		t.Errorf("复合主键错误信息应含列名 a/b，实际: %v", err)
	}
}
