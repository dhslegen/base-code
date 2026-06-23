// Package generator 的渲染测试——驱动 TDD 红绿循环。
// Go 小白知识点：同包测试文件（package generator，非 package generator_test）可直接访问包内未导出成员。
package generator

import (
	"strings"
	"testing"

	"github.com/dahaoshen/base-code-go/internal/model"
)

// sampleData 构造一份完整的模板渲染测试数据，对应 sys_user 表。
func sampleData() TemplateData {
	return TemplateData{
		Author: "zhaowenhao", Since: "2026-06-23",
		TableName: "sys_user", BasePackage: "com.dahaoshen.demo",
		ModelUpperCamel: "SysUser", ModelCamel: "sysUser", ModelKebab: "sys-user",
		ModelComment: "系统用户", PkFieldUpperCamel: "Id", IdType: "Long",
		UseJakarta: true, IsWithAutoFill: false,
		Fields: []model.FieldMetadata{
			{JavaType: "Long", Name: "id", TableField: "id", Comment: "主键", IsPrimaryKey: true},
			{JavaType: "String", Name: "name", TableField: "name", Comment: "姓名"},
		},
	}
}

// TestRender_Mapper 验证 mapper 模板能渲染出正确的接口声明。
func TestRender_Mapper(t *testing.T) {
	out, err := Render("mapper", sampleData())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "public interface SysUserMapper extends BaseMapper<SysUser>") {
		t.Errorf("mapper 渲染缺少接口声明:\n%s", out)
	}
}

// TestRender_Po 验证 po 模板渲染出 @TableName、@TableId、字段声明、@Serial 等关键内容。
func TestRender_Po(t *testing.T) {
	out, err := Render("po", sampleData())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`@TableName(value = "sys_user")`,
		`@TableId(value = "id", type = IdType.AUTO)`,
		`private Long id;`,
		`private String name;`,
		`@Serial`, // UseJakarta=true
	} {
		if !strings.Contains(out, want) {
			t.Errorf("po 渲染缺少 %q:\n%s", want, out)
		}
	}
}

// TestRender_Service_InlinedAndPruned 验证 service 模板：
// 1. 内联了 com.dahaoshen.mybatismax.service.IMaxService（中央组件不再依赖外部 ISuperService）
// 2. 幂等相关代码（createIdempotency / LockCondition / ISuperService）已被彻底裁剪
func TestRender_Service_InlinedAndPruned(t *testing.T) {
	out, err := Render("service", sampleData())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "com.dahaoshen.mybatismax.service.IMaxService") {
		t.Error("service 应内联 IMaxService 中央组件包")
	}
	if !strings.Contains(out, "extends IMaxService<SysUser>") {
		t.Error("service 应 extends IMaxService")
	}
	// 幂等代码必须被裁剪
	for _, banned := range []string{"createIdempotency", "LockCondition", "ISuperService"} {
		if strings.Contains(out, banned) {
			t.Errorf("service 不应再出现已裁剪的 %q", banned)
		}
	}
}
