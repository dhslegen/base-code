package naming

import "testing"

// 表驱动测试：Go 惯用法，一组用例跑同一逻辑。
func TestCamel(t *testing.T) {
	cases := map[string]string{
		"user_name":  "userName",
		"user-name":  "userName",
		"created_at": "createdAt",
		"USER_NAME":  "userName", // 大写列名也应规整为小驼峰（与 Java toCamelCase 一致）
		"id":         "id",
	}
	for in, want := range cases {
		if got := Camel(in); got != want {
			t.Errorf("Camel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUpperCamel(t *testing.T) {
	cases := map[string]string{
		"user_role": "UserRole",
		"id":         "Id",
	}
	for in, want := range cases {
		if got := UpperCamel(in); got != want {
			t.Errorf("UpperCamel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestKebab(t *testing.T) {
	cases := map[string]string{
		"UserRole": "user-role",
		"Id":        "id",
	}
	for in, want := range cases {
		if got := Kebab(in); got != want {
			t.Errorf("Kebab(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCapitalize(t *testing.T) {
	cases := map[string]string{
		"userName": "UserName",
		"":          "", // 空串 guard
	}
	for in, want := range cases {
		if got := Capitalize(in); got != want {
			t.Errorf("Capitalize(%q) = %q, want %q", in, got, want)
		}
	}
}
