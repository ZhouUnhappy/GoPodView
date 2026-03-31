
# 外部依赖解析功能实现计划

## 现状分析

当前后端只解析项目根目录内的 `.go` 文件，外部依赖被完全忽略：
- `isExternal()` 始终返回 `false`（占位实现）
- `dirToImportPath()` 直接返回相对路径，未拼接 module 名
- Scanner 只递归项目根目录
- 依赖图中只有项目内部的 Pod 和 Edge

## 核心设计思路

1. 解析项目 `go.mod` 获取 module 名和依赖列表
2. 通过 `GOMODCACHE` 定位外部依赖源码
3. 仅解析项目**直接引用**的外部包（非传递依赖），控制范围
4. 外部 Pod 与内部 Pod 使用相同的数据结构，通过 `isExternal` 字段区分
5. 前端通过不同视觉样式展示外部 Pod，支持展开查看 Container 和源码

---

## Task 1: 创建计划文档

在 `docs/external-dependency-design.md` 中记录本计划。

---

## Task 2: go.mod 解析器

**新建文件**: `backend/internal/parser/gomod.go`

功能：
- 解析项目根目录的 `go.mod` 文件
- 提取 module 名（如 `github.com/user/project`）
- 提取 require 列表（module path + version）
- 提取 replace 指令（处理本地替换）
- 获取 `GOMODCACHE` 路径（通过 `go env GOMODCACHE` 或环境变量）

关键结构：
```go
type ModuleInfo struct {
    ModuleName   string              // go.mod 中的 module 名
    GoVersion    string              // go 版本
    Requirements []ModRequirement    // require 列表
    Replaces     map[string]string   // replace 映射
    ModCachePath string              // GOMODCACHE 路径
}

type ModRequirement struct {
    Path    string  // e.g., "github.com/gin-gonic/gin"
    Version string  // e.g., "v1.10.0"
}
```

关键方法：
- `ParseGoMod(projectRoot string) (*ModuleInfo, error)` — 解析 go.mod
- `(m *ModuleInfo) ResolveModulePath(importPath string) (fsPath string, ok bool)` — 将 import path 解析为 GOMODCACHE 中的文件系统路径

路径解析规则：
```
importPath: "github.com/gin-gonic/gin/binding"
→ module: "github.com/gin-gonic/gin" @ "v1.10.0"
→ fsPath: "$GOMODCACHE/github.com/gin-gonic/gin@v1.10.0/binding/"
```

---

## Task 3: 修复内部 import 解析

**修改文件**: `backend/internal/parser/analyzer.go`

3a. 为 `Analyzer` 添加 `ModuleInfo` 字段：
```go
type Analyzer struct {
    parser     *ProjectParser
    modInfo    *ModuleInfo           // 新增
    importMap  map[string]string
    pkgToPods  map[string][]string
}
```

3b. 修复 `dirToImportPath()`：
```go
func (a *Analyzer) dirToImportPath(dir string) string {
    if a.modInfo == nil || a.modInfo.ModuleName == "" {
        return dir
    }
    if dir == "." || dir == "" {
        return a.modInfo.ModuleName
    }
    return a.modInfo.ModuleName + "/" + dir
}
```

3c. 正确实现 `isExternal()`：
```go
func (a *Analyzer) isExternal(importPath string) bool {
    if a.modInfo == nil || a.modInfo.ModuleName == "" {
        return false
    }
    return !strings.HasPrefix(importPath, a.modInfo.ModuleName)
}
```
注意：`isExternal` 从独立函数改为 `Analyzer` 的方法，因为需要访问 `modInfo`。

3d. 修复 `resolveImport()` 支持完整 module import path：
- 先尝试用完整 import path 匹配 `importMap`
- 如果 import path 以 module 名为前缀，截取后缀作为相对路径查找 `pkgToPods`

---

## Task 4: 外部依赖扫描与解析

**修改文件**: `backend/internal/parser/scanner.go`, `backend/internal/parser/parser.go`, `backend/internal/parser/analyzer.go`

4a. Scanner 新增方法：
```go
// ScanExternalPackage 扫描 GOMODCACHE 中某个包目录的 .go 文件
func ScanExternalPackage(packageDir string) ([]string, error)
```
- 只扫描目标包目录（非递归），收集 `.go` 文件
- 跳过 `_test.go` 文件（外部测试文件无需展示）

4b. Parser 支持外部文件解析：
- `ParseFile` 当前使用 `filepath.Join(p.Root, relPath)` 拼接路径
- 新增 `ParseExternalFile(absPath, displayPath string)` 方法
  - `absPath`: GOMODCACHE 中的实际路径
  - `displayPath`: 用于展示的路径（如 `github.com/gin-gonic/gin/gin.go`）
  - Pod 的 `IsExternal` 设为 `true`
  - Pod 的 `ModulePath` 设为模块路径

4c. Analyzer 中集成外部解析：
- 在 `AnalyzeAll()` 中，解析完内部文件后：
  1. 收集所有内部 Pod 的 imports 中属于外部的 import path
  2. 按 import path 去重
  3. 通过 `ModuleInfo.ResolveModulePath()` 定位每个外部包在 GOMODCACHE 中的路径
  4. 调用 `ScanExternalPackage()` + `ParseExternalFile()` 解析外部包
  5. 将外部 Pod 加入 `parser.Pods`
  6. 在 `buildPodDependencies()` 中建立内部 Pod → 外部 Pod 的依赖边

---

## Task 5: 数据模型扩展

**修改文件**: `backend/internal/model/pod.go`

```go
type Pod struct {
    Path       string       `json:"path"`
    Package    string       `json:"package"`
    FileName   string       `json:"fileName"`
    Imports    []string     `json:"imports"`
    Containers []*Container `json:"containers"`
    DependsOn  []string     `json:"dependsOn"`
    DependedBy []string     `json:"dependedBy"`
    IsExternal bool         `json:"isExternal"`    // 新增：是否为外部依赖
    ModulePath string       `json:"modulePath,omitempty"` // 新增：所属模块路径
}
```

---

## Task 6: 前端类型与 API 适配

**修改文件**: `frontend/src/types/index.ts`

```typescript
export interface Pod {
  path: string
  package: string
  fileName: string
  imports: string[]
  containers: Container[]
  dependsOn: string[]
  dependedBy: string[]
  isExternal?: boolean      // 新增
  modulePath?: string       // 新增
}
```

---

## Task 7: 前端 PodNode 视觉区分

**修改文件**: `frontend/src/components/PodGraph/PodNode.vue`

- Dot 模式：外部 Pod 使用虚线边框 + 不同形状或颜色标记
- Card 模式：外部 Pod 卡片 header 显示 `modulePath`，使用不同的边框颜色（如灰色/紫色）
- Card header 示例：`gin-gonic/gin | context.go | github.com/gin-gonic/gin/context.go`
- 添加 "External" 小标签/badge

---

## Task 8: 前端 Store 与图交互适配

**修改文件**: `frontend/src/stores/project.ts`, `frontend/src/components/PodGraph/PodGraph.vue`

- `podMap` 和 `edges` 正常包含外部 Pod
- 外部 Pod 支持展开查看 Container 和源码（与内部 Pod 一致）
- 外部 Pod 不出现在文件树中（文件树仅展示项目内部文件）
- PodGraph 布局算法无需大改，外部 Pod 作为普通节点参与布局
- 颜色分配：外部 Pod 使用统一的灰色调，内部 Pod 保持按 package 着色

---

## Task 9: 外部依赖显示开关

**修改文件**: `frontend/src/stores/project.ts`, `frontend/src/components/Controls/DepthControl.vue`（或新建控件）

- Store 新增 `showExternalDeps: boolean` 状态
- 图中根据该状态过滤外部 Pod 和对应的 Edge
- 在控制区域添加开关按钮（默认开启）

---

## 风险与注意事项

1. **性能**：大型项目可能有数百个外部依赖包，每个包含几十个 .go 文件。需要只解析被直接引用的包，不做传递解析。
2. **GOMODCACHE 不存在**：用户可能未执行 `go mod download`，需要优雅降级（提示用户先运行 `go mod tidy`）。
3. **replace 指令**：go.mod 中的 `replace` 指令会改变模块的实际路径，需要正确处理。
4. **vendor 模式**：如果项目使用 `vendor/` 目录，应优先从 vendor 读取而非 GOMODCACHE。当前 vendor 被 skip，需要调整。
5. **GOMODCACHE 文件只读**：这些文件权限为 `r--r--r--`，只读访问没问题。

---

## 实施顺序

```
Task 1 (计划文档)
    ↓
Task 2 (go.mod 解析器)
    ↓
Task 3 (修复内部 import 解析) — 依赖 Task 2
    ↓
Task 5 (数据模型扩展) — 可与 Task 3 并行
    ↓
Task 4 (外部依赖扫描与解析) — 依赖 Task 2, 3, 5
    ↓
Task 6 (前端类型适配) — 依赖 Task 5
    ↓
Task 7 (PodNode 视觉区分) — 依赖 Task 6
Task 8 (Store 与图交互适配) — 依赖 Task 6，可与 Task 7 并行
Task 9 (外部依赖显示开关) — 依赖 Task 8
```
