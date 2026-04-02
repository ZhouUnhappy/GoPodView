<script setup lang="ts">
import { computed, watch, onBeforeUnmount, nextTick, type ComponentPublicInstance, ref } from 'vue'
import { Handle, Position } from '@vue-flow/core'
import * as monaco from 'monaco-editor'
import { useProjectStore } from '../../stores/project'
import type { Pod, Container, ContainerType } from '../../types'

interface ContainerGroup {
  parent: Container
  methods: Container[]
}

const props = defineProps<{
  data: {
    pod: Pod
    isExpanded: boolean
    dotColor: string
  }
}>()

const store = useProjectStore()
const cardRef = ref<HTMLElement | null>(null)
const cardWidth = ref<number>(360)

const isFocused = computed(() => store.focusedPodPath === props.data.pod.path)

const containerCount = computed(() => props.data.pod.containers?.length ?? 0)

const dotSize = computed(() => {
  const count = containerCount.value
  return Math.min(48, Math.max(24, 20 + count * 3))
})

const tooltipText = computed(() => {
  const p = props.data.pod
  return `${p.path}\npkg: ${p.package}\n${containerCount.value} containers`
})

const containerTypeColors: Record<ContainerType, string> = {
  func: '#409eff',
  struct: '#67c23a',
  interface: '#e6a23c',
  const: '#9b59b6',
  var: '#8b6914',
}

const groupedContainers = computed<(Container | ContainerGroup)[]>(() => {
  const containers = props.data.pod.containers ?? []
  if (!containers.length) return []

  const structNames = new Set<string>()
  for (const c of containers) {
    if (c.type === 'struct' || c.type === 'interface') {
      structNames.add(c.name)
    }
  }

  const methodMap = new Map<string, Container[]>()
  const standalone: Container[] = []

  for (const c of containers) {
    if (c.type === 'func' && c.name.includes('.')) {
      const receiver = c.name.substring(0, c.name.indexOf('.'))
      const clean = receiver.replace(/^\*/, '')
      if (structNames.has(clean)) {
        if (!methodMap.has(clean)) methodMap.set(clean, [])
        methodMap.get(clean)!.push(c)
        continue
      }
    }
    standalone.push(c)
  }

  const result: (Container | ContainerGroup)[] = []
  for (const c of standalone) {
    if ((c.type === 'struct' || c.type === 'interface') && methodMap.has(c.name)) {
      result.push({ parent: c, methods: methodMap.get(c.name)! })
      methodMap.delete(c.name)
    } else {
      result.push(c)
    }
  }

  for (const [, methods] of methodMap) {
    for (const m of methods) result.push(m)
  }

  return result
})

const cardStyle = computed(() => {
  const baseWidth = cardWidth.value
  return {
    width: baseWidth + 'px',
    minWidth: '260px',
    maxWidth: '800px'
  }
})

let isResizing = false
let startX = 0
let startWidth = 0
let hasDragged = false

function startResize(e: MouseEvent) {
  isResizing = true
  hasDragged = false
  startX = e.clientX
  startWidth = cardRef.value?.offsetWidth || 360
  document.addEventListener('mousemove', handleResize)
  document.addEventListener('mouseup', stopResize)
  e.preventDefault()
  e.stopPropagation()
}

function handleResize(e: MouseEvent) {
  if (!isResizing) return
  const deltaX = e.clientX - startX
  if (Math.abs(deltaX) > 3) {
    hasDragged = true
  }
  const newWidth = Math.max(260, Math.min(800, startWidth + deltaX))
  cardWidth.value = newWidth
}

function stopResize() {
  isResizing = false
  document.removeEventListener('mousemove', handleResize)
  document.removeEventListener('mouseup', stopResize)
  // Clear hasDragged after a short delay to prevent the click that follows mouseup
  setTimeout(() => {
    hasDragged = false
  }, 50)
}

const activeContainerNames = computed<Set<string>>(() => {
  return store.containerStates[props.data.pod.path]?.activeContainers ?? new Set()
})

const hasActiveContainer = computed(() => activeContainerNames.value.size > 0)

function isContainerActiveByName(name: string): boolean {
  return activeContainerNames.value.has(name)
}

const expandedGroup = computed<string | null>(() => {
  const podPath = props.data.pod.path
  const state = store.containerStates[podPath]
  if (!state) return null
  // Return the first expanded group (or could track multiple)
  for (const groupName of state.expandedGroups) {
    return groupName
  }
  return null
})
// 根据是否有展开的容器自动调整卡片宽度
watch(hasActiveContainer, (hasActive) => {
  if (hasActive) {
    cardWidth.value = 800
  } else {
    cardWidth.value = 360
  }
})

const editors = new Map<string, monaco.editor.IStandaloneCodeEditor>()

function isGroup(item: Container | ContainerGroup): item is ContainerGroup {
  return 'parent' in item && 'methods' in item
}

async function handleClick() {
  // Prevent fold action if we just finished a resize drag
  if (hasDragged) {
    return
  }

  const podPath = props.data.pod.path

  // 点击已聚焦的 Pod：展开它（focused → expanded）
  if (store.focusedPodPath === podPath) {
    store.focusPod(podPath)
    return
  }

  // 点击其他 Pod（不改变聚焦，仅展开）
  if (store.focusedPodPath && podPath !== store.focusedPodPath) {
    await store.expandInlinePod(podPath)
    return
  }
}

function handleContainerClick(container: Container, event: MouseEvent) {
  event.stopPropagation()
  if (event.metaKey || event.ctrlKey) {
    jumpToRef(container)
    return
  }
  toggleCodeView(container)
}

function handleGroupClick(group: ContainerGroup, event: MouseEvent) {
  event.stopPropagation()
  const podPath = props.data.pod.path
  const groupName = group.parent.name
  console.log('handleGroupClick:', podPath, groupName, 'isExpanded:', store.isGroupExpanded(podPath, groupName))
  if (store.isGroupExpanded(podPath, groupName)) {
    store.collapseGroup(podPath, groupName)
    if (store.isContainerActive(podPath, group.parent.name)) {
      store.deactivateContainer(podPath, group.parent.name)
    }
  } else {
    store.expandGroup(podPath, groupName)
    store.activateContainer(podPath, group.parent.name)
  }
  nextTick(() => store.bumpLayout())
}

function toggleCodeView(container: Container) {
  const podPath = props.data.pod.path
  console.log('toggleCodeView:', podPath, container.name, 'isActive:', store.isContainerActive(podPath, container.name))
  console.log('containerStates:', store.containerStates[podPath])
  if (store.isContainerActive(podPath, container.name)) {
    store.deactivateContainer(podPath, container.name)
    nextTick(() => store.bumpLayout())
    return
  }
  store.activateContainer(podPath, container.name)
  nextTick(() => store.bumpLayout())
}

async function jumpToRef(container: Container) {
  if (!container.references?.length) return
  const r = container.references[0]
  store.selectContainer(r.podPath, r.containerName)
}

function handleRefClick(podPath: string, containerName: string, event: MouseEvent) {
  event.stopPropagation()
  store.selectContainer(podPath, containerName)
}

function popOutCode(container: Container, event: MouseEvent) {
  event.stopPropagation()
  store.openFloatingTab(container)
}

function onEditorMount(container: Container) {
  return function(el: Element | ComponentPublicInstance | null) {
    if (!el || !(el instanceof HTMLElement)) return
    // 如果已存在该容器的编辑器，先销毁
    const existing = editors.get(container.name)
    if (existing) {
      existing.dispose()
    }
    const editor = monaco.editor.create(el, {
      value: container.sourceCode ?? '',
      language: 'go',
      theme: 'vs',
      readOnly: true,
      minimap: { enabled: false },
      fontSize: 12,
      lineNumbers: 'on',
      scrollBeyondLastLine: false,
      automaticLayout: true,
      wordWrap: 'on',
      padding: { top: 8 },
      scrollbar: { vertical: 'auto', horizontal: 'auto' },
    })
    editors.set(container.name, editor)
  }
}

watch(hasActiveContainer, (hasActive) => {
  if (!hasActive) {
    // 清理所有编辑器
    editors.forEach((editor) => editor.dispose())
    editors.clear()
  }
})

watch(() => props.data.isExpanded, (expanded) => {
  if (!expanded) {
    // When pod collapses, clear all active containers for this pod
    store.clearActiveContainers(props.data.pod.path)
  }
})

watch(
  () => store.selectedContainer,
  (sc) => {
    if (!sc || sc.pod !== props.data.pod.path || !props.data.isExpanded) return
    const containers = props.data.pod.containers ?? []
    const match = containers.find((c) => c.name === sc.name)
    if (match) {
      // Activate the selected container in store
      store.activateContainer(props.data.pod.path, sc.name)
      const dot = sc.name.indexOf('.')
      if (dot >= 0) {
        const receiver = sc.name.substring(0, dot).replace(/^\*/, '')
        store.expandGroup(props.data.pod.path, receiver)
      }
    }
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  editors.forEach((editor) => editor.dispose())
  editors.clear()
})

function shortMethodName(fullName: string) {
  const dot = fullName.indexOf('.')
  return dot >= 0 ? fullName.substring(dot + 1) : fullName
}
</script>

<template>
  <div
    v-if="data.isExpanded"
    ref="cardRef"
    class="pod-card nopan nowheel"
    :style="cardStyle"
    @click="handleClick"
  >
    <Handle type="target" :position="Position.Left" class="handle-hidden" />
    <Handle type="source" :position="Position.Right" class="handle-hidden" />

    <div class="resize-handle" @mousedown="startResize" @click.stop title="Drag to resize"></div>

    <div class="card-header">
      <span class="card-package">{{ data.pod.package }}</span>
      <span class="card-filename">{{ data.pod.fileName }}</span>
      <span class="card-filepath" :title="data.pod.path">{{ data.pod.path }}</span>
    </div>

    <div class="card-containers">
      <template v-for="item in groupedContainers" :key="isGroup(item) ? item.parent.name : item.name">
        <!-- Struct/Interface group -->
        <template v-if="isGroup(item)">
          <div
            class="container-item group-header"
            :class="{ 'container-active': isContainerActiveByName(item.parent.name) }"
            @click="handleGroupClick(item, $event)"
          >
            <span
              class="container-badge"
              :style="{ background: containerTypeColors[item.parent.type] }"
            >
              {{ item.parent.type.charAt(0).toUpperCase() }}
            </span>
            <span class="container-name" :title="item.parent.signature">{{ item.parent.name }}</span>
            <span class="method-count">{{ item.methods.length }}m</span>
            <span class="expand-arrow" :class="{ rotated: expandedGroup === item.parent.name }">&#9654;</span>
          </div>

          <!-- Struct code -->
          <div v-if="isContainerActiveByName(item.parent.name)" class="inline-code" @click.stop>
            <div class="code-toolbar">
              <div class="code-sig">{{ item.parent.signature }}</div>
              <button class="code-action-btn" @click="popOutCode(item.parent, $event)" title="Pop Out">&#8599;</button>
            </div>
            <div :ref="onEditorMount(item.parent)" class="code-editor"></div>
          </div>

          <!-- Methods -->
          <template v-if="expandedGroup === item.parent.name">
            <div
              v-for="m in item.methods"
              :key="m.name"
              class="container-section method-section"
            >
              <div
                class="container-item method-item"
                :class="{ 'container-active': isContainerActiveByName(m.name) }"
                @click="handleContainerClick(m, $event)"
              >
                <span class="container-badge" :style="{ background: containerTypeColors.func }">F</span>
                <span class="container-name" :title="m.signature">{{ shortMethodName(m.name) }}</span>
                <span class="container-lines">L{{ m.startLine }}-{{ m.endLine }}</span>
              </div>

              <div v-if="isContainerActiveByName(m.name)" class="inline-code" @click.stop>
                <div class="code-toolbar">
                  <div class="code-sig">{{ m.signature }}</div>
                  <button class="code-action-btn" @click="popOutCode(m, $event)" title="Pop Out">&#8599;</button>
                </div>
                <div :ref="onEditorMount(m)" class="code-editor"></div>
                <div v-if="m.references?.length" class="code-refs">
                  <div
                    v-for="r in m.references"
                    :key="r.podPath + '#' + r.containerName"
                    class="ref-item"
                    @click="handleRefClick(r.podPath, r.containerName, $event)"
                  >
                    <span class="ref-type">{{ r.type }}</span>
                    <span class="ref-target">{{ r.containerName }}</span>
                    <span class="ref-pod">{{ r.podPath }}</span>
                  </div>
                </div>
              </div>
            </div>
          </template>
        </template>

        <!-- Standalone container -->
        <template v-else>
          <div class="container-section">
            <div
              class="container-item"
              :class="{ 'container-active': isContainerActiveByName((item as Container).name) }"
              @click="handleContainerClick(item as Container, $event)"
            >
              <span
                class="container-badge"
                :style="{ background: containerTypeColors[(item as Container).type] }"
              >
                {{ (item as Container).type.charAt(0).toUpperCase() }}
              </span>
              <span class="container-name" :title="(item as Container).signature">{{ (item as Container).name }}</span>
              <span class="container-lines">L{{ (item as Container).startLine }}-{{ (item as Container).endLine }}</span>
            </div>

            <div v-if="isContainerActiveByName((item as Container).name)" class="inline-code" @click.stop>
              <div class="code-toolbar">
                <div class="code-sig">{{ (item as Container).signature }}</div>
                <button class="code-action-btn" @click="popOutCode(item as Container, $event)" title="Pop Out">&#8599;</button>
              </div>
              <div :ref="onEditorMount(item as Container)" class="code-editor"></div>
              <div v-if="(item as Container).references?.length" class="code-refs">
                <div
                  v-for="r in (item as Container).references"
                  :key="r.podPath + '#' + r.containerName"
                  class="ref-item"
                  @click="handleRefClick(r.podPath, r.containerName, $event)"
                >
                  <span class="ref-type">{{ r.type }}</span>
                  <span class="ref-target">{{ r.containerName }}</span>
                  <span class="ref-pod">{{ r.podPath }}</span>
                </div>
              </div>
            </div>
          </div>
        </template>
      </template>
    </div>
  </div>

  <!-- Dot Mode (default) -->
  <div
    v-else
    class="pod-dot-wrapper"
    @click="handleClick"
    :title="tooltipText"
  >
    <Handle type="target" :position="Position.Left" class="handle-hidden" />
    <Handle type="source" :position="Position.Right" class="handle-hidden" />

    <div
      class="pod-dot"
      :class="{ focused: isFocused }"
      :style="{
        width: dotSize + 'px',
        height: dotSize + 'px',
        background: data.dotColor,
      }"
    >
      <span class="dot-count" v-if="containerCount > 0">{{ containerCount }}</span>
    </div>
    <span class="dot-label" :class="{ 'label-focused': isFocused }">
      {{ data.pod.fileName }}
    </span>
  </div>
</template>

<style scoped>
.handle-hidden { opacity: 0 !important; width: 1px !important; height: 1px !important; }

.pod-dot-wrapper { display: flex; flex-direction: column; align-items: center; gap: 4px; cursor: pointer; }
.pod-dot { border-radius: 50%; display: flex; align-items: center; justify-content: center; transition: all 0.25s ease; border: 2px solid transparent; box-shadow: 0 1px 4px rgba(0,0,0,.12); }
.pod-dot:hover { transform: scale(1.15); box-shadow: 0 2px 10px rgba(0,0,0,.2); }
.pod-dot.focused { border-color: #fff; box-shadow: 0 0 0 3px rgba(64,158,255,.5), 0 2px 10px rgba(0,0,0,.15); transform: scale(1.2); }
.dot-count { font-size: 10px; font-weight: 700; color: #fff; text-shadow: 0 1px 2px rgba(0,0,0,.3); }
.dot-label { font-size: 10px; color: #909399; max-width: 80px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; text-align: center; }
.label-focused { color: #303133; font-weight: 600; font-size: 11px; }

.pod-card { background: #fff; border: 2px solid #409eff; border-radius: 8px; padding: 10px 14px; min-width: 260px; max-width: 800px; cursor: pointer; box-shadow: 0 4px 20px rgba(64,158,255,.2); position: relative; }

.resize-handle {
  position: absolute;
  right: 0;
  top: 0;
  bottom: 0;
  width: 6px;
  cursor: col-resize;
  background: transparent;
  transition: background 0.2s;
  border-radius: 0 6px 6px 0;
}

.resize-handle:hover {
  background: rgba(64, 158, 255, 0.3);
}

.card-header { display: flex; flex-direction: column; gap: 2px; margin-bottom: 8px; padding-bottom: 6px; border-bottom: 1px solid #ebeef5; }
.card-package { font-size: 11px; color: #909399; font-weight: 500; }
.card-filename { font-size: 13px; font-weight: 600; color: #303133; }
.card-filepath { font-size: 10px; color: #909399; font-family: monospace; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; opacity: 0.7; }

.card-containers { display: flex; flex-direction: column; gap: 2px; max-height: 1200px; overflow-y: auto; }
.container-section { display: flex; flex-direction: column; }
.method-section { padding-left: 16px; }

.container-item { display: flex; align-items: center; gap: 6px; padding: 4px 6px; border-radius: 4px; cursor: pointer; font-size: 12px; transition: background 0.15s; }
.container-item:hover { background: #f0f7ff; }
.container-item.container-active { background: #ecf5ff; border-left: 2px solid #409eff; }

.group-header { font-weight: 500; }
.method-count { font-size: 10px; color: #909399; background: #f0f0f0; padding: 0 4px; border-radius: 8px; }
.expand-arrow { font-size: 8px; color: #909399; transition: transform 0.2s; display: inline-block; }
.expand-arrow.rotated { transform: rotate(90deg); }
.method-item { font-size: 11px; }

.container-badge { display: inline-flex; align-items: center; justify-content: center; width: 18px; height: 18px; border-radius: 3px; color: #fff; font-size: 10px; font-weight: 700; flex-shrink: 0; }
.container-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: #606266; flex: 1; }
.container-lines { font-size: 10px; color: #c0c4cc; flex-shrink: 0; }

.inline-code { margin: 4px 0 4px 24px; border: 1px solid #e4e7ed; border-radius: 4px; overflow: hidden; cursor: default; }
.code-toolbar { display: flex; align-items: center; background: #f5f7fa; border-bottom: 1px solid #e4e7ed; padding-right: 2px; gap: 2px; }
.code-sig { padding: 4px 8px; font-size: 11px; font-family: monospace; color: #606266; white-space: pre; overflow-x: auto; flex: 1; }
.code-action-btn { border: none; background: transparent; color: #909399; font-size: 13px; cursor: pointer; padding: 2px 6px; border-radius: 3px; flex-shrink: 0; line-height: 1; }
.code-action-btn:hover { background: #e4e7ed; color: #303133; }
.code-editor { height: 200px; min-width: 300px; }
.code-refs { border-top: 1px solid #e4e7ed; padding: 4px 8px; background: #fafafa; }
.ref-item { display: flex; align-items: center; gap: 6px; padding: 2px 0; cursor: pointer; font-size: 11px; }
.ref-item:hover { color: #409eff; }
.ref-type { font-size: 10px; color: #909399; background: #f0f0f0; padding: 0 4px; border-radius: 2px; }
.ref-target { font-weight: 500; color: #409eff; }
.ref-pod { color: #c0c4cc; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
