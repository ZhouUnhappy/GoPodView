# 外部依赖解析设计

## 目标

为 Go 项目补充外部依赖 Pod 的解析与展示能力，同时保持当前内部文件树、源码查看和图布局能力不变。

## 实现要点

1. 后端在分析项目时读取根目录 `go.mod`。
2. 通过 `go mod edit -json` 提取 `module`、`require`、`replace` 信息。
3. 通过 `go env GOMODCACHE` 获取模块缓存目录，并结合 `replace` 解析外部包真实源码路径。
4. 初始化阶段不解析外部包；用户查看 struct / func 代码时只展示引用列表，不加载外部 Pod。
5. 只有在用户点击 external ref-target 时，才按该引用的 import path 懒加载对应外部包，并且图里只补入命中 target 所在的那个 pod。
6. 外部 Pod 复用现有 `Pod` / `Container` 结构，使用 `isExternal` 和 `modulePath` 标记。
7. 外部 Pod 的展示路径统一使用 import path 形式，例如 `github.com/gin-gonic/gin/context.go`，便于前后端复用现有按路径索引的逻辑。
8. 前端图中外部 Pod 使用统一灰色系和额外 badge 展示，并在引用列表中对 external ref 单独打标。

## 和原计划的差异

- `replace` 不能只记录字符串映射，必须区分本地目录替换和远程版本替换，否则无法正确定位源码目录。
- 外部包路径解析虽然最终仍会落到 `GOMODCACHE`，但实现上先解析模块元信息，再计算实际目录，这样可以兼容 `replace`。
- 依赖边仅在点击 external ref-target 后，补充“当前 Pod -> 目标定义所在 pod”，避免把同包其他 `.go` 文件一起打进图里。

## 降级行为

- 如果项目根目录不存在 `go.mod`，系统继续解析内部文件，但不会补充外部依赖。
- 如果模块缓存中不存在目标依赖源码，对应外部包会被跳过，不阻塞整个项目分析。
