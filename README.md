# base-code-go

数据库代码生成器（Go 版）。详见 ../base-code/docs/superpowers/specs 设计稿。

## 生成 API 层的工程级前置依赖

生成的 `api` / `api-impl` 层引用以下**目标工程需自行预置**的公共类（base-code 不生成它们，每个工程一份，与源工程约定一致）：

- `{base-package}.constants.ApiConstants` — 提供 `NAME`（Feign 服务名）与 `PREFIX`（API 路径前缀），用于 `@FeignClient` 与各端点的 `PREFIX`。
- `{base-package}.model.dto.req.PageQueryReqDto` — 非表相关的通用分页请求 DTO（仅含 current/size），是 `pageAll` 端点的入参。注意：各表生成的 `XxxPageQueryReqDto` 继承自该表的 `XxxQueryReqDto`，与这个通用基类是两回事。

未预置上述类时，`api`/`api-impl` 层产物无法编译（这是与源工程一致的既有契约）。
