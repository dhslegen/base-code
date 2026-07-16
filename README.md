# base-code

[![Release](https://img.shields.io/github/v/release/dhslegen/base-code?sort=semver&color=brightgreen)](https://github.com/dhslegen/base-code/releases)
[![License](https://img.shields.io/github/license/dhslegen/base-code?color=blue)](LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/dhslegen/base-code)](go.mod)
![Platforms](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey)

数据库代码生成器（Go 版）。连接 MySQL / PostgreSQL，扫描表结构，按约定生成 MyBatis-Plus 分层 Java 代码（全 14 层）。

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

### 命令格式

```
base-code gen \
  --config base-code.yaml \
  --tables <逗号分隔的表名> \
  [--dialect mysql|postgresql] \
  [--without-api] \
  [--only-table-modify] \
  [--dry-run]
```

### Flag 说明

| Flag | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--config` | string | `base-code.yaml` | 配置文件路径 |
| `--tables` | string | **必填** | 逗号分隔的表名，如 `sys_user,sys_role` |
| `--dialect` | string | （使用配置文件中的值） | 覆盖配置文件中的 SQL 方言（`mysql` 或 `postgresql`） |
| `--without-api` | bool | `false` | 不生成 API 相关层，仅保留后端内部层 |
| `--only-table-modify` | bool | `false` | 仅生成改表影响的层（用于改列后局部重生成） |
| `--dry-run` | bool | `false` | 只打印生成代码到终端，不落盘 |

### 最小配置文件（base-code.yaml）

```yaml
base-code:
  base-package: com.example.demo        # Java 基础包名（必填）
  output-root: ./src/main/java          # Java 源文件输出根目录（必填）

  # 可选字段（缺省均有约定默认值）
  # resources-root: ./src/main/resources  # mapper-xml 输出根；缺省由 output-root 推导
  # author: zhaowenhao                    # 代码 @author；缺省读 git config user.name
  # use-jakarta: true                     # true=jakarta 包（Spring Boot 3+），false=javax 包
  # date-type: modern                     # modern=java.time.*（默认），legacy=java.util.Date

  datasource:
    dialect: mysql                      # mysql 或 postgresql
    host: localhost
    port: 3306
    username: root
    password: secret
    database: demo

  # 自动填充列（@TableField(fill=...)），缺省约定如下
  # auto-fill:
  #   insert-columns: [created_at, updated_at, created_by, updated_by]
  #   update-columns: [updated_at, updated_by]
```

### 使用示例

```bash
# 生成 sys_user 表的全 14 层
base-code gen --config base-code.yaml --tables sys_user

# 同时生成多张表
base-code gen --config base-code.yaml --tables "sys_user,sys_role,sys_menu"

# 改列后只重新生成受影响的 6 层（不覆盖 service/api 手写代码）
base-code gen --config base-code.yaml --tables sys_user --only-table-modify

# 不生成 API/Feign 层（纯后端服务）
base-code gen --config base-code.yaml --tables sys_user --without-api

# 预览生成内容（不落盘）
base-code gen --config base-code.yaml --tables sys_user --dry-run

# 用 PostgreSQL（覆盖配置文件中的方言）
base-code gen --config base-code.yaml --tables sys_user --dialect postgresql
```

---

## Shell 补全

cobra 内置 `completion` 子命令，支持 bash / zsh / fish / PowerShell。

```bash
# zsh
base-code completion zsh > "${fpath[1]}/_base-code"

# bash
base-code completion bash > /etc/bash_completion.d/base-code

# fish
base-code completion fish > ~/.config/fish/completions/base-code.fish
```

---

## 生成的 14 层一览

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

## 生成 API 层的工程级前置依赖

生成的 `api` / `api-impl` 层引用以下**目标工程需自行预置**的公共类（base-code 不生成它们，每个工程一份，与源工程约定一致）：

- `{base-package}.constants.ApiConstants` — 提供 `NAME`（Feign 服务名）与 `PREFIX`（API 路径前缀），用于 `@FeignClient` 与各端点的 `PREFIX`。
- `{base-package}.model.dto.req.PageQueryReqDto` — 非表相关的通用分页请求 DTO（仅含 current/size），是 `pageAll` 端点的入参。注意：各表生成的 `XxxPageQueryReqDto` 继承自该表的 `XxxQueryReqDto`，与这个通用基类是两回事。

未预置上述类时，`api`/`api-impl` 层产物无法编译（这是与源工程一致的既有契约）。
