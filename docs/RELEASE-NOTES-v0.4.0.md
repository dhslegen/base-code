# base-code v0.4.0 — CLI 专业化改版

本版对命令行界面做了一次面向「专业、简洁、agent 友好」的重构。**包含破坏性变更，升级前请阅读迁移说明。**

## ⚠️ 破坏性变更（升级必读）

### 1. 默认不再生成 API 层（14 层 → 6 层）

**这是静默的行为变化，最需注意。** v0.3.x 默认生成全 14 层；v0.4.0 默认只生成 **6 层后端核心**：

```
po  mapper  mapper-xml  service  service-impl  query
```

不再默认生成的 8 层：`api`、`api-impl`、`converter`、`req-dto`、`resp-dto`、`query-req-dto`、`page-query-req-dto`、`update-by-query-req-dto`。

**原因**：DTO 五层与 converter 只被 `api`/`api-impl` 消费；不生成 API 层时它们没有消费者，属于死代码。默认砍掉使产物自洽。

**恢复全 14 层（二选一）**：
- 命令行加 `--with-api`
- 或配置文件加顶层键 `with-api: true`

> 老用户既有 `base-code.yaml` **不含** `with-api` 键，升级后默认只出 6 层。若你的工程依赖 Feign/DTO 层，**务必**加上 `with-api: true`，否则会缺文件（表现为编译期找不到 `XxxApi`/`XxxReqDto`）。

### 2. Flag 改名（旧名已删除）

| 旧 flag（已删） | 新 flag | 说明 |
|---|---|---|
| `--without-api` | `--with-api` | **语义反转**：旧的「不要 API」→ 新的「要 API」，缺省 false |
| `--only-table-modify` | `--sync-schema` | 改表后同步受影响层 |
| `--service-name` | `--api-service-name` | @FeignClient 服务名 |
| `--base-path` | `--api-base-path` | API 基础路径前缀 |

旧名会**响亮失败**（`unknown flag`），据此更新脚本即可。

### 3. 必填项收缩为 3 项

现在仅 `--tables`、`--base-package`、`--db-name` 必填。其余连接参数有约定默认值（见下），缺失时给出可复制的三参最短命令样例。

## ✨ 改进

### 连接参数约定默认值下沉

| 参数 | 默认值 |
|---|---|
| `--dialect` | `mysql` |
| `--db-host` | `127.0.0.1` |
| `--db-port` | 按方言派生（mysql 3306 / postgresql 5432） |
| `--db-user` | `root` |
| `--db-password` | 空 |
| `--output-root` | `./src/main/java` |

本地开发最短命令：

```bash
base-code gen --tables it_user --base-package com.example.hello --db-name hello-mysql
```

优先级不变：**显式 flag > 配置文件 > 约定默认值**。配置文件里的显式值不会被这些默认值覆盖。

### 帮助文本对标 openapi-generator

`base-code gen --help` 现按语义分组（生成目标 / 数据库连接 / API 层 / 生成行为），每个 flag 标注真实默认值，并附 4 条典型场景示例。

## 📌 `--sync-schema` 的层集合说明

`--sync-schema` 只重新生成「受表结构影响」的层，且与 `--with-api` 的取值联动：

- 默认（API-less 工程）：`po`、`mapper-xml`、`query` 共 **3 层**
- 配 `--with-api`（含 DTO 的工程）：加上 `req-dto`、`resp-dto`、`query-req-dto` 共 **6 层**

> 注意：若你的工程用 `--with-api` 生成过 14 层，日后单跑 `--sync-schema` 请同样带上 `--with-api`，否则 DTO 层不会被刷新而可能过期。

## 安装 / 升级

```bash
brew upgrade base-code        # Homebrew
scoop update base-code        # Scoop
go install github.com/dhslegen/base-code@latest
```
