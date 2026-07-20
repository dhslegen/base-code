# base-code

[![Release](https://img.shields.io/github/v/release/dhslegen/base-code?sort=semver&color=brightgreen)](https://github.com/dhslegen/base-code/releases)
[![License](https://img.shields.io/github/license/dhslegen/base-code?color=blue)](LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/dhslegen/base-code)](go.mod)
![Platforms](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey)

数据库代码生成器（Go 版）。连接 MySQL / PostgreSQL，扫描表结构，按约定生成 MyBatis-Plus 分层 Java 代码（默认 6 层后端核心，加 `--with-api` 生成全 14 层）。

> 想深入了解每一站的实现原理？阅读 [docs/TUTORIAL.md](docs/TUTORIAL.md)（全流水线教学，含 Java 对照与 Go 小白知识点）。

---

## 安装

### 方式一：Homebrew（macOS / Linux，推荐）

```bash
brew install dhslegen/tap/base-code
```

安装后 `base-code` 直接可用（Homebrew 自动纳入 PATH）。

### 方式二：Scoop（Windows）

```powershell
scoop bucket add dhslegen https://github.com/dhslegen/scoop-bucket
scoop install base-code
```

### 方式三：go install（需 Go 1.21+）

```bash
go install github.com/dhslegen/base-code@latest
```

> 安装后二进制名即 `base-code`，与 `--help` 命令名、发行包名完全一致。
>
> **`command not found: base-code`？** `go install` 把二进制放在 `$(go env GOPATH)/bin`（默认 `~/go/bin`），需确保它在 PATH：
> ```bash
> echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.zshrc && source ~/.zshrc
> ```
> 用 Homebrew / Scoop 安装则无此问题。

### 方式四：本地构建

```bash
git clone https://github.com/dhslegen/base-code.git
cd base-code
go build -o base-code .
./base-code gen --help
```

---

## 用法

### 一行命令（无需配置文件，agent 友好）

必填项仅 `tables`、`base-package`、`db-name` 三项——且必填裁决的是 **flag 与配置文件合并后的生效值**，任一来源提供即可；其余（数据库连接、API 层标识等）均有约定默认值，可不写 `base-code.yaml` 直接执行（优先级：flag > 配置文件 > 约定默认值）：

```bash
# 最短命令（默认 6 层，不含 API/DTO 层）
base-code gen --tables it_user --base-package com.example.hello --db-name hello

# 生成全 14 层（含 Feign api/api-impl）
base-code gen --tables it_user --base-package com.example.hello --db-name hello --with-api
```

- `--with-api` 默认 `false`，即**默认不生成 API 层**；加此开关（或配置 `with-api: true`）才生成 `api`/`api-impl` 两层，凑齐全 14 层。
- 连接参数均有约定默认值：`--dialect` 缺省 `mysql`、`--db-host` 缺省 `127.0.0.1`、`--db-user` 缺省 `root`、`--db-port` 按方言派生（mysql→3306，postgresql→5432）；`--api-service-name`/`--api-base-path` 缺省从 `--base-package` 末段派生。
- flag 与配置文件混用时 flag 逐项覆盖文件值；显式 `--config` 指向的文件必须存在，未显式时默认 `base-code.yaml` 缺席即进入纯 flag 模式。
- 缺必填项时报错信息会给出可复制的完整命令样例，agent 读错误即可自修复。

### 命令格式

```
base-code gen --tables <表名,...> --base-package <包名> --db-name <库名> [flags]
```

flag 按用途分四组：生成目标、数据库连接、API 层、生成行为，详见下方「Flag 说明」；完整帮助见 `base-code gen --help`。

### Flag 说明

与 `base-code gen --help` 的四个分组一一对应。

#### 生成目标

| Flag | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--tables` | string | **必填**¹ | 逗号分隔的表名，如 `sys_user,sys_role` |
| `--base-package` | string | **必填**¹ | Java 基础包名 |
| `--output-root` | string | `./src/main/java` | Java 源文件输出根目录 |
| `--resources-root` | string | 由 output-root 推导 | mapper-xml 输出根目录 |

> ¹ **必填**指 flag 与配置文件合并后的生效值必须存在：flag 与配置文件二选一即可（flag 优先）。三个必填项均可写入配置文件（`tables` / `base-package` / `datasource.database`），此时命令行可一个 flag 都不传。

#### 数据库连接

| Flag | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--dialect` | string | `mysql` | SQL 方言：`mysql` 或 `postgresql` |
| `--db-host` | string | `127.0.0.1` | 数据库主机 |
| `--db-port` | int | 按方言 3306/5432 | 数据库端口 |
| `--db-user` | string | `root` | 数据库用户名 |
| `--db-password` | string | 空 | 数据库密码 |
| `--db-name` | string | **必填**¹ | 数据库名 |

#### API 层

| Flag | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--api-service-name` | string | base-package 末段 | `@FeignClient` 服务名 |
| `--api-base-path` | string | `/`+base-package 末段 | API 基础路径前缀 |
| `--with-api` | bool | `false` | 生成 API 层 `api`/`api-impl` 及其依赖的 DTO/converter（不加则仅 6 层后端核心，加则全 14 层） |

#### 生成行为

| Flag | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--sync-schema` | bool | `false` | 改表后只重新生成受表结构影响的层 |
| `--dry-run` | bool | `false` | 只打印生成代码到终端，不落盘 |
| `--config` | string | `base-code.yaml` | 配置文件路径，缺席时进入纯 flag 模式 |
| `--author` | string | 读 git config user.name | 代码 @author |
| `--use-jakarta` | bool | `true` | `true`=jakarta 包（Spring Boot 3+），`false`=javax 包 |
| `--date-type` | string | `modern` | `modern`=java.time.*，`legacy`=java.util.Date |
| `--auto-fill-insert` | string | `created_at,updated_at,created_by,updated_by` | 插入自动填充列，逗号分隔 |
| `--auto-fill-update` | string | `updated_at,updated_by` | 更新自动填充列，逗号分隔 |

### 最小配置文件（base-code.yaml）

```yaml
base-code:
  tables: [sys_user, sys_role]          # 生成目标表（必填；也可用 --tables 内联提供）
  base-package: com.example.demo        # Java 基础包名（必填）

  # with-api: true  # 默认不生成 API 层；配 true 生成全 14 层

  # 可选字段（缺省均有约定默认值）
  # output-root: ./src/main/java          # Java 源文件输出根目录；缺省 ./src/main/java
  # resources-root: ./src/main/resources  # mapper-xml 输出根；缺省由 output-root 推导
  # author: zhaowenhao                    # 代码 @author；缺省读 git config user.name
  # use-jakarta: true                     # true=jakarta 包（Spring Boot 3+），false=javax 包
  # date-type: modern                     # modern=java.time.*（默认），legacy=java.util.Date

  # API 层服务标识（可选；缺省从 base-package 末段派生：demo → demo、/demo）
  # api:
  #   service-name: demo-service           # @FeignClient 的服务名（注册中心应用名）
  #   base-path: /admin-api/demo           # 所有 API 端点的基础路径前缀

  datasource:
    database: demo                      # 数据库名（必填）
    # dialect: mysql                      # mysql 或 postgresql；缺省 mysql
    # host: 127.0.0.1                     # 缺省 127.0.0.1
    # port: 3306                          # 缺省按方言派生：mysql→3306，postgresql→5432
    # username: root                      # 缺省 root
    # password: secret                    # 缺省空

  # 自动填充列（@TableField(fill=...)），缺省约定如下
  # auto-fill:
  #   insert-columns: [created_at, updated_at, created_by, updated_by]
  #   update-columns: [updated_at, updated_by]
```

### 使用示例

```bash
# 生成 sys_user 表的默认 6 层后端核心（不含 API/DTO 层）
base-code gen --config base-code.yaml --tables sys_user

# 配置文件已含 tables 时，零 flag 直接执行（flag 提供的 --tables 会整体覆盖文件值）
base-code gen --config base-code.yaml

# 同时生成多张表
base-code gen --config base-code.yaml --tables "sys_user,sys_role,sys_menu"

# 改表后只重新生成受表结构影响的层（不覆盖 service/api 手写代码）
base-code gen --config base-code.yaml --tables sys_user --sync-schema

# 生成全 14 层（含 API/Feign 层）
base-code gen --config base-code.yaml --tables sys_user --with-api

# 预览生成内容（不落盘）
base-code gen --config base-code.yaml --tables sys_user --dry-run

# 用 PostgreSQL（覆盖配置文件中的方言）
base-code gen --config base-code.yaml --tables sys_user --dialect postgresql
```

---

## Shell 补全

**Homebrew 安装（v0.2.1+）自动装好 bash / zsh / fish 补全，无需任何操作。**

其他安装方式（go install / 本地构建）可用内置 `completion` 子命令手动生成，写入一个**确定在 `$fpath` 中**的目录（不要用 `${fpath[1]}`，其指向因环境而异）：

```bash
# zsh：Homebrew 环境的标准补全目录
base-code completion zsh > "$(brew --prefix)/share/zsh/site-functions/_base-code"

# zsh：oh-my-zsh 用户也可直接放 custom/completions（天然在 fpath 中）
base-code completion zsh > ~/.oh-my-zsh/custom/completions/_base-code

# 写入后重建补全缓存
rm -f ~/.zcompdump*; exec zsh

# bash
base-code completion bash > /etc/bash_completion.d/base-code

# fish
base-code completion fish > ~/.config/fish/completions/base-code.fish
```

> zsh 提示：`$(brew --prefix)/share/zsh/site-functions` 需在 `compinit` 之前进入 `FPATH` 才生效。若补全不工作，在 `~/.zshrc` 里 **source oh-my-zsh（或调用 compinit）之前**加入：
> `FPATH="$(brew --prefix)/share/zsh/site-functions:$FPATH"`

---

## 生成的 14 层一览

默认生成 6 层后端核心（`po`/`mapper`/`mapper-xml`/`service`/`service-impl`/`query`）；`api`/`api-impl` 及其依赖的 DTO/converter 需 `--with-api` 或 `with-api: true` 才会生成，凑齐全 14 层。

| 层 | 文件（以 `sys_user` 为例） | 说明 |
|----|--------------------------|------|
| `po` | `SysUser.java` | 实体类（@TableName + @TableId） |
| `mapper` | `SysUserMapper.java` | MyBatis-Plus Mapper 接口 |
| `mapper-xml` | `SysUserMapper.xml` | MyBatis XML ResultMap（落 resources） |
| `service` | `SysUserService.java` | Service 接口 |
| `service-impl` | `SysUserServiceImpl.java` | Service 实现类 |
| `query` | `SysUserQuery.java` | 条件查询对象 |
| `converter` | `SysUserConverter.java` | DTO ↔ PO 转换器 |
| `req-dto` | `SysUserReqDto.java` | 请求 DTO |
| `resp-dto` | `SysUserRespDto.java` | 响应 DTO |
| `query-req-dto` | `SysUserQueryReqDto.java` | 查询请求 DTO |
| `page-query-req-dto` | `SysUserPageQueryReqDto.java` | 分页查询请求 DTO |
| `update-by-query-req-dto` | `SysUserUpdateByQueryReqDto.java` | 按条件更新请求 DTO |
| `api` | `SysUserApi.java` | Feign RPC 接口 |
| `api-impl` | `SysUserApiImpl.java` | Feign RPC 实现 |

---

## 自包含

生成的全部 14 层代码**不依赖任何目标工程预置类**：`@FeignClient` 服务名与 API 基础路径由配置 `api:` 节内联进产物；`pageAll` 端点直接以 `current`/`size` 两个查询参数收参。拿到产物即可编译（中央组件依赖除外，见 pom 依赖说明）。
