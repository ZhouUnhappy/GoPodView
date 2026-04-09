# GoPodView

[English](README.md) | **中文**

Go 项目代码结构可视化工具，借鉴 Kubernetes 概念进行展示。

Go 源文件以 **Pod** 形式呈现，文件内部的声明（函数、结构体、接口、常量、变量）以 **Container** 形式呈现。文件间的 import 依赖关系以连线的方式在交互式图中渲染。点击任意 Container 即可在浏览器中直接查看其源代码。

## 截图

> 以下截图以 [eino-examples](https://github.com/cloudwego/eino-examples) 仓库作为示例项目。

### 全局 Pod 视图
项目中所有 Go 文件渲染为彩色圆点。圆点大小反映 Container 数量，颜色按 package 分组。

![全局视图](docs/readme/global-view.png)

### 聚焦视图
在左侧文件树中点击文件聚焦对应 Pod——只展示该 Pod 及其直接依赖，自动重新布局为清晰的思维导图树形结构。

![聚焦视图](docs/readme/focused-view.png)

### 展开视图
再次点击已聚焦的 Pod 展开——查看文件内所有 Container。Struct 的方法会归类在其接收者类型下。

![展开视图](docs/readme/expanded-view.png)

## 功能特性

- **文件树** — 左侧面板浏览项目目录，点击文件聚焦到对应 Pod（唯一修改聚焦的方式）
- **文件树搜索** — 实时搜索过滤文件
- **折叠全部** — 快速折叠文件树中所有展开的节点
- **URL 参数自动填充** — 项目路径自动从 URL 参数填充
- **Pod 依赖图** — 基于 Vue Flow 的交互式节点图（缩放、平移、拖拽）
- **聚焦模式** — 在文件树中选择文件，隔离显示对应 Pod 及其依赖，思维导图树形布局
- **展开模式** — 点击已聚焦的 Pod 展开查看 Container（func、struct、interface、const、var）；点击邻居 Pod 可内联展开
- **浮动代码标签页** — 将任意代码视图弹出为独立可拖拽的标签页，使用 Monaco Editor；支持同时打开多个标签页
- **内联代码** — 点击任意 Container 预览带 Go 语法高亮的源代码
- **动态编辑器高度** — Monaco 编辑器根据内容大小自动调整高度
- **多 Active Containers** — 支持跨不同 Pod 的多个 active containers
- **Struct 方法分组** — 带接收者的方法嵌套在对应的 struct/interface 下，点击可展开/收起
- **Pod 文件路径** — 展开的 Pod 卡片在头部显示完整文件路径
- **VSCode 式导航** — `Cmd+[` 后退、`Cmd+]` 前进、`Cmd+Click` 跳转到引用
- **URL 状态同步** — 当前项目、聚焦文件、视图层级、展开的 Pod 同步到 URL
- **Package 颜色分组** — 节点按 package 着色便于视觉识别
- **外部依赖** — 始终显示并按需懒加载以提升性能
- **固定缩放** — 导航操作仅平移画布，缩放由用户手动控制

## 技术栈

| 层 | 技术 |
|---|------|
| 后端 | Go (go/ast, go/parser), Gin |
| 前端 | Vue 3, TypeScript, Vite |
| 图可视化 | Vue Flow |
| 代码查看 | Monaco Editor |
| UI 组件 | Element Plus |
| 状态管理 | Pinia |

## 快速开始

```bash
# 启动后端和前端（后台运行）
./dev.sh start [--project <path>] [--go_port <port>] [--vite_port <port>] [--log <dir>]

# 重启服务
./dev.sh restart

# 停止服务
./dev.sh stop
```

示例：
```bash
./dev.sh start --project /path/to/go/project --go_port 8080 --vite_port 5173
```

浏览器打开 http://localhost:5173 即可使用。

也可以在界面左侧输入框中输入项目路径后点击 **Load** 加载。

## API 接口

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/project` | POST | 设置要分析的项目路径 |
| `/api/filetree` | GET | 获取项目文件树 |
| `/api/pods` | GET | 获取所有 Pod 及依赖边 |
| `/api/pod/:path` | GET | 获取单个 Pod 详情 |
| `/api/containers/:path` | GET | 获取 Pod 内所有 Container（含源码） |
| `/api/container/:path?name=` | GET | 获取单个 Container |
| `/api/reference-target/:path` | GET | 获取外部引用的目标 Pod（懒加载） |
| `/api/dependencies/:path?depth=` | GET | 获取 N 级依赖 |

## 项目结构

```
GoPodView/
├── backend/                 # Go 后端
│   ├── main.go              # 入口
│   ├── internal/
│   │   ├── parser/          # AST 解析引擎
│   │   ├── model/           # 数据模型 (Pod, Container)
│   │   └── api/             # HTTP 处理器 + 路由
│   └── go.mod
├── frontend/                # Vue 3 前端
│   ├── src/
│   │   ├── components/
│   │   │   ├── PodGraph/    # Vue Flow 图 + 自定义 PodNode
│   │   │   ├── FileTree/    # 侧边栏文件树
│   │   │   ├── CodeView/    # Monaco Editor 封装
│   │   │   ├── Breadcrumb/  # 面包屑导航
│   │   │   └── Controls/    # UI 控件
│   │   ├── stores/          # Pinia 状态管理
│   │   ├── api/             # HTTP 客户端
│   │   └── types/           # TypeScript 类型定义
│   └── package.json
├── dev.sh                   # 开发启动脚本
├── Makefile
└── README.md
```

## 许可证

[Apache-2.0](LICENSE)
