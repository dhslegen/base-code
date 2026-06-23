package model

import "testing"

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

func TestFindSinglePrimaryKey_None(t *testing.T) {
	_, err := FindSinglePrimaryKey([]FieldMetadata{{Name: "name"}}, "user")
	if err == nil {
		t.Fatal("无主键时应返回错误")
	}
}

func TestFindSinglePrimaryKey_Composite(t *testing.T) {
	fields := []FieldMetadata{
		{Name: "a", TableField: "a", IsPrimaryKey: true},
		{Name: "b", TableField: "b", IsPrimaryKey: true},
	}
	_, err := FindSinglePrimaryKey(fields, "user")
	if err == nil {
		t.Fatal("复合主键时应返回错误")
	}
}
