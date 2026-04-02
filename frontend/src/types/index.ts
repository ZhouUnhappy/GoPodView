export type ContainerType = 'func' | 'struct' | 'interface' | 'const' | 'var'
export type ReferenceType = 'call' | 'type_ref' | 'embed'

export interface Reference {
  containerName: string
  podPath: string
  type: ReferenceType
}

export interface Container {
  name: string
  type: ContainerType
  pod: string
  startLine: number
  endLine: number
  signature: string
  sourceCode?: string
  references: Reference[]
}

export interface Pod {
  path: string
  package: string
  fileName: string
  imports: string[]
  containers: Container[]
  dependsOn: string[]
  dependedBy: string[]
}

export interface PodEdge {
  source: string
  target: string
}

export interface FileTreeNode {
  name: string
  path: string
  isDir: boolean
  children?: FileTreeNode[]
}

export interface PodsResponse {
  pods: Pod[]
  edges: PodEdge[]
}

export interface DependenciesResponse {
  root: string
  depth: number
  pods: Pod[]
  edges: PodEdge[]
}

export type ViewLevel = 'global' | 'focused' | 'expanded' | 'code'

export interface NavigationEntry {
  level: ViewLevel
  podPath?: string
  containerName?: string
  expandedPods?: string[]
  expandedGroups?: Record<string, string[]>
  activeContainers?: Record<string, string[]>
}

export interface FloatingTab {
  id: string
  title: string
  signature: string
  sourceCode: string
  podPath: string
  containerName: string
  x: number
  y: number
}
