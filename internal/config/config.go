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

// Overrides 表示命令行内联配置对文件配置的逐项覆盖。
// Go 小白知识点：全指针字段——nil 表示「命令行未提供该项」，与零值（""、0、false）区分开，
// 这样 --use-jakarta=false、--db-port 0 这类「显式传零值」也能正确表达。
type Overrides struct {
	BasePackage    *string
	OutputRoot     *string
	ResourcesRoot  *string
	Author         *string
	UseJakarta     *bool
	DateType       *string
	Dialect        *string
	DbHost         *string
	DbPort         *int
	DbUser         *string
	DbPassword     *string
	DbName         *string
	ServiceName    *string
	BasePath       *string
	AutoFillInsert *[]string
	AutoFillUpdate *[]string
}

// LoadWithOverrides 加载配置并叠加命令行内联覆盖。
// requireFile=false 时文件不存在不视为错误（纯 flag 模式，agent 一行直达）；
// true 时文件必须存在（用户显式 --config，不静默忽略）。
// 优先级：显式 flag > 配置文件 > 约定默认值。
func LoadWithOverrides(path string, requireFile bool, ov Overrides) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		var r root
		if err := yaml.Unmarshal(data, &r); err != nil {
			return Config{}, fmt.Errorf("解析配置 %s 失败: %w", path, err)
		}
		cfg = r.BaseCode
	case os.IsNotExist(err) && !requireFile:
		// 纯 flag 模式：默认配置文件缺席是合法状态，从零配置起步
	default:
		return Config{}, fmt.Errorf("读取配置 %s 失败: %w", path, err)
	}
	applyOverrides(&cfg, ov)
	applyDefaults(&cfg)
	if err := validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Load 读取并解析配置文件，随后补齐约定默认值。
// LoadWithOverrides 的薄封装：必须有文件、无内联覆盖，保持既有调用方兼容。
func Load(path string) (Config, error) {
	return LoadWithOverrides(path, true, Overrides{})
}

// applyOverrides 将命令行显式提供的内联配置逐项覆盖到 cfg（nil 跳过）。
func applyOverrides(c *Config, ov Overrides) {
	if ov.BasePackage != nil {
		c.BasePackage = *ov.BasePackage
	}
	if ov.OutputRoot != nil {
		c.OutputRoot = *ov.OutputRoot
	}
	if ov.ResourcesRoot != nil {
		c.ResourcesRoot = *ov.ResourcesRoot
	}
	if ov.Author != nil {
		c.Author = *ov.Author
	}
	if ov.UseJakarta != nil {
		c.UseJakarta = ov.UseJakarta
	}
	if ov.DateType != nil {
		c.DateType = *ov.DateType
	}
	if ov.Dialect != nil {
		c.Datasource.Dialect = *ov.Dialect
	}
	if ov.DbHost != nil {
		c.Datasource.Host = *ov.DbHost
	}
	if ov.DbPort != nil {
		c.Datasource.Port = *ov.DbPort
	}
	if ov.DbUser != nil {
		c.Datasource.Username = *ov.DbUser
	}
	if ov.DbPassword != nil {
		c.Datasource.Password = *ov.DbPassword
	}
	if ov.DbName != nil {
		c.Datasource.Database = *ov.DbName
	}
	if ov.ServiceName != nil {
		c.Api.ServiceName = *ov.ServiceName
	}
	if ov.BasePath != nil {
		c.Api.BasePath = *ov.BasePath
	}
	if ov.AutoFillInsert != nil {
		c.AutoFill.InsertColumns = *ov.AutoFillInsert
	}
	if ov.AutoFillUpdate != nil {
		c.AutoFill.UpdateColumns = *ov.AutoFillUpdate
	}
}

// validate 校验必填项。错误信息面向 agent 自修复：列出缺失 flag 并给出可复制的完整命令样例。
func validate(c Config) error {
	var missing []string
	if c.BasePackage == "" {
		missing = append(missing, "--base-package")
	}
	if c.OutputRoot == "" {
		missing = append(missing, "--output-root")
	}
	if c.Datasource.Host == "" {
		missing = append(missing, "--db-host")
	}
	if c.Datasource.Username == "" {
		missing = append(missing, "--db-user")
	}
	if c.Datasource.Database == "" {
		missing = append(missing, "--db-name")
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf(`缺少必填配置：%s
可写入配置文件（--config），或用内联 flag 一行直达，例如：
  base-code gen --tables sys_user \
    --base-package com.example.demo --output-root ./src/main/java \
    --dialect mysql --db-host 127.0.0.1 --db-user root --db-password '***' --db-name demo`,
		strings.Join(missing, "、"))
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
	// 端口缺省按方言派生：mysql→3306，postgresql→5432（内联模式 agent 可少传一项）。
	if c.Datasource.Port == 0 {
		if c.Datasource.Dialect == "postgresql" {
			c.Datasource.Port = 5432
		} else {
			c.Datasource.Port = 3306
		}
	}
}
