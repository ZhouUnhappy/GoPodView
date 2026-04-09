<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { Document, Folder } from '@element-plus/icons-vue'
import { useProjectStore } from '../../stores/project'
import type { FileTreeNode } from '../../types'
import type { ElTree } from 'element-plus'

const emit = defineEmits<{
  'set-project': [path: string]
}>()

const store = useProjectStore()
const searchQuery = ref('')
const projectInput = ref(store.projectPath)
const treeRef = ref<InstanceType<typeof ElTree>>()

const treeData = computed(() => {
  if (!store.fileTree) return []
  return store.fileTree.children ?? []
})

const expandedKeys = computed(() => {
  if (!store.focusedPodPath) return [] as string[]
  const parts = store.focusedPodPath.split('/')
  const keys: string[] = []
  for (let i = 1; i < parts.length; i++) {
    keys.push(parts.slice(0, i).join('/'))
  }
  return keys
})

const filterMethod = (value: string, data: FileTreeNode) => {
  if (!value) return true
  return data.name.toLowerCase().includes(value.toLowerCase())
}

function handleNodeClick(data: FileTreeNode) {
  if (!data.isDir) {
    store.focusPod(data.path)
  }
}

function handleLoadProject() {
  const path = projectInput.value.trim()
  if (path) {
    emit('set-project', path)
  }
}

function getNodeIcon(data: FileTreeNode) {
  return data.isDir ? Folder : Document
}

function getNodeClass(data: FileTreeNode) {
  if (!data.isDir && data.path === store.focusedPodPath) {
    return 'tree-node-active'
  }
  return ''
}

async function syncTreeToFocused() {
  const path = store.focusedPodPath
  if (!path || !treeRef.value) return
  await nextTick()
  for (const key of expandedKeys.value) {
    const node = treeRef.value.getNode(key)
    if (node && !node.expanded) {
      node.expand()
    }
  }
  treeRef.value.setCurrentKey(path)
}

watch(() => store.projectPath, (newPath) => {
  projectInput.value = newPath
})

watch(() => store.focusedPodPath, syncTreeToFocused)

watch(treeData, async (data) => {
  if (data.length > 0 && store.focusedPodPath) {
    await nextTick()
    syncTreeToFocused()
  }
})
</script>

<template>
  <div class="file-tree">
    <div class="tree-header">
      <el-input
        v-model="projectInput"
        placeholder="Go project path..."
        size="small"
        clearable
        @keyup.enter="handleLoadProject"
      >
        <template #append>
          <el-button @click="handleLoadProject" :loading="store.loading">
            Load
          </el-button>
        </template>
      </el-input>
    </div>

    <div v-if="store.fileTree" class="tree-search">
      <el-input
        v-model="searchQuery"
        placeholder="Search files..."
        size="small"
        clearable
      />
    </div>

    <div v-if="store.loading" class="tree-loading">
      <el-skeleton :rows="8" animated />
    </div>

    <el-tree
      v-else-if="treeData.length"
      ref="treeRef"
      :data="treeData"
      :props="{ label: 'name', children: 'children', isLeaf: (data: FileTreeNode) => !data.isDir }"
      :filter-node-method="filterMethod"
      :filter-value="searchQuery"
      node-key="path"
      highlight-current
      @node-click="handleNodeClick"
    >
      <template #default="{ data }: { data: FileTreeNode }">
        <span class="tree-node" :class="getNodeClass(data)">
          <el-icon :size="14">
            <component :is="getNodeIcon(data)" />
          </el-icon>
          <span class="tree-node-label">{{ data.name }}</span>
        </span>
      </template>
    </el-tree>

    <div v-else class="tree-empty">
      <p>Enter a Go project path and click "Load"</p>
    </div>
  </div>
</template>

<style scoped>
.file-tree {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.tree-header {
  padding: 12px 12px 8px;
}

.tree-search {
  padding: 0 12px 8px;
}

.tree-loading {
  padding: 12px;
}

.tree-empty {
  padding: 24px 12px;
  text-align: center;
  color: var(--text-secondary);
  font-size: 13px;
}

.tree-node {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  padding: 2px 0;
}

.tree-node-active {
  color: #409eff;
  font-weight: 600;
}

.tree-node-label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>

<style>
.file-tree .el-tree {
  background: transparent;
}

.file-tree .el-tree-node__content {
  background: transparent;
}

.file-tree .el-tree-node__content:hover {
  background: rgba(0, 0, 0, 0.04);
}
</style>
