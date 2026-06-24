// generate_test.go 测试编排器的三个核心行为：
//  1. TestOutputPath       — 路径约定是否正确（po/mapper/service 三层）
//  2. TestGenerate_WritesFiles — 非 dry-run 时是否落盘
//  3. TestGenerate_DryRunNoWrite — dry-run 时只写 out、不落盘
//  4. TestBuildTemplateData_NoKey — 无主键/复合主键时 BuildTemplateData 应返回 error
//
// Go 小白知识点：
//   - bytes.Buffer 实现了 io.Writer 接口，测试时可用它"截获"写出内容，无需真正的文件或 os.Stdout。
//   - t.TempDir() 在测试结束后自动清理临时目录，省去手动 defer os.RemoveAll 的麻烦。
//   - os.ReadDir 读目录条目，len==0 证明没有任何文件被创建，用于验证 dry-run 不落盘。
package generator

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/dahaoshen/base-code-go/internal/config"
	"github.com/dahaoshen/base-code-go/internal/model"
)

// sampleMeta 构造 sys_user 表的最小化元数据：一个 bigint 主键 + 一个 varchar 字段。
func sampleMeta() model.TableMetadata {
	return model.TableMetadata{
		TableName: "sys_user", TableComment: "用户表",
		Columns: []model.ColumnMetadata{
			{ColumnName: "id", ColumnType: "bigint", ColumnComment: "主键", IsPrimaryKey: true},
			{ColumnName: "user_name", ColumnType: "varchar(64)", ColumnComment: "用户名"},
		},
	}
}

// sampleCfg 构造一个指向临时目录 out 的最小化配置。
// UseJakarta 用指针是因为 config.Config.UseJakarta 为 *bool，以区分"未配置"和"明确 false"。
func sampleCfg(out string) config.Config {
	jakarta := true
	return config.Config{
		BasePackage: "com.dahaoshen.demo", OutputRoot: out, Author: "tester",
		UseJakarta: &jakarta, DateType: "modern",
		Datasource: config.Datasource{Dialect: "mysql"},
		AutoFill:   config.AutoFill{InsertColumns: []string{"created_at"}, UpdateColumns: []string{"updated_at"}},
	}
}

// TestOutputPath 验证 7 层的落盘路径：java 层按包路径、mapper-xml 落 resources/mapper 且不建包子目录。
//
// Go 小白知识点：filepath.FromSlash 把"/"统一转成当前 OS 的路径分隔符，
// 让测试用例写成 Unix 风格 "/"，在 Windows 上也能正确比较 "\"。
func TestOutputPath(t *testing.T) {
	jr := filepath.FromSlash("/proj/src/main/java")
	rr := filepath.FromSlash("/proj/src/main/resources")
	cases := []struct {
		layer, want string
	}{
		{"po", "/proj/src/main/java/com/dahaoshen/demo/model/po/SysUser.java"},
		{"mapper", "/proj/src/main/java/com/dahaoshen/demo/mapper/SysUserMapper.java"},
		{"service", "/proj/src/main/java/com/dahaoshen/demo/service/SysUserService.java"},
		{"service-impl", "/proj/src/main/java/com/dahaoshen/demo/service/impl/SysUserServiceImpl.java"},
		{"query", "/proj/src/main/java/com/dahaoshen/demo/model/query/SysUserQuery.java"},
		{"converter", "/proj/src/main/java/com/dahaoshen/demo/converter/SysUserConverter.java"},
		{"mapper-xml", "/proj/src/main/resources/mapper/SysUserMapper.xml"},
		{"req-dto", "/proj/src/main/java/com/dahaoshen/demo/model/dto/req/SysUserReqDto.java"},
		{"resp-dto", "/proj/src/main/java/com/dahaoshen/demo/model/dto/resp/SysUserRespDto.java"},
		{"query-req-dto", "/proj/src/main/java/com/dahaoshen/demo/model/dto/req/SysUserQueryReqDto.java"},
		{"page-query-req-dto", "/proj/src/main/java/com/dahaoshen/demo/model/dto/req/SysUserPageQueryReqDto.java"},
		{"update-by-query-req-dto", "/proj/src/main/java/com/dahaoshen/demo/model/dto/req/SysUserUpdateByQueryReqDto.java"},
	}
	for _, tc := range cases {
		got, err := OutputPath(tc.layer, "com.dahaoshen.demo", jr, rr, "SysUser")
		if err != nil {
			t.Fatalf("layer=%s 意外错误: %v", tc.layer, err)
		}
		if want := filepath.FromSlash(tc.want); got != want {
			t.Errorf("layer=%s: OutputPath = %q, want %q", tc.layer, got, want)
		}
	}
	// 边界：未知层应返回 error（而非静默返回错误路径）
	if _, err := OutputPath("nope", "com.x", jr, rr, "X"); err == nil {
		t.Error("未知层应返回 error")
	}
}

// TestResolveResourcesRoot 验证由 java 根派生 resources 根，配置优先。
//
// 两种路径：
//  1. configured 为空 → 把 javaRoot 末段 src/main/java 换成 src/main/resources
//  2. configured 非空 → 直接返回 configured（配置优先于约定）
func TestResolveResourcesRoot(t *testing.T) {
	jr := filepath.FromSlash("/proj/src/main/java")
	// 场景 1：未配置 resources-root，自动由 java 根派生
	if got := ResolveResourcesRoot(jr, ""); got != filepath.FromSlash("/proj/src/main/resources") {
		t.Errorf("派生 = %q", got)
	}
	// 场景 2：显式配置优先，不做任何派生
	if got := ResolveResourcesRoot(jr, "/custom/res"); got != "/custom/res" {
		t.Errorf("配置优先 = %q", got)
	}
}

// TestGenerate_WritesFiles 验证 dryRun=false 时三层文件均落盘。
// 只检查 po 文件是否存在（作为代表性断言；mapper/service 同理生成）。
func TestGenerate_WritesFiles(t *testing.T) {
	dir := t.TempDir()
	cfg := sampleCfg(dir)
	// dryRun=false, out=nil — 不需要 io.Writer，传 nil 即可
	if err := Generate(cfg, sampleMeta(), []string{"po", "mapper", "service"}, false, nil); err != nil {
		t.Fatal(err)
	}
	// po 文件名无后缀：SysUser.java（不是 SysUserPo.java）
	poPath := filepath.Join(dir, "com/dahaoshen/demo/model/po/SysUser.java")
	if _, err := os.Stat(poPath); err != nil {
		t.Errorf("未生成 po 文件: %v", err)
	}
}

// TestGenerate_DryRunNoWrite 验证 dryRun=true 时：
//  1. 内容写入 out（buf.Len() > 0）
//  2. 临时目录保持空（不落盘）
//  3. 约定路径上确实未创建文件
//
// 这展示了 io.Writer 接口的核心价值：Generate 内部对 out 只调用 fmt.Fprintf，
// 测试传 bytes.Buffer、main 传 os.Stdout，代码一字不改。
func TestGenerate_DryRunNoWrite(t *testing.T) {
	dir := t.TempDir()
	cfg := sampleCfg(dir)
	var buf bytes.Buffer
	if err := Generate(cfg, sampleMeta(), []string{"po"}, true, &buf); err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Error("dry-run 应把内容写到 out")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Error("dry-run 不应落盘")
	}
	// 精准验证：po 层约定路径上确实未创建文件
	poPath := filepath.Join(dir, "com/dahaoshen/demo/model/po/SysUser.java")
	if _, err := os.Stat(poPath); !os.IsNotExist(err) {
		t.Error("dry-run 不应在约定路径落盘")
	}
}

// TestBuildTemplateData_NoPrimaryKey 验证无主键时 BuildTemplateData 返回 error。
// Go 中"快速失败"（fail-fast）是惯用手法：非法状态尽早 return error，
// 避免后续代码对 TemplateData 的零值做出错误假设。
func TestBuildTemplateData_NoPrimaryKey(t *testing.T) {
	meta := model.TableMetadata{
		TableName:    "no_pk_table",
		TableComment: "无主键表",
		Columns: []model.ColumnMetadata{
			{ColumnName: "name", ColumnType: "varchar(64)", ColumnComment: "名称"},
		},
	}
	jakarta := true
	cfg := config.Config{
		BasePackage: "com.test", Author: "tester",
		UseJakarta: &jakarta, DateType: "modern",
		Datasource: config.Datasource{Dialect: "mysql"},
	}
	_, err := BuildTemplateData(meta, cfg)
	if err == nil {
		t.Error("无主键时 BuildTemplateData 应返回 error，但返回了 nil")
	}
}

// TestBuildTemplateData_CompositePrimaryKey 验证复合主键时 BuildTemplateData 返回 error。
// 框架（MyBatis-Plus @TableId）只支持单列主键，复合主键必须快速失败。
func TestBuildTemplateData_CompositePrimaryKey(t *testing.T) {
	meta := model.TableMetadata{
		TableName:    "composite_pk_table",
		TableComment: "复合主键表",
		Columns: []model.ColumnMetadata{
			{ColumnName: "pk1", ColumnType: "bigint", IsPrimaryKey: true},
			{ColumnName: "pk2", ColumnType: "bigint", IsPrimaryKey: true},
		},
	}
	jakarta := true
	cfg := config.Config{
		BasePackage: "com.test", Author: "tester",
		UseJakarta: &jakarta, DateType: "modern",
		Datasource: config.Datasource{Dialect: "mysql"},
	}
	_, err := BuildTemplateData(meta, cfg)
	if err == nil {
		t.Error("复合主键时 BuildTemplateData 应返回 error，但返回了 nil")
	}
}

// TestGenerate_MapperXmlToResources 验证 mapper-xml 落到 resources/mapper 而非 java 包路径。
//
// Go 小白知识点：
//   - t.TempDir() 创建的目录在测试结束后自动清理，路径具有唯一性（避免并行测试冲突）。
//   - filepath.Join 跨平台拼接：Windows 用 "\", Unix 用 "/"，不要手写分隔符。
//   - sampleCfg 的 OutputRoot 必须以 "src/main/java" 结尾，ResolveResourcesRoot 才能
//     自动派生 resources 根（把末段 java 替换为 resources）。
func TestGenerate_MapperXmlToResources(t *testing.T) {
	root := t.TempDir()
	// OutputRoot 以 src/main/java 结尾，使 ResolveResourcesRoot 能派生出 src/main/resources
	javaRoot := filepath.Join(root, "src", "main", "java")
	cfg := sampleCfg(javaRoot) // OutputRoot = .../src/main/java
	if err := Generate(cfg, sampleMeta(), []string{"mapper-xml"}, false, nil); err != nil {
		t.Fatal(err)
	}
	// mapper-xml 应落到 resources/mapper/SysUserMapper.xml，而非 java 包子目录
	xmlPath := filepath.Join(root, "src", "main", "resources", "mapper", "SysUserMapper.xml")
	if _, err := os.Stat(xmlPath); err != nil {
		t.Errorf("mapper-xml 应落到 %s: %v", xmlPath, err)
	}
}

// TestGenerate_WithoutApiLayers 验证非 API 的 7 层全部能渲染并落盘（端到端冒烟，不连库）。
//
// 七层：po、mapper、service、service-impl、query、converter、mapper-xml。
// 此测试确认 Task1-4 的基础设施完整：所有模板都能加载、所有层都有落盘路径、无 panic。
// 抽查 service-impl 与 mapper-xml 两个代表性文件。
//
// Go 小白知识点：
//   - 多个文件的验证通过 for 循环批量检查，减少重复代码。
//   - 使用 []string{...} 承载层列表，与 cmd/gen.go 的实现保持对应。
func TestGenerate_WithoutApiLayers(t *testing.T) {
	root := t.TempDir()
	javaRoot := filepath.Join(root, "src", "main", "java")
	cfg := sampleCfg(javaRoot)
	layers := []string{"po", "mapper", "service", "service-impl", "query", "converter", "mapper-xml"}
	if err := Generate(cfg, sampleMeta(), layers, false, nil); err != nil {
		t.Fatalf("7 层生成失败: %v", err)
	}
	// 抽查 service-impl 与 mapper-xml 两个代表
	si := filepath.Join(root, "src", "main", "java", "com", "dahaoshen", "demo", "service", "impl", "SysUserServiceImpl.java")
	xml := filepath.Join(root, "src", "main", "resources", "mapper", "SysUserMapper.xml")
	for _, p := range []string{si, xml} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("应生成 %s: %v", p, err)
		}
	}
}

// TestGenerate_DtoLayers 验证 5 个 DTO 层全部能渲染落盘到 model/dto/req|resp。
func TestGenerate_DtoLayers(t *testing.T) {
	root := t.TempDir()
	cfg := sampleCfg(filepath.Join(root, "src", "main", "java"))
	layers := []string{"req-dto", "resp-dto", "query-req-dto", "page-query-req-dto", "update-by-query-req-dto"}
	if err := Generate(cfg, sampleMeta(), layers, false, nil); err != nil {
		t.Fatalf("DTO 层生成失败: %v", err)
	}
	base := filepath.Join(root, "src", "main", "java", "com", "dahaoshen", "demo", "model", "dto")
	checks := map[string]string{
		filepath.Join(base, "req", "SysUserReqDto.java"):              "req",
		filepath.Join(base, "resp", "SysUserRespDto.java"):            "resp",
		filepath.Join(base, "req", "SysUserQueryReqDto.java"):         "query",
		filepath.Join(base, "req", "SysUserPageQueryReqDto.java"):     "page",
		filepath.Join(base, "req", "SysUserUpdateByQueryReqDto.java"): "update",
	}
	for p := range checks {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("应生成 %s: %v", p, err)
		}
	}
}

// TestSelectLayers 验证层过滤交集逻辑（复现 Java BaseCodeApplication）。
//
// Go 小白知识点：
//   - 辅助函数 contains 对切片做线性扫描（O(n)），仅用于测试断言，无性能要求。
//   - 测试函数命名 TestXxx，参数固定为 *testing.T，go test 自动识别并执行。
//   - 通过多个子场景（默认/withoutApi/onlyTableModify/两者）在同一测试函数内覆盖多条路径，
//     避免重复 boilerplate（测试也讲 DRY）。
func TestSelectLayers(t *testing.T) {
	contains := func(xs []string, v string) bool {
		for _, x := range xs {
			if x == v {
				return true
			}
		}
		return false
	}
	// 默认：全 14 层，且成员与顺序与 AllLayers() 完全一致
	if got := SelectLayers(false, false); !reflect.DeepEqual(got, AllLayers()) {
		t.Errorf("默认应等于 AllLayers()（全14层同序），得 %v", got)
	}
	// withoutApi：不含 api/api-impl/dto
	wa := SelectLayers(false, true)
	for _, must := range []string{"po", "service-impl", "mapper-xml"} {
		if !contains(wa, must) {
			t.Errorf("--without-api 应含 %q", must)
		}
	}
	for _, no := range []string{"api", "api-impl", "req-dto"} {
		if contains(wa, no) {
			t.Errorf("--without-api 不应含 %q", no)
		}
	}
	// onlyTableModify：含 req-dto/resp-dto，不含 service
	otm := SelectLayers(true, false)
	for _, must := range []string{"po", "req-dto", "resp-dto", "query-req-dto"} {
		if !contains(otm, must) {
			t.Errorf("--only-table-modify 应含 %q", must)
		}
	}
	if contains(otm, "service") {
		t.Error("--only-table-modify 不应含 service")
	}
	// 两者交集 = {po, query, mapper-xml}
	both := SelectLayers(true, true)
	if len(both) != 3 {
		t.Errorf("两者交集应 3 层，得 %v", both)
	}
	for _, must := range []string{"po", "query", "mapper-xml"} {
		if !contains(both, must) {
			t.Errorf("交集应含 %q", must)
		}
	}
}
