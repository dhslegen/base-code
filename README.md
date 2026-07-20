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

  # API 层服务标识（可选；缺省从 base-package 末段派生：demo → demo、/demo）
  # api:
  #   service-name: demo-service           # @FeignClient 的服务名（注册中心应用名）
  #   base-path: /admin-api/demo           # 所有 API 端点的基础路径前缀

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
