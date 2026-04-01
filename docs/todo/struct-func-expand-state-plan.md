# Struct/Func 展开状态支持计划

在 Pod 展开视图下，记录并恢复每个 Container（struct/func/interface）的展开状态，使其在页面刷新、导航后退/前进、URL 分享时保持一致。

---

## 现状与目标

**现状**：URL 参数 `?expanded=pod1,pod2` 仅记录 Pod 级别的展开状态，Container 展开状态丢失。

**目标**：实现对 Container 级别展开状态的完整支持：
1. Store 维护每个 Pod 下展开的 Container 集合
2. URL 参数同步 Container 展开状态
3. 导航历史包含 Container 展开信息
4. 从 URL 恢复时还原 Container 状态

---

## 核心设计

### 数据结构

每个 Pod 下需要记录**两层独立的展开状态**：

1. **`expandedGroups`** — 展开的 struct/interface 组（显示其 methods 列表）
2. **`activeContainers`** — 显示代码的 Container（可以是 struct、method 或 standalone func）

这两者是正交关系：可以展开 struct 组但不显示其代码，也可以展开组同时显示某个 method 的代码。

```typescript
interface PodContainerState {
  expandedGroups: Set<string>     // struct/interface name 集合
  activeContainer: string | null    // 显示代码的 container name
}

const expandedContainers = ref<Record<string, PodContainerState>>({})
// key: Pod 路径，value: 该 Pod 的 Container 展开状态
```

扩展 `NavigationEntry` 接口：

```typescript
export interface NavigationEntry {
  level: ViewLevel
  podPath?: string
  containerName?: string
  expandedPods?: string[]
  // 每个 Pod 的展开组集合
  expandedGroups?: Record<string, string[]>
  // 每个 Pod 当前显示代码的 Container
  activeContainers?: Record<string, string>
}
```

### URL 参数设计

新增参数 `containers`，格式为逗号分隔的组合：

```
?project=/path/to/project&file=foo/bar.go&level=expanded&expanded=pod1,pod2&groups=pod1:StructA,pod2:StructC&active=pod1:FuncB,pod2:StructC
```

- `groups` — 展开的 struct/interface 组（显示 methods 列表）
- `active` — 显示代码的 Container

注意：`podPath` 需要 `encodeURIComponent` 处理以避免冒号冲突。

---

## Task 分解

### Task 1: 类型定义扩展

**文件**: `frontend/src/types/index.ts`

- 为 `NavigationEntry` 添加 `expandedContainers?: Record<string, string[]>`

---

### Task 2: Store 状态管理

**文件**: `frontend/src/stores/project.ts`

**新增状态**:
```typescript
interface PodContainerState {
  expandedGroups: Set<string>     // struct/interface name 集合
  activeContainer: string | null    // 显示代码的 container name
}

const containerStates = ref<Record<string, PodContainerState>>({})
// key: Pod 路径，value: 该 Pod 的 Container 展开状态
```

**核心方法**:
- `expandGroup(podPath: string, groupName: string)` — 展开 struct/interface 组
- `collapseGroup(podPath: string, groupName: string)` — 折叠组
- `activateContainer(podPath: string, containerName: string)` — 设置显示代码的 Container
- `deactivateContainer(podPath: string)` — 关闭代码显示
- `isGroupExpanded(podPath: string, groupName: string): boolean`
- `isContainerActive(podPath: string, containerName: string): boolean`
- `snapshotContainerState(): { expandedGroups: Record<string, string[]>, activeContainers: Record<string, string | null> }`
- `restoreContainerState(snapshot)` — 从快照恢复两层状态

**集成点**:
- `expandPod()` / `collapseInlinePod()` — pushNavigation 时包含两层状态快照
- `applyNavigation()` — 从 entry 恢复 `expandedGroups` 和 `activeContainers`
- `resetView()` / `focusPod()` — 清空所有 Pod 的 Container 状态

---

### Task 3: URL 同步

**文件**: `frontend/src/stores/project.ts`

**syncUrlState()**:
- 当 `viewLevel === 'expanded'` 时，将两层状态序列化为 URL 参数
- `groups` 参数：`podPath:groupName` 格式，逗号分隔
- `active` 参数：`podPath:containerName` 格式（仅记录当前显示代码的 Container）

**watch 依赖**:
- 在现有 watch 数组中添加 `JSON.stringify(snapshotExpandedContainers())` 依赖项

---

### Task 4: URL 恢复

**文件**: `frontend/src/stores/project.ts`

**restoreFromUrl()**:
- 解析 `groups` 和 `active` URL 参数
- `groups` 反序列化：解析 `podPath:groupName`，构建 `expandedGroups` 映射
- `active` 反序列化：解析 `podPath:containerName`，构建 `activeContainers` 映射
- 验证 Pod 存在性后调用 `restoreContainerState()`

---

### Task 5: PodNode 集成

**文件**: `frontend/src/components/PodGraph/PodNode.vue`

当前 PodNode 有两层本地状态：
1. `expandedGroup` — 展开的 struct/interface 组（显示 methods 列表）
2. `activeContainer` — 显示代码编辑器的 Container

**改造方向**:
- `handleGroupClick()` 中调用 `store.expandGroup()` / `store.collapseGroup()`
- `toggleCodeView()` 中调用 `store.activateContainer()` / `store.deactivateContainer()`
- 通过 `store.isGroupExpanded()` 和 `store.isContainerActive()` 判断状态
- 从 Store 状态初始化组件的本地 `expandedGroup` 和 `activeContainer`

---

### Task 6: 优化与边界处理

**导航历史频率**:
- 建议只在激活 Container（显示代码）时记录导航历史
- 普通展开/折叠组不单独 pushNavigation，避免历史记录爆炸

**URL 长度控制**:
- `groups` 可能包含多个组的展开状态
- `active` 通常只有 0-1 个（当前显示代码的 Container）
- 如果展开大量组，URL 可能过长，后续可考虑限制数量

**恢复时验证**:
- 恢复前验证 Pod 是否存在于 `podMap`
- 忽略无效的 Container 名称

---

## 风险与注意事项

1. **Map 响应式陷阱**：Vue 3 对 `Map` 的响应式追踪不如 `Record` 可靠，建议用后者

2. **URL 长度限制**：大量 Container 展开时 URL 可能过长，后续可考虑限制最大数量

3. **两层展开状态**：PodNode 既有 `expandedGroup`（展开 methods 列表）又有 `activeContainer`（显示代码），需要分别独立管理

4. **向后兼容**：旧 URL 不含 `containers` 参数时应正常降级加载

---

## 实施顺序

```
Task 1 (类型定义) — 基础工作
    ↓
Task 2 (Store 状态管理) — 核心逻辑
    ↓
Task 3 (URL 同步) — 依赖 Task 2
    ↓
Task 4 (URL 恢复) — 依赖 Task 2, 3
    ↓
Task 5 (PodNode 集成) — 用户交互，依赖 Task 2
    ↓
Task 6 (优化与边界处理) — 收尾工作
```

---

## 测试要点

1. 展开单个 Container，刷新页面，确认保持展开
2. 展开多个 Pod 的多个 Container，确认 URL 参数格式正确
3. 后退/前进导航，确认 Container 状态正确恢复
4. 复制 URL 在新标签页打开，确认状态一致
5. 聚焦不同 Pod，确认 Container 状态清空
6. 不带 `containers` 参数的旧 URL 正常加载
