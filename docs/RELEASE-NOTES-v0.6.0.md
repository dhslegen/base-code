# v0.6.0 发布说明

## 亮点：`base-code init` 一键生成配置模版

从零手写 `base-code.yaml` 门槛高——改是容易的。新命令在当前目录生成教学型全量模版：

```bash
base-code init    # 生成 ./base-code.yaml（已存在时报错，--force 覆盖）
```

- **必填三项已填可运行示例值**：`tables` / `base-package` / `datasource.database`
- **其余配置全部以注释列出并标注约定默认值**——需要什么就取消注释改什么
- 编辑后直接 `base-code gen`（`gen` 默认读取 `./base-code.yaml`，无需 `--config`）

三步上手：**init → 编辑 → gen**。

## 其他改进

- 缺参报错在原有「可复制内联命令样例」之外，追加第二条自救通路提示：「也可先执行 `base-code init` 生成配置模版」。
- 模版有效性由测试机械把关：产出文件必须能通过真实配置加载与必填校验，未来配置演进不会让模版悄悄失效。

## 兼容性

纯新增命令，`gen` 行为无任何变化。

## 升级

```bash
brew update && brew upgrade base-code   # macOS / Linux
scoop update base-code                  # Windows
```
