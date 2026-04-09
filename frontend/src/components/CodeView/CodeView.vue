<script setup lang="ts">
import { ref, watch, onMounted, onBeforeUnmount, computed } from 'vue'
import * as monaco from 'monaco-editor'
import { useProjectStore } from '../../stores/project'
import type { Reference } from '../../types'

const store = useProjectStore()
const editorContainer = ref<HTMLElement | null>(null)
let editor: monaco.editor.IStandaloneCodeEditor | null = null

const container = computed(() => store.selectedContainer)

onMounted(() => {
  if (editorContainer.value) {
    editor = monaco.editor.create(editorContainer.value, {
      value: container.value?.sourceCode ?? '',
      language: 'go',
      theme: 'vs',
      readOnly: true,
      minimap: { enabled: false },
      fontSize: 13,
      lineNumbers: 'on',
      scrollBeyondLastLine: false,
      automaticLayout: true,
      wordWrap: 'on',
      padding: { top: 12 },
    })
  }
})

watch(container, (newVal) => {
  if (editor && newVal) {
    editor.setValue(newVal.sourceCode ?? '')
  }
})

onBeforeUnmount(() => {
  editor?.dispose()
})

function handleBackToExpanded() {
  if (container.value?.pod) {
    store.expandPod(container.value.pod)
  }
}

async function handleRefClick(ref: Reference) {
  if (!container.value) return
  await store.openReference(container.value.pod, container.value.name, ref)
}
</script>

<template>
  <div class="code-view">
    <div class="code-header">
      <div class="code-info">
        <el-button size="small" text @click="handleBackToExpanded">
          &larr; 返回
        </el-button>
        <el-tag size="small" :type="containerTagType(container?.type)">
          {{ container?.type }}
        </el-tag>
        <span class="code-name">{{ container?.name }}</span>
        <span class="code-lines" v-if="container">
          L{{ container.startLine }}-{{ container.endLine }}
        </span>
      </div>
      <div class="code-signature" v-if="container?.signature">
        <code>{{ container.signature }}</code>
      </div>
    </div>

    <div ref="editorContainer" class="editor-container"></div>

    <div v-if="container?.references?.length" class="references-panel">
      <div class="ref-title">引用</div>
      <div
        v-for="ref in container.references"
        :key="(ref.podPath || ref.importPath || 'ref') + '#' + ref.containerName"
        class="ref-item"
        @click="handleRefClick(ref)"
      >
        <el-tag v-if="ref.isExternal" size="small" type="info">
          external
        </el-tag>
        <el-tag size="small" :type="ref.type === 'call' ? 'primary' : 'warning'">
          {{ ref.type }}
        </el-tag>
        <span class="ref-name">{{ ref.containerName }}</span>
        <span class="ref-pod">{{ ref.podPath || ref.importPath }}</span>
      </div>
    </div>
  </div>
</template>

<script lang="ts">
function containerTagType(type?: string) {
  switch (type) {
    case 'func': return 'primary'
    case 'struct': return 'success'
    case 'interface': return 'warning'
    case 'const': return 'info'
    case 'var': return 'danger'
    default: return 'info'
  }
}
</script>

<style scoped>
.code-view {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: #fff;
}

.code-header {
  padding: 12px 16px;
  border-bottom: 1px solid var(--border-color);
}

.code-info {
  display: flex;
  align-items: center;
  gap: 8px;
}

.code-name {
  font-size: 15px;
  font-weight: 600;
  color: #303133;
}

.code-lines {
  font-size: 12px;
  color: #909399;
}

.code-signature {
  margin-top: 6px;
  font-size: 12px;
  color: #606266;
  background: #f5f7fa;
  padding: 4px 8px;
  border-radius: 4px;
  overflow-x: auto;
}

.code-signature code {
  white-space: pre;
}

.editor-container {
  flex: 1;
  min-height: 200px;
}

.references-panel {
  border-top: 1px solid var(--border-color);
  padding: 12px 16px;
  max-height: 200px;
  overflow-y: auto;
}

.ref-title {
  font-size: 13px;
  font-weight: 600;
  margin-bottom: 8px;
  color: #303133;
}

.ref-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 0;
  cursor: pointer;
  font-size: 12px;
}

.ref-item:hover {
  color: #409eff;
}

.ref-name {
  font-weight: 500;
}

.ref-pod {
  color: #909399;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
