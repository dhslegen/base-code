# v0.5.0 发布说明

## 亮点：必填校验下沉至「合并后生效配置」

必填不再是 flag 层面的概念。`tables`、`base-package`、`db-name` 三个必填项由 **flag 与配置文件合并后的生效值**统一裁决——任一来源提供即可（flag 优先），命令行层面彻底没有必填限制。

### tables 首次获得配置态

```yaml
base-code:
  tables: [sys_user, sys_role]   # 列表形态；--tables（CSV）显式提供时整体覆盖
  base-package: com.example.demo
  datasource:
    database: demo
```

配置文件写全三个必填项后，可**零 flag 直接执行**：

```bash
base-code gen --config base-code.yaml
```

### 行为变化

| 场景 | v0.4.x | v0.5.0 |
|---|---|---|
| 配置文件含 `tables`，命令行不传 `--tables` | cobra 报 `required flag(s) "tables" not set`，配置永远无法生效 | 正常生成 |
| flag 与配置文件都缺 `tables` | cobra 解析层报错 | 合并后校验报「缺少必填配置：--tables」，附可复制的最短命令样例 |
| `--tables` 与配置文件同时提供 | 仅 flag 生效（配置态不存在） | flag 整体覆盖文件值（与其余配置项的覆盖语义一致） |

### 兼容性

- 既有用法完全兼容：继续用 `--tables` 内联传表名的命令无任何变化。
- 无破坏性变更；报错文案从 cobra 英文错误变为中文「缺少必填配置」提示（面向 agent 自修复）。

## 升级

```bash
brew update && brew upgrade base-code   # macOS / Linux
scoop update base-code                  # Windows
```
