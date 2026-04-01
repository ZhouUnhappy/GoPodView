<script setup lang="ts">
import { computed, markRaw } from 'vue'
import { VueFlow, useVueFlow } from '@vue-flow/core'
import { Background } from '@vue-flow/background'
import { Controls } from '@vue-flow/controls'
import '@vue-flow/core/dist/style.css'
import '@vue-flow/core/dist/theme-default.css'
import '@vue-flow/controls/dist/style.css'
import PodNode from './PodNode.vue'
import FloatingCodeTab from './FloatingCodeTab.vue'
import { useProjectStore } from '../../stores/project'
import type { Pod } from '../../types'

const store = useProjectStore()
const { getNodes } = useVueFlow()

type PodPosition = { x: number; y: number }
type PodSize = { width: number; height: number }

interface FocusedContext {
  visible: Set<string>
  adjacency: Map<string, Set<string>>
}

interface BranchTree {
  id: string
  children: BranchTree[]
  span: number
}

const nodeTypes = {
  pod: markRaw(PodNode),
} as Record<string, any>

const PALETTE = [
  '#409eff', '#67c23a', '#e6a23c', '#f56c6c', '#909399',
  '#9b59b6', '#1abc9c', '#e67e22', '#3498db', '#e74c3c',
  '#2ecc71', '#f39c12', '#8e44ad', '#16a085', '#d35400',
]

const pkgColorMap = computed(() => {
  const map = new Map<string, string>()
  let idx = 0
  for (const pod of store.pods) {
    if (!map.has(pod.package)) {
      map.set(pod.package, PALETTE[idx % PALETTE.length])
      idx++
    }
  }
  return map
})

const measuredNodeSizes = computed(() => {
  const map = new Map<string, PodSize>()
  for (const node of getNodes.value) {
    const width = node.dimensions?.width ?? 0
    const height = node.dimensions?.height ?? 0
    if (width > 0 && height > 0) {
      map.set(node.id, { width, height })
    }
  }
  return map
})

const focusedContext = computed<FocusedContext | null>(() => {
  if (store.viewLevel === 'global' || !store.focusedPodPath) {
    return null
  }
  return buildFocusedContext(store.focusedPodPath, store.pods, store.edges, store.expandedPods)
})

const visiblePodPaths = computed(() => {
  if (!focusedContext.value) {
    return new Set(store.pods.map((p) => p.path))
  }
  return focusedContext.value.visible
})

const flowNodes = computed(() => {
  if (!store.pods.length) return []

  void store.layoutVersion
  const visible = visiblePodPaths.value
  const isFocusedView = store.viewLevel !== 'global' && store.focusedPodPath
  const nodeSizes = measuredNodeSizes.value

  const globalPositions = layoutNodes(store.pods, store.edges)
  const focusedPositions = isFocusedView && focusedContext.value
    ? layoutFocused(store.focusedPodPath!, focusedContext.value, store.podMap, store.expandedPods, nodeSizes)
    : null

  return store.pods.map((pod) => {
    const isVisible = visible.has(pod.path)
    const pos = (focusedPositions && isVisible
      ? focusedPositions.get(pod.path)
      : globalPositions.get(pod.path)) ?? { x: 0, y: 0 }

    return {
      id: pod.path,
      type: 'pod',
      position: pos,
      hidden: !isVisible,
      data: {
        pod,
        isExpanded: store.isPodExpanded(pod.path),
        dotColor: pkgColorMap.value.get(pod.package) ?? '#409eff',
      },
    }
  })
})

const flowEdges = computed(() => {
  const visible = visiblePodPaths.value
  return store.edges
    .filter((edge) => visible.has(edge.source) && visible.has(edge.target))
    .map((edge, idx) => ({
      id: `e-${idx}`,
      source: edge.source,
      target: edge.target,
      animated: store.focusedPodPath
        ? edge.source === store.focusedPodPath || edge.target === store.focusedPodPath
        : false,
      style: getEdgeStyle(edge.source, edge.target),
    }))
})

function getEdgeStyle(source: string, target: string) {
  const primary = new Set([store.focusedPodPath, ...store.expandedPods])
  if (primary.size === 0 || !store.focusedPodPath) {
    return { stroke: '#b1b3b8', strokeWidth: 1.5 }
  }
  if (primary.has(source) || primary.has(target)) {
    return { stroke: '#409eff', strokeWidth: 2.5 }
  }
  return { stroke: '#b1b3b8', strokeWidth: 1, opacity: 0.4 }
}

function buildFocusedContext(
  centerPath: string,
  pods: Pod[],
  edges: { source: string; target: string }[],
  expandedSet: Set<string>,
): FocusedContext {
  const adjacency = buildEdgeMaps(pods.map((pod) => pod.path), edges)
  const reachableExpanded = new Set<string>([centerPath])
  const queue = [centerPath]

  while (queue.length > 0) {
    const current = queue.shift()!
    const canExpand = current === centerPath || expandedSet.has(current)

    if (!canExpand) continue

    for (const neighbor of sortPodPaths(adjacency.get(current) ?? [])) {
      if (!expandedSet.has(neighbor) || reachableExpanded.has(neighbor)) continue
      reachableExpanded.add(neighbor)
      queue.push(neighbor)
    }
  }

  const visible = new Set<string>([centerPath])

  for (const neighbor of sortPodPaths(adjacency.get(centerPath) ?? [])) {
    visible.add(neighbor)
  }

  for (const path of reachableExpanded) {
    visible.add(path)

    const canExpand = path === centerPath || expandedSet.has(path)
    if (!canExpand) continue

    for (const neighbor of sortPodPaths(adjacency.get(path) ?? [])) {
      visible.add(neighbor)
    }
  }

  return { visible, adjacency }
}

function layoutFocused(
  centerPath: string,
  context: FocusedContext,
  podLookup: Map<string, Pod>,
  expandedSet: Set<string>,
  nodeSizes: Map<string, PodSize>,
) {
  const positions = new Map<string, PodPosition>()
  positions.set(centerPath, { x: 0, y: 0 })

  const claimed = new Set<string>([centerPath])
  const rightTrees = buildBranchTrees(centerPath, centerPath, context, expandedSet, claimed)
  rightTrees.forEach((tree) => computeTreeSpan(tree, podLookup, expandedSet, nodeSizes))
  placeBranchTrees(centerPath, rightTrees, 0, 0, positions, podLookup, expandedSet, nodeSizes)

  placeLeftovers(centerPath, context, positions, podLookup, expandedSet, nodeSizes)

  return normalizePositions(positions)
}

function buildBranchTrees(
  centerPath: string,
  parentPath: string,
  context: FocusedContext,
  expandedSet: Set<string>,
  claimed: Set<string>,
  ancestry = new Set<string>([parentPath]),
): BranchTree[] {
  const canExpand = parentPath === centerPath || expandedSet.has(parentPath)
  if (!canExpand) return []

  const children = sortBranchChildren(
    [...(context.adjacency.get(parentPath) ?? [])].filter((path) => (
      context.visible.has(path) &&
      !claimed.has(path) &&
      !ancestry.has(path)
    )),
  )

  return children.map((path) => {
    claimed.add(path)
    const nextAncestry = new Set(ancestry)
    nextAncestry.add(path)

    return {
      id: path,
      children: buildBranchTrees(centerPath, path, context, expandedSet, claimed, nextAncestry),
      span: 0,
    }
  })
}

function computeTreeSpan(
  tree: BranchTree,
  podLookup: Map<string, Pod>,
  expandedSet: Set<string>,
  nodeSizes: Map<string, PodSize>,
): number {
  const VERTICAL_GAP = 44
  const nodeSize = getPodSize(tree.id, podLookup, expandedSet, nodeSizes)

  if (tree.children.length === 0) {
    tree.span = nodeSize.height
    return tree.span
  }

  const childSpan = tree.children.reduce((sum, child, index) => {
    const span = computeTreeSpan(child, podLookup, expandedSet, nodeSizes)
    return sum + span + (index > 0 ? VERTICAL_GAP : 0)
  }, 0)

  tree.span = Math.max(nodeSize.height, childSpan)
  return tree.span
}

function placeBranchTrees(
  parentPath: string,
  trees: BranchTree[],
  parentX: number,
  parentY: number,
  positions: Map<string, PodPosition>,
  podLookup: Map<string, Pod>,
  expandedSet: Set<string>,
  nodeSizes: Map<string, PodSize>,
) {
  if (!trees.length) return

  const VERTICAL_GAP = 44
  const parentSize = getPodSize(parentPath, podLookup, expandedSet, nodeSizes)

  // Calculate total height of children group
  const totalChildrenSpan = trees.reduce((sum, tree, index) => {
    return sum + tree.span + (index > 0 ? VERTICAL_GAP : 0)
  }, 0)

  // Center children group relative to parent's vertical center
  const parentCenterY = parentY + parentSize.height / 2
  let cursorY = parentCenterY - totalChildrenSpan / 2

  for (const tree of trees) {
    const gap = getHorizontalGap(parentPath, tree.id, expandedSet)
    const childX = parentX + parentSize.width + gap
    const childY = cursorY

    positions.set(tree.id, {
      x: childX,
      y: childY,
    })

    placeBranchTrees(
      tree.id,
      tree.children,
      childX,
      childY,
      positions,
      podLookup,
      expandedSet,
      nodeSizes,
    )

    cursorY += tree.span + VERTICAL_GAP
  }
}

function placeLeftovers(
  centerPath: string,
  context: FocusedContext,
  positions: Map<string, PodPosition>,
  podLookup: Map<string, Pod>,
  expandedSet: Set<string>,
  nodeSizes: Map<string, PodSize>,
) {
  const leftovers = sortPodPaths(
    [...context.visible].filter((path) => path !== centerPath && !positions.has(path)),
  )

  if (!leftovers.length) return

  const rootPos = positions.get(centerPath) ?? { x: 0, y: 0 }
  const rootSize = getPodSize(centerPath, podLookup, expandedSet, nodeSizes)
  const VERTICAL_GAP = 44
  let rightY = rootPos.y

  for (const path of leftovers) {
    positions.set(path, {
      x: rootPos.x + rootSize.width + 220,
      y: rightY,
    })

    const size = getPodSize(path, podLookup, expandedSet, nodeSizes)
    rightY += size.height + VERTICAL_GAP
  }
}

function getHorizontalGap(parentPath: string, childPath: string, expandedSet: Set<string>) {
  const parentExpanded = expandedSet.has(parentPath)
  const childExpanded = expandedSet.has(childPath)

  if (parentExpanded && childExpanded) return 180
  if (parentExpanded || childExpanded) return 150
  return 120
}

function getPodSize(
  path: string,
  podLookup: Map<string, Pod>,
  expandedSet: Set<string>,
  nodeSizes: Map<string, PodSize>,
) {
  const measured = nodeSizes.get(path)
  if (measured) return measured

  const pod = podLookup.get(path)
  const isExpanded = expandedSet.has(path)

  if (isExpanded) {
    const rows = Math.max(1, pod?.containers?.length ?? 0)
    return {
      width: 360,
      height: Math.min(560, 96 + rows * 32),
    }
  }

  return path === store.focusedPodPath
    ? { width: 120, height: 96 }
    : { width: 96, height: 84 }
}

function buildEdgeMaps(paths: string[], edges: { source: string; target: string }[]) {
  const adjacency = new Map<string, Set<string>>()

  for (const path of paths) {
    adjacency.set(path, new Set())
  }

  for (const edge of edges) {
    if (!adjacency.has(edge.source)) adjacency.set(edge.source, new Set())
    if (!adjacency.has(edge.target)) adjacency.set(edge.target, new Set())

    adjacency.get(edge.source)!.add(edge.target)
  }

  return adjacency
}

function sortBranchChildren(paths: string[]) {
  return [...paths].sort(comparePodPaths)
}

function sortPodPaths(paths: Iterable<string>) {
  return [...paths].sort(comparePodPaths)
}

function comparePodPaths(a: string, b: string) {
  const dirA = a.substring(0, a.lastIndexOf('/'))
  const dirB = b.substring(0, b.lastIndexOf('/'))
  if (dirA !== dirB) return dirA.localeCompare(dirB)
  return a.localeCompare(b)
}

function layoutNodes(pods: Pod[], edges: { source: string; target: string }[]) {
  const positions = new Map<string, { x: number; y: number }>()
  if (pods.length === 0) return positions

  const adjacency = new Map<string, Set<string>>()
  const inDegree = new Map<string, number>()

  for (const pod of pods) {
    adjacency.set(pod.path, new Set())
    inDegree.set(pod.path, 0)
  }

  for (const edge of edges) {
    if (adjacency.has(edge.source) && adjacency.has(edge.target)) {
      adjacency.get(edge.source)!.add(edge.target)
      inDegree.set(edge.target, (inDegree.get(edge.target) ?? 0) + 1)
    }
  }

  const layers: string[][] = []
  const placed = new Set<string>()
  let queue = pods
    .filter((p) => (inDegree.get(p.path) ?? 0) === 0)
    .map((p) => p.path)

  while (queue.length > 0) {
    layers.push([...queue])
    queue.forEach((n) => placed.add(n))
    const nextQueue: string[] = []
    for (const node of queue) {
      for (const neighbor of adjacency.get(node) ?? []) {
        if (!placed.has(neighbor)) {
          const remaining = (inDegree.get(neighbor) ?? 1) - 1
          inDegree.set(neighbor, remaining)
          if (remaining <= 0) {
            nextQueue.push(neighbor)
          }
        }
      }
    }
    queue = nextQueue
  }

  for (const pod of pods) {
    if (!placed.has(pod.path)) {
      if (layers.length === 0) layers.push([])
      layers[layers.length - 1].push(pod.path)
    }
  }

  for (const layer of layers) {
    layer.sort((a, b) => {
      const dirA = a.substring(0, a.lastIndexOf('/'))
      const dirB = b.substring(0, b.lastIndexOf('/'))
      if (dirA !== dirB) return dirA.localeCompare(dirB)
      return a.localeCompare(b)
    })
  }

  const COL_GAP = 250
  const ROW_GAP = 70
  const GROUP_GAP = 30
  const MAX_PER_COLUMN = 15

  for (let col = 0; col < layers.length; col++) {
    const layer = layers[col]

    const subCols: string[][] = []
    for (let i = 0; i < layer.length; i += MAX_PER_COLUMN) {
      subCols.push(layer.slice(i, i + MAX_PER_COLUMN))
    }

    for (let sc = 0; sc < subCols.length; sc++) {
      const nodes = subCols[sc]
      const xBase = col * COL_GAP * 1.5 + sc * COL_GAP * 0.6

      let y = 0
      let prevDir = ''
      for (const nodePath of nodes) {
        const dir = nodePath.substring(0, nodePath.lastIndexOf('/'))
        if (prevDir && dir !== prevDir) {
          y += GROUP_GAP
        }
        prevDir = dir
        positions.set(nodePath, { x: xBase, y })
        y += ROW_GAP
      }

      const totalHeight = y - ROW_GAP
      for (const nodePath of nodes) {
        const pos = positions.get(nodePath)!
        pos.y -= totalHeight / 2
      }
    }
  }

  return normalizePositions(positions, 120, 80)
}

function normalizePositions(
  positions: Map<string, PodPosition>,
  paddingX = 80,
  paddingY = 80,
) {
  if (!positions.size) return positions

  let minX = Number.POSITIVE_INFINITY
  let minY = Number.POSITIVE_INFINITY

  for (const pos of positions.values()) {
    minX = Math.min(minX, pos.x)
    minY = Math.min(minY, pos.y)
  }

  if (!Number.isFinite(minX) || !Number.isFinite(minY)) {
    return positions
  }

  const offsetX = paddingX - minX
  const offsetY = paddingY - minY

  if (offsetX === 0 && offsetY === 0) {
    return positions
  }

  return new Map(
    [...positions.entries()].map(([path, pos]) => [
      path,
      { x: pos.x + offsetX, y: pos.y + offsetY },
    ]),
  )
}
</script>

<template>
  <div class="pod-graph">
    <div v-if="!store.pods.length && !store.loading" class="graph-empty">
      <el-empty description="Load a Go project to view the Pod dependency graph" />
    </div>

    <VueFlow
      v-else
      :nodes="flowNodes"
      :edges="flowEdges"
      :node-types="nodeTypes"
      :default-viewport="{ zoom: 1, x: 0, y: 0 }"
      :min-zoom="0.1"
      :max-zoom="2"
      class="vue-flow-wrapper"
    >
      <Background />
      <Controls />

      <div class="vue-flow__panel vue-flow__panel-bottom-right">
        <el-button-group size="small">
          <el-button
            :disabled="store.historyIndex <= 0"
            @click="store.goBack()"
            title="Cmd+["
          >
            &larr;
          </el-button>
          <el-button
            :disabled="store.historyIndex >= store.navigationHistory.length - 1"
            @click="store.goForward()"
            title="Cmd+]"
          >
            &rarr;
          </el-button>
        </el-button-group>
      </div>
    </VueFlow>

    <FloatingCodeTab
      v-for="tab in store.floatingTabs"
      :key="tab.id"
      :tab="tab"
    />
  </div>
</template>

<style scoped>
.pod-graph {
  width: 100%;
  height: 100%;
}

.vue-flow-wrapper {
  width: 100%;
  height: 100%;
}

.graph-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
}

.vue-flow__panel-bottom-right {
  position: absolute;
  left: 52px;
  bottom: 0px;
  z-index: 10;
}
</style>
