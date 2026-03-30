<script setup lang="ts">
import { ref, onBeforeUnmount, type ComponentPublicInstance } from 'vue'
import * as monaco from 'monaco-editor'
import { useProjectStore } from '../../stores/project'
import type { FloatingTab } from '../../types'

const props = defineProps<{ tab: FloatingTab }>()
const store = useProjectStore()

let editorInstance: monaco.editor.IStandaloneCodeEditor | null = null
const tabWidth = ref(480)
const tabHeight = ref(300)
const dragging = ref(false)
const resizing = ref(false)
const dragOffset = { x: 0, y: 0 }
const resizeStart = { w: 0, h: 0, mx: 0, my: 0 }

function onEditorMount(el: Element | ComponentPublicInstance | null) {
  if (!el || !(el instanceof HTMLElement)) return
  editorInstance?.dispose()
  editorInstance = monaco.editor.create(el, {
    value: props.tab.sourceCode,
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
  })
}

function startDrag(e: MouseEvent) {
  dragging.value = true
  dragOffset.x = e.clientX - props.tab.x
  dragOffset.y = e.clientY - props.tab.y
  window.addEventListener('mousemove', onDrag)
  window.addEventListener('mouseup', stopDrag)
}

function onDrag(e: MouseEvent) {
  if (!dragging.value) return
  props.tab.x = e.clientX - dragOffset.x
  props.tab.y = e.clientY - dragOffset.y
}

function stopDrag() {
  dragging.value = false
  window.removeEventListener('mousemove', onDrag)
  window.removeEventListener('mouseup', stopDrag)
}

function startResize(e: MouseEvent) {
  e.preventDefault()
  e.stopPropagation()
  resizing.value = true
  resizeStart.w = tabWidth.value
  resizeStart.h = tabHeight.value
  resizeStart.mx = e.clientX
  resizeStart.my = e.clientY
  window.addEventListener('mousemove', onResize)
  window.addEventListener('mouseup', stopResize)
}

function onResize(e: MouseEvent) {
  if (!resizing.value) return
  tabWidth.value = Math.max(300, resizeStart.w + e.clientX - resizeStart.mx)
  tabHeight.value = Math.max(300, resizeStart.h + e.clientY - resizeStart.my)
}

function stopResize() {
  resizing.value = false
  window.removeEventListener('mousemove', onResize)
  window.removeEventListener('mouseup', stopResize)
}

function close() {
  store.closeFloatingTab(props.tab.id)
}

onBeforeUnmount(() => {
  editorInstance?.dispose()
  window.removeEventListener('mousemove', onDrag)
  window.removeEventListener('mouseup', stopDrag)
  window.removeEventListener('mousemove', onResize)
  window.removeEventListener('mouseup', stopResize)
})
</script>

<template>
  <div
    class="floating-tab-wrapper"
    :style="{ left: tab.x + 'px', top: tab.y + 'px', width: tabWidth + 'px', height: tabHeight + 'px' }"
  >
    <div class="floating-tab">
      <div class="tab-header" @mousedown="startDrag">
        <span class="tab-title" :title="tab.podPath + ' / ' + tab.containerName">{{ tab.title }}</span>
        <button class="tab-btn tab-close" @click.stop="close" title="Close">&times;</button>
      </div>
      <div class="tab-sig">{{ tab.signature }}</div>
      <div :ref="onEditorMount" class="tab-editor"></div>
    </div>
    <div class="resize-handle" @mousedown="startResize"></div>
  </div>
</template>

<style scoped>
.floating-tab-wrapper {
  position: absolute;
  z-index: 200;
  display: flex;
  flex-direction: column;
}

.floating-tab {
  width: 100%;
  height: 100%;
  background: #fff;
  border: 1px solid #dcdfe6;
  border-radius: 6px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15);
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.tab-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 10px;
  background: #f5f7fa;
  border-bottom: 1px solid #e4e7ed;
  cursor: grab;
  user-select: none;
}

.tab-header:active { cursor: grabbing; }

.tab-title {
  font-size: 12px;
  font-weight: 600;
  color: #303133;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

.tab-btn {
  border: none;
  background: transparent;
  color: #909399;
  font-size: 16px;
  cursor: pointer;
  padding: 0 4px;
  border-radius: 3px;
  line-height: 1;
  flex-shrink: 0;
}

.tab-btn:hover { background: #e4e7ed; color: #303133; }
.tab-close:hover { background: #f56c6c; color: #fff; }

.tab-sig {
  padding: 4px 10px;
  font-size: 11px;
  font-family: monospace;
  color: #606266;
  background: #fafafa;
  border-bottom: 1px solid #e4e7ed;
  white-space: pre;
  overflow-x: auto;
}

.tab-editor {
  flex: 1;
  min-height: 100px;
}

.resize-handle {
  position: absolute;
  right: -4px;
  bottom: -4px;
  width: 20px;
  height: 20px;
  cursor: nwse-resize;
  z-index: 10;
}

.resize-handle::after {
  content: '';
  position: absolute;
  right: 4px;
  bottom: 4px;
  width: 8px;
  height: 8px;
  border-right: 2px solid #c0c4cc;
  border-bottom: 2px solid #c0c4cc;
}

.resize-handle:hover::after {
  border-color: #409eff;
}
</style>
