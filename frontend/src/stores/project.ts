import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'
import type {
  FileTreeNode,
  Pod,
  PodEdge,
  Container,
  ViewLevel,
  NavigationEntry,
  FloatingTab,
} from '../types'
import * as api from '../api/client'

export const useProjectStore = defineStore('project', () => {
  const projectPath = ref('')
  const fileTree = ref<FileTreeNode | null>(null)
  const pods = ref<Pod[]>([])
  const edges = ref<PodEdge[]>([])
  const loading = ref(false)

  const viewLevel = ref<ViewLevel>('global')
  const focusedPodPath = ref<string | null>(null)
  const expandedPods = ref<Set<string>>(new Set())
  const selectedContainer = ref<Container | null>(null)
  const dependencyDepth = ref(1)

  const navigationHistory = ref<NavigationEntry[]>([])
  const historyIndex = ref(-1)
  const floatingTabs = ref<FloatingTab[]>([])

  // Container 展开状态管理
  interface PodContainerState {
    expandedGroups: Set<string>
    activeContainers: Set<string>
  }

  const containerStates = ref<Record<string, PodContainerState>>({})

  function getPodContainerState(podPath: string): PodContainerState {
    if (!containerStates.value[podPath]) {
      return {
        expandedGroups: new Set(),
        activeContainers: new Set(),
      }
    }
    return containerStates.value[podPath]
  }

  function expandGroup(podPath: string, groupName: string) {
    const state = getPodContainerState(podPath)
    const newState = { ...state, expandedGroups: new Set(state.expandedGroups) }
    newState.expandedGroups.add(groupName)
    containerStates.value = { ...containerStates.value, [podPath]: newState }
  }

  function collapseGroup(podPath: string, groupName: string) {
    const state = containerStates.value[podPath]
    if (!state) return
    const newState = { ...state, expandedGroups: new Set(state.expandedGroups) }
    newState.expandedGroups.delete(groupName)
    containerStates.value = { ...containerStates.value, [podPath]: newState }
  }

  function isGroupExpanded(podPath: string, groupName: string): boolean {
    return containerStates.value[podPath]?.expandedGroups.has(groupName) ?? false
  }

  function activateContainer(podPath: string, containerName: string) {
    const state = containerStates.value[podPath]
    const activeContainers = state ? new Set(state.activeContainers) : new Set<string>()
    activeContainers.add(containerName)
    const newState = state ? { ...state, activeContainers } : { expandedGroups: new Set<string>(), activeContainers }
    containerStates.value = { ...containerStates.value, [podPath]: newState }
  }

  function deactivateContainer(podPath: string, containerName: string) {
    const state = containerStates.value[podPath]
    if (!state) return
    const newActiveContainers = new Set(state.activeContainers)
    newActiveContainers.delete(containerName)
    const newState = { ...state, activeContainers: newActiveContainers }
    containerStates.value = { ...containerStates.value, [podPath]: newState }
  }

  function clearActiveContainers(podPath: string) {
    const state = containerStates.value[podPath]
    if (!state) return
    const newState = { ...state, activeContainers: new Set<string>() }
    containerStates.value = { ...containerStates.value, [podPath]: newState }
  }

  function isContainerActive(podPath: string, containerName: string): boolean {
    return containerStates.value[podPath]?.activeContainers.has(containerName) ?? false
  }

  function snapshotContainerState(): {
    expandedGroups: Record<string, string[]>
    activeContainers: Record<string, string[]>
  } {
    const expandedGroups: Record<string, string[]> = {}
    const activeContainers: Record<string, string[]> = {}

    for (const [podPath, state] of Object.entries(containerStates.value)) {
      if (state.expandedGroups.size > 0) {
        expandedGroups[podPath] = Array.from(state.expandedGroups)
      }
      if (state.activeContainers.size > 0) {
        activeContainers[podPath] = Array.from(state.activeContainers)
      }
    }

    return { expandedGroups, activeContainers }
  }

  function restoreContainerState(
    expandedGroups: Record<string, string[]>,
    activeContainers: Record<string, string[] | null>,
  ) {
    const newStates: Record<string, PodContainerState> = {}

    for (const [podPath, groups] of Object.entries(expandedGroups)) {
      if (!newStates[podPath]) {
        newStates[podPath] = { expandedGroups: new Set<string>(), activeContainers: new Set<string>() }
      }
      newStates[podPath].expandedGroups = new Set(groups)
    }

    for (const [podPath, active] of Object.entries(activeContainers)) {
      if (!newStates[podPath]) {
        newStates[podPath] = { expandedGroups: new Set<string>(), activeContainers: new Set<string>() }
      }
      if (active) {
        newStates[podPath].activeContainers = new Set(active)
      }
    }

    containerStates.value = newStates
  }

  function clearContainerStates() {
    containerStates.value = {}
  }

  type ViewAction = 'none' | 'focus' | 'expand' | 'jump' | 'code-toggle'
  const lastAction = ref<ViewAction>('none')
  const layoutVersion = ref(0)

  function bumpLayout() {
    lastAction.value = 'code-toggle'
    layoutVersion.value++
  }

  const podMap = computed(() => {
    const m = new Map<string, Pod>()
    for (const p of pods.value) {
      m.set(p.path, p)
    }
    return m
  })

  const focusedPod = computed(() => {
    if (!focusedPodPath.value) return null
    return podMap.value.get(focusedPodPath.value) ?? null
  })

  function isPodExpanded(path: string) {
    return expandedPods.value.has(path)
  }

  async function loadProject(path: string) {
    loading.value = true
    try {
      await api.setProject(path)
      projectPath.value = path

      const [tree, podsData] = await Promise.all([
        api.getFileTree(),
        api.getPods(),
      ])

      fileTree.value = tree
      pods.value = podsData.pods
      edges.value = podsData.edges

      resetView()
    } finally {
      loading.value = false
    }
  }

  async function refreshData() {
    if (!projectPath.value) return
    loading.value = true
    try {
      const [tree, podsData] = await Promise.all([
        api.getFileTree(),
        api.getPods(),
      ])
      fileTree.value = tree
      pods.value = podsData.pods
      edges.value = podsData.edges
    } finally {
      loading.value = false
    }
  }

  function resetView() {
    viewLevel.value = 'global'
    focusedPodPath.value = null
    expandedPods.value = new Set()
    selectedContainer.value = null
    clearContainerStates()
    navigationHistory.value = [{ level: 'global' }]
    historyIndex.value = 0
  }

  function pushNavigation(entry: NavigationEntry) {
    navigationHistory.value = navigationHistory.value.slice(0, historyIndex.value + 1)
    navigationHistory.value.push(entry)
    historyIndex.value = navigationHistory.value.length - 1
    syncUrlState()
  }

  function snapshotExpandedPods(rootPath?: string | null) {
    const items = [...expandedPods.value]
    if (rootPath) {
      items.sort((a, b) => {
        if (a === rootPath) return -1
        if (b === rootPath) return 1
        return a.localeCompare(b)
      })
      return items
    }
    return items.sort((a, b) => a.localeCompare(b))
  }

  function buildAdjacency() {
    const adjacency = new Map<string, Set<string>>()

    for (const pod of pods.value) {
      adjacency.set(pod.path, new Set())
    }

    for (const edge of edges.value) {
      if (!adjacency.has(edge.source)) adjacency.set(edge.source, new Set())
      adjacency.get(edge.source)!.add(edge.target)
    }

    return adjacency
  }

  function collectExpandedBranch(rootPath: string, expanded: Set<string>) {
    const adjacency = buildAdjacency()
    const branch = new Set<string>()
    const queue = [rootPath]

    while (queue.length > 0) {
      const current = queue.shift()!
      if (branch.has(current)) continue
      branch.add(current)

      for (const next of adjacency.get(current) ?? []) {
        if (expanded.has(next)) {
          queue.push(next)
        }
      }
    }

    return branch
  }

  function focusPod(podPath: string) {
    if (viewLevel.value === 'focused' && focusedPodPath.value === podPath) {
      expandPod(podPath)
      return
    }

    lastAction.value = 'focus'
    viewLevel.value = 'focused'
    focusedPodPath.value = podPath
    expandedPods.value = new Set()
    selectedContainer.value = null
    clearContainerStates()
    pushNavigation({ level: 'focused', podPath })
  }

  async function expandInlinePod(podPath: string) {
    if (!focusedPodPath.value || podPath === focusedPodPath.value) {
      await expandPod(podPath)
      return
    }

    if (expandedPods.value.has(podPath)) {
      collapseInlinePod(podPath)
      return
    }

    lastAction.value = 'expand'
    viewLevel.value = 'expanded'
    selectedContainer.value = null

    const newSet = new Set(expandedPods.value)
    newSet.add(focusedPodPath.value)
    newSet.add(podPath)
    expandedPods.value = newSet

    await ensurePodSourceCode(podPath)
    const { expandedGroups, activeContainers } = snapshotContainerState()
    pushNavigation({
      level: 'expanded',
      podPath: focusedPodPath.value,
      expandedPods: snapshotExpandedPods(focusedPodPath.value),
      expandedGroups,
      activeContainers,
    })
  }

  function collapseInlinePod(podPath: string) {
    if (
      !focusedPodPath.value ||
      viewLevel.value !== 'expanded' ||
      podPath === focusedPodPath.value ||
      !expandedPods.value.has(podPath)
    ) {
      return
    }

    const removed = collectExpandedBranch(podPath, expandedPods.value)
    const newSet = new Set(expandedPods.value)
    for (const path of removed) {
      newSet.delete(path)
    }
    newSet.add(focusedPodPath.value)

    expandedPods.value = newSet

    if (selectedContainer.value && removed.has(selectedContainer.value.pod)) {
      selectedContainer.value = null
    }

    lastAction.value = 'none'
    const { expandedGroups, activeContainers } = snapshotContainerState()
    pushNavigation({
      level: 'expanded',
      podPath: focusedPodPath.value,
      expandedPods: snapshotExpandedPods(focusedPodPath.value),
      expandedGroups,
      activeContainers,
    })
  }

  async function expandPod(podPath: string) {
    lastAction.value = 'expand'
    viewLevel.value = 'expanded'
    focusedPodPath.value = podPath
    selectedContainer.value = null

    const newSet = new Set(expandedPods.value)
    newSet.add(podPath)
    expandedPods.value = newSet

    await ensurePodSourceCode(podPath)
    const { expandedGroups, activeContainers } = snapshotContainerState()
    pushNavigation({
      level: 'expanded',
      podPath,
      expandedPods: snapshotExpandedPods(podPath),
      expandedGroups,
      activeContainers,
    })
  }

  async function ensurePodSourceCode(podPath: string) {
    const pod = podMap.value.get(podPath)
    if (pod) {
      const hasSource = pod.containers?.some((c) => c.sourceCode && c.sourceCode.length > 0)
      if (!hasSource) {
        const containers = await api.getContainers(podPath)
        pod.containers = containers
      }
    }
  }

  async function selectContainer(podPath: string, containerName: string) {
    lastAction.value = 'jump'
    const layoutRoot = focusedPodPath.value ?? podPath
    const keepCurrentRoot = viewLevel.value === 'expanded' && !!focusedPodPath.value

    const newSet = new Set(expandedPods.value)
    if (keepCurrentRoot) {
      newSet.add(layoutRoot)
    }
    newSet.add(podPath)
    expandedPods.value = newSet

    await ensurePodSourceCode(podPath)

    viewLevel.value = 'expanded'
    focusedPodPath.value = keepCurrentRoot ? layoutRoot : podPath
    const container = await api.getContainer(podPath, containerName)
    selectedContainer.value = container
    const { expandedGroups, activeContainers } = snapshotContainerState()
    pushNavigation({
      level: 'expanded',
      podPath: keepCurrentRoot ? layoutRoot : podPath,
      containerName,
      expandedPods: snapshotExpandedPods(keepCurrentRoot ? layoutRoot : podPath),
      expandedGroups,
      activeContainers,
    })
  }

  function goBack() {
    if (historyIndex.value <= 0) return
    historyIndex.value--
    applyNavigation(navigationHistory.value[historyIndex.value])
  }

  function goForward() {
    if (historyIndex.value >= navigationHistory.value.length - 1) return
    historyIndex.value++
    applyNavigation(navigationHistory.value[historyIndex.value])
  }

  function applyNavigation(entry: NavigationEntry) {
    viewLevel.value = entry.level
    focusedPodPath.value = entry.podPath ?? null
    if (entry.level === 'expanded') {
      expandedPods.value = new Set(entry.expandedPods ?? (entry.podPath ? [entry.podPath] : []))
    } else {
      expandedPods.value = new Set()
    }
    if (entry.level !== 'expanded') {
      selectedContainer.value = null
    }
    // Restore container states
    if (entry.expandedGroups || entry.activeContainers) {
      restoreContainerState(entry.expandedGroups ?? {}, entry.activeContainers ?? {})
    }
  }

  function setDependencyDepth(depth: number) {
    dependencyDepth.value = Math.max(1, Math.min(10, depth))
  }

  let tabCounter = 0
  function openFloatingTab(container: Container) {
    const existing = floatingTabs.value.find(
      (t) => t.podPath === container.pod && t.containerName === container.name,
    )
    if (existing) return

    tabCounter++
    const offset = tabCounter * 30
    floatingTabs.value.push({
      id: `tab-${Date.now()}-${tabCounter}`,
      title: container.name,
      signature: container.signature,
      sourceCode: container.sourceCode ?? '',
      podPath: container.pod,
      containerName: container.name,
      x: 100 + offset,
      y: 80 + offset,
    })
  }

  function closeFloatingTab(id: string) {
    floatingTabs.value = floatingTabs.value.filter((t) => t.id !== id)
  }

  const suppressUrlSync = ref(false)

  function syncUrlState() {
    if (suppressUrlSync.value) return

    const params = new URLSearchParams()

    if (projectPath.value) {
      params.set('project', projectPath.value)
    }

    if (focusedPodPath.value) {
      params.set('file', focusedPodPath.value)
    }

    if (viewLevel.value !== 'global') {
      params.set('level', viewLevel.value)
    }

    if (viewLevel.value === 'expanded' && focusedPodPath.value) {
      const expanded = snapshotExpandedPods(focusedPodPath.value)
        .filter((path) => path !== focusedPodPath.value)

      if (expanded.length > 0) {
        params.set('expanded', expanded.join(','))
      }

      // Add container states to URL
      const { expandedGroups, activeContainers } = snapshotContainerState()

      const groupEntries: string[] = []
      for (const [podPath, groups] of Object.entries(expandedGroups)) {
        for (const group of groups) {
          groupEntries.push(`${encodeURIComponent(podPath)}:${encodeURIComponent(group)}`)
        }
      }
      if (groupEntries.length > 0) {
        params.set('groups', groupEntries.join(','))
      }

      const activeEntries: string[] = []
      for (const [podPath, containers] of Object.entries(activeContainers)) {
        for (const container of containers) {
          activeEntries.push(`${encodeURIComponent(podPath)}:${encodeURIComponent(container)}`)
        }
      }
      if (activeEntries.length > 0) {
        params.set('active', activeEntries.join(','))
      }
    }

    const qs = params.toString()
    const newUrl = qs
      ? `${window.location.pathname}?${qs}`
      : window.location.pathname
    window.history.replaceState(null, '', newUrl)
  }

  watch(
    [
      focusedPodPath,
      viewLevel,
      projectPath,
      () => snapshotExpandedPods(focusedPodPath.value).join('|'),
      () => JSON.stringify(snapshotContainerState()),
    ],
    syncUrlState,
  )

  async function restoreFromUrl() {
    const params = new URLSearchParams(window.location.search)
    const project = params.get('project')
    const file = params.get('file')
    const level = params.get('level') as ViewLevel | null
    const expandedParam = params.get('expanded')
    const groupsParam = params.get('groups')
    const activeParam = params.get('active')

    if (!project) return false

    suppressUrlSync.value = true

    loading.value = true
    try {
      await api.setProject(project)
      projectPath.value = project

      const [tree, podsData] = await Promise.all([
        api.getFileTree(),
        api.getPods(),
      ])

      fileTree.value = tree
      pods.value = podsData.pods
      edges.value = podsData.edges
    } finally {
      loading.value = false
    }

    suppressUrlSync.value = false

    if (file && podMap.value.has(file)) {
      if (level === 'expanded') {
        const expandedList = (expandedParam ?? '')
          .split(',')
          .map((item) => item.trim())
          .filter((item) => item.length > 0 && podMap.value.has(item))
        const expandedListWithRoot = Array.from(new Set([file, ...expandedList]))

        await Promise.all(expandedListWithRoot.map((path) => ensurePodSourceCode(path)))

        viewLevel.value = 'expanded'
        focusedPodPath.value = file
        expandedPods.value = new Set(expandedListWithRoot)
        selectedContainer.value = null

        // Restore container states from URL
        const expandedGroups: Record<string, string[]> = {}
        const activeContainers: Record<string, string[]> = {}

        if (groupsParam) {
          for (const entry of groupsParam.split(',')) {
            const [podPath, groupName] = entry.split(':').map(decodeURIComponent)
            if (podPath && groupName && podMap.value.has(podPath)) {
              if (!expandedGroups[podPath]) {
                expandedGroups[podPath] = []
              }
              expandedGroups[podPath].push(groupName)
            }
          }
        }

        if (activeParam) {
          for (const entry of activeParam.split(',')) {
            const [podPath, containerName] = entry.split(':').map(decodeURIComponent)
            if (podPath && containerName && podMap.value.has(podPath)) {
              if (!activeContainers[podPath]) {
                activeContainers[podPath] = []
              }
              activeContainers[podPath].push(containerName)
            }
          }
        }

        restoreContainerState(expandedGroups, activeContainers)

        const { expandedGroups: snapshotGroups, activeContainers: snapshotActive } = snapshotContainerState()
        navigationHistory.value = [{
          level: 'expanded',
          podPath: file,
          expandedPods: expandedListWithRoot,
          expandedGroups: snapshotGroups,
          activeContainers: snapshotActive,
        }]
        historyIndex.value = 0
        syncUrlState()
      } else {
        focusPod(file)
      }
    } else {
      resetView()
    }

    return true
  }

  return {
    projectPath,
    fileTree,
    pods,
    edges,
    loading,
    viewLevel,
    focusedPodPath,
    expandedPods,
    selectedContainer,
    dependencyDepth,
    navigationHistory,
    historyIndex,
    podMap,
    focusedPod,
    lastAction,
    layoutVersion,
    isPodExpanded,
    bumpLayout,
    loadProject,
    refreshData,
    resetView,
    focusPod,
    expandInlinePod,
    expandPod,
    selectContainer,
    goBack,
    goForward,
    setDependencyDepth,
    floatingTabs,
    openFloatingTab,
    closeFloatingTab,
    restoreFromUrl,
    // Container state methods
    containerStates,
    expandGroup,
    collapseGroup,
    isGroupExpanded,
    activateContainer,
    deactivateContainer,
    clearActiveContainers,
    isContainerActive,
  }
})
