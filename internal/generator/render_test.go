// Package generator 的渲染测试——驱动 TDD 红绿循环。
// Go 小白知识点：同包测试文件（package generator，非 package generator_test）可直接访问包内未导出成员。
package generator

import (
	"strings"
	"testing"

	"github.com/dhslegen/base-code/internal/model"
)

// sampleData 构造一份完整的模板渲染测试数据，对应 sys_user 表。
// JdbcType 字段是 mapper-xml 模板渲染 jdbcType 属性的来源，必须显式设置；
// render 阶段不经过 BuildTemplateData（不走类型映射），因此需在此手动填入。
func sampleData() TemplateData {
	return TemplateData{
		Author: "zhaowenhao", Since: "2026-06-23",
		TableName: "sys_user", BasePackage: "com.dahaoshen.demo",
		ServiceName: "demo-svc", BasePath: "/admin-api/demo",
		ModelUpperCamel: "SysUser", ModelCamel: "sysUser", ModelKebab: "sys-user",
		ModelComment: "系统用户", PkFieldUpperCamel: "Id", IdType: "Long",
		UseJakarta: true, IsWithAutoFill: false,
		Fields: []model.FieldMetadata{
			{JavaType: "Long", JdbcType: "BIGINT", Name: "id", TableField: "id", Comment: "主键", IsPrimaryKey: true},
			{JavaType: "String", JdbcType: "VARCHAR", Name: "name", TableField: "name", Comment: "姓名"},
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
	// 接口须声明与 service-impl @Override 对齐的 21 个方法
	for _, want := range []string{"existsByQuery", "getByQuery", "updateByQuery", "getIdGroupByQuery", "getIdMapByIds"} {
		if !strings.Contains(out, want) {
			t.Errorf("service 接口应声明 %q（与 service-impl 的 @Override 对齐）:\n%s", want, out)
		}
	}
}

// TestRender_Po_AutoFill 验证 autoFill 字段渲染 fill = FieldFill.INSERT，
// 且 IsWithAutoFill=true 时用通配 import（com.baomidou.mybatisplus.annotation.*）。
func TestRender_Po_AutoFill(t *testing.T) {
	d := sampleData()
	d.IsWithAutoFill = true
	d.Fields = append(d.Fields, model.FieldMetadata{
		JavaType: "LocalDateTime", Name: "createdAt", TableField: "created_at",
		Comment: "创建时间", AutoFill: "insert",
	})
	out, err := Render("po", d)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "fill = FieldFill.INSERT") {
		t.Errorf("应渲染 autoFill 的 FieldFill.INSERT:\n%s", out)
	}
	if !strings.Contains(out, "import com.baomidou.mybatisplus.annotation.*;") {
		t.Errorf("IsWithAutoFill=true 应使用通配 import:\n%s", out)
	}
}

// TestRender_Po_NoJakarta 验证 UseJakarta=false 时不出现 @Serial / java.io.Serial import。
func TestRender_Po_NoJakarta(t *testing.T) {
	d := sampleData()
	d.UseJakarta = false
	out, err := Render("po", d)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "@Serial") || strings.Contains(out, "import java.io.Serial;") {
		t.Errorf("UseJakarta=false 不应出现 @Serial / java.io.Serial:\n%s", out)
	}
}

// TestRender_ServiceImpl_InlinedAndPruned 验证 service-impl 内联 MaxServiceImpl、无幂等代码、主键方法引用正确。
func TestRender_ServiceImpl_InlinedAndPruned(t *testing.T) {
	out, err := Render("service-impl", sampleData())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"com.dahaoshen.mybatismax.service.impl.MaxServiceImpl",
		"extends MaxServiceImpl<SysUserMapper, SysUser>",
		"implements SysUserService",
		"toMap(list(), SysUser::getId)", // PkFieldUpperCamel=Id
	} {
		if !strings.Contains(out, want) {
			t.Errorf("service-impl 缺少 %q:\n%s", want, out)
		}
	}
	for _, banned := range []string{"createIdempotency", "LockCondition", "DistributedLock", "SuperServiceImpl", "updateByIdIdempotency", "saveOrUpdateIdempotency"} {
		if strings.Contains(out, banned) {
			t.Errorf("service-impl 不应出现已裁剪的 %q", banned)
		}
	}
}

// TestRender_Query 验证 query 对象渲染字段 @TableField 与 Serializable。
func TestRender_Query(t *testing.T) {
	out, err := Render("query", sampleData())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"public class SysUserQuery implements Serializable",
		`@TableField(value = "name")`,
		"private String name;",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("query 缺少 %q:\n%s", want, out)
		}
	}
}

// TestRender_Converter 验证 converter 的 MapStruct 接口与五个映射方法。
func TestRender_Converter(t *testing.T) {
	out, err := Render("converter", sampleData())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"public interface SysUserConverter",
		"Mappers.getMapper(SysUserConverter.class)",
		"SysUserRespDto toRespDto(SysUser sysUser)",
		"SysUserQuery fromQueryReqDtoToQuery(SysUserQueryReqDto sysUserQueryReqDto)",
		"List<SysUserRespDto> toRespDto(List<SysUser>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("converter 缺少 %q:\n%s", want, out)
		}
	}
}

// TestRender_MapperXml 验证 XML 的 namespace、id/result 主键区分、Base_Column_List 逗号分隔。
//
// Go 小白知识点：mapper-xml 是 XML 文件而非 Java 代码，但 Go 的 text/template 对文本类型无感知——
// 同一套 Render 函数既能渲染 .java 又能渲染 .xml，只要模板语法（{{.Field}}）正确即可。
func TestRender_MapperXml(t *testing.T) {
	out, err := Render("mapper-xml", sampleData())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`namespace="com.dahaoshen.demo.mapper.SysUserMapper"`,
		`<id column="id" jdbcType="BIGINT" property="id"/>`,          // 主键用 <id>
		`<result column="name" jdbcType="VARCHAR" property="name"/>`, // 非主键用 <result>
	} {
		if !strings.Contains(out, want) {
			t.Errorf("mapper-xml 缺少 %q:\n%s", want, out)
		}
	}
	// Base_Column_List：两列应以逗号分隔且不以逗号结尾
	if !strings.Contains(out, "id,") || !strings.Contains(out, "name") {
		t.Errorf("Base_Column_List 应含逗号分隔的列:\n%s", out)
	}
	if strings.Contains(out, "name,") {
		t.Errorf("Base_Column_List 末列不应有尾逗号:\n%s", out)
	}
}

// TestRender_ReqDto 验证 req-dto 字段循环 + @Schema + Serializable。
func TestRender_ReqDto(t *testing.T) {
	out, err := Render("req-dto", sampleData())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"public class SysUserReqDto implements Serializable",
		`@Schema(description = "姓名")`,
		"private String name;",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("req-dto 缺少 %q:\n%s", want, out)
		}
	}
}

// TestRender_RespDto 验证 resp-dto 类名与字段。
func TestRender_RespDto(t *testing.T) {
	out, err := Render("resp-dto", sampleData())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "public class SysUserRespDto implements Serializable") {
		t.Errorf("resp-dto 类声明错误:\n%s", out)
	}
	if !strings.Contains(out, "package com.dahaoshen.demo.model.dto.resp;") {
		t.Errorf("resp-dto 包名应为 model.dto.resp:\n%s", out)
	}
}

// TestRender_QueryReqDto 验证 query-req-dto 类名。
func TestRender_QueryReqDto(t *testing.T) {
	out, err := Render("query-req-dto", sampleData())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "public class SysUserQueryReqDto implements Serializable") {
		t.Errorf("query-req-dto 类声明错误:\n%s", out)
	}
}

// TestRender_PageQueryReqDto 验证分页 DTO 继承 QueryReqDto 且含 page() 方法与 current/size。
func TestRender_PageQueryReqDto(t *testing.T) {
	out, err := Render("page-query-req-dto", sampleData())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"class SysUserPageQueryReqDto extends SysUserQueryReqDto implements Serializable",
		"@EqualsAndHashCode(callSuper = true)",
		"private Long current = 1L;",
		"private Long size = 10L;",
		"public <T> Page<T> page()",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("page-query-req-dto 缺少 %q:\n%s", want, out)
		}
	}
}

// TestRender_UpdateByQueryReqDto 验证包装 DTO 含 entity(ReqDto) 与 queryReqDto 两字段。
func TestRender_UpdateByQueryReqDto(t *testing.T) {
	out, err := Render("update-by-query-req-dto", sampleData())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"class SysUserUpdateByQueryReqDto implements Serializable",
		"private SysUserReqDto entity;",
		"private SysUserQueryReqDto queryReqDto;",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("update-by-query-req-dto 缺少 %q:\n%s", want, out)
		}
	}
}

// TestRender_Api_PrunedIdempotency 验证 api 接口含核心端点、内联 Result、无幂等端点。
func TestRender_Api_PrunedIdempotency(t *testing.T) {
	out, err := Render("api", sampleData())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"import com.dahaoshen.restcore.Result;",
		"public interface SysUserApi {",
		`@FeignClient(name = "demo-svc")`,
		`String PREFIX = "/admin-api/demo/sys-user";`,
		`@GetMapping(PREFIX + "/page-all")`,
		`@RequestParam(value = "current", defaultValue = "1") Long current`,
		`@PostMapping(PREFIX + "/create")`,
		"Result<SysUserRespDto> create(",
		"Result<Boolean> existsByQuery(",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("api 缺少 %q:\n%s", want, out)
		}
	}
	for _, banned := range []string{"createIdempotency", "updateByIdIdempotency", "saveOrUpdateIdempotency", "LockConditionReqDto", "ApiConstants", "RequestBody PageQueryReqDto"} {
		if strings.Contains(out, banned) {
			t.Errorf("api 不应出现已裁剪的 %q", banned)
		}
	}
	endpoints := strings.Count(out, "@PostMapping") + strings.Count(out, "@GetMapping") +
		strings.Count(out, "@PutMapping") + strings.Count(out, "@DeleteMapping")
	if endpoints != 24 {
		t.Errorf("api 应有 24 个端点映射，实得 %d", endpoints)
	}
}

// TestRender_ApiImpl_PrunedAndReconciled 验证 api-impl 内联 Result、无幂等、delete 调 deleteById。
func TestRender_ApiImpl_PrunedAndReconciled(t *testing.T) {
	out, err := Render("api-impl", sampleData())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"import com.dahaoshen.restcore.Result;",
		"public class SysUserApiImpl implements SysUserApi {",
		"sysUserService.deleteById(id)", // delete 端点应调 deleteById（已纠正）
		"sysUserService.existsByQuery(query)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("api-impl 缺少 %q:\n%s", want, out)
		}
	}
	for _, banned := range []string{"createIdempotency", "LockCondition", "DistributedLock", "projectKeyword", ".delete(id)"} {
		if strings.Contains(out, banned) {
			t.Errorf("api-impl 不应出现 %q", banned)
		}
	}
}

// TestRender_ApiImpl_PageAllInlined 验证 api-impl 的 pageAll 为双参数签名、直调 service。
func TestRender_ApiImpl_PageAllInlined(t *testing.T) {
	out, err := Render("api-impl", sampleData())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"public Result<Page<SysUserRespDto>> pageAll(Long current, Long size) {",
		"sysUserService.pageAll(current, size);",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("api-impl 缺少 %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "pageAll(PageQueryReqDto") {
		t.Error("api-impl 不应再引用裸 PageQueryReqDto")
	}
}
