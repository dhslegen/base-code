// Package config 负责加载 base-code.yaml 并补齐约定默认值。
// 设计目标：约定优于配置——layers/包名/idType 等不再配置，仅留必填的工程包与数据库连接。
package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config 是生成器的全部配置。
// Go 小白知识点：结构体标签 `yaml:"..."` 告诉 yaml 库 YAML 键如何对应字段。
type Config struct {
	BasePackage   string     `yaml:"base-package"`
	OutputRoot    string     `yaml:"output-root"`
	ResourcesRoot string     `yaml:"resources-root"` // 可选；mapper-xml 输出根，缺省由 OutputRoot 推导
	Author        string     `yaml:"author"`
	UseJakarta    *bool      `yaml:"use-jakarta"` // 指针：区分「未配置」与「配置为 false」
	DateType      string     `yaml:"date-type"`
	Api           Api        `yaml:"api"`
	Datasource    Datasource `yaml:"datasource"`
	AutoFill      AutoFill   `yaml:"auto-fill"`
}

// Datasource 表示数据库连接配置。
type Datasource struct {
	Dialect  string `yaml:"dialect"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}

// Api 表示生成 API 层所需的服务标识配置。
// service-name 填入 @FeignClient(name=...)（注册中心应用名）；
// base-path 是所有 API 端点的基础路径前缀。
// 两者缺省均从 base-package 末段派生（见 applyDefaults）。
type Api struct {
	ServiceName string `yaml:"service-name"`
	BasePath    string `yaml:"base-path"`
}

// AutoFill 表示自动填充列的约定。
type AutoFill struct {
	InsertColumns []string `yaml:"insert-columns"`
	UpdateColumns []string `yaml:"update-columns"`
}

// root 用于剥掉 yaml 顶层的 base-code: 包裹层。
type root struct {
	BaseCode Config `yaml:"base-code"`
}

// Load 读取并解析配置文件，随后补齐约定默认值。
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("读取配置 %s 失败: %w", path, err)
	}
	var r root
	if err := yaml.Unmarshal(data, &r); err != nil {
		return Config{}, fmt.Errorf("解析配置 %s 失败: %w", path, err)
	}
	cfg := r.BaseCode
	applyDefaults(&cfg)
	if cfg.BasePackage == "" {
		return Config{}, fmt.Errorf("base-package 为必填项")
	}
	return cfg, nil
}

// applyDefaults 填充约定缺省值（指针接收者以便修改入参）。
// Go 小白知识点：*bool 指针用来区分"未设置"（nil）与"明确设置为 false"（&false）。
func applyDefaults(c *Config) {
	if c.UseJakarta == nil {
		t := true
		c.UseJakarta = &t
	}
	if c.DateType == "" {
		c.DateType = "modern"
	}
	if c.Author == "" {
		c.Author = gitUserName()
	}
	if len(c.AutoFill.InsertColumns) == 0 {
		c.AutoFill.InsertColumns = []string{"created_at", "updated_at", "created_by", "updated_by"}
	}
	if len(c.AutoFill.UpdateColumns) == 0 {
		c.AutoFill.UpdateColumns = []string{"updated_at", "updated_by"}
	}
	// api 缺省派生：service-name = base-package 末段；base-path = "/" + 末段。
	// 显式配置优先——只补空缺字段，两字段各自独立判断。
	seg := c.BasePackage
	if i := strings.LastIndex(seg, "."); i >= 0 {
		seg = seg[i+1:]
	}
	if c.Api.ServiceName == "" {
		c.Api.ServiceName = seg
	}
	if c.Api.BasePath == "" {
		c.Api.BasePath = "/" + seg
	}
}
