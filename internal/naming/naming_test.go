package naming

import "testing"

// 表驱动测试：Go 惯用法，一组用例跑同一逻辑。
func TestCamel(t *testing.T) {
	cases := map[string]string{
		"user_name": "userName",
		"user-name": "userName",
		"CreatedAt": "createdAt",
		"id":        "id",
	}
	for in, want := range cases {
		if got := Camel(in); got != want {
			t.Errorf("Camel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUpperCamel(t *testing.T) {
	if got := UpperCamel("user_role"); got != "UserRole" {
		t.Errorf("UpperCamel = %q, want UserRole", got)
	}
}

func TestKebab(t *testing.T) {
	if got := Kebab("UserRole"); got != "user-role" {
		t.Errorf("Kebab = %q, want user-role", got)
	}
}

func TestCapitalize(t *testing.T) {
	if got := Capitalize("userName"); got != "UserName" {
		t.Errorf("Capitalize = %q, want UserName", got)
	}
}
