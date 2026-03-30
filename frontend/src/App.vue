<script setup lang="ts">
import { onMounted, onBeforeUnmount } from 'vue'
import { useProjectStore } from './stores/project'
import FileTree from './components/FileTree/FileTree.vue'
import PodGraph from './components/PodGraph/PodGraph.vue'
import AppBreadcrumb from './components/Breadcrumb/AppBreadcrumb.vue'

const store = useProjectStore()

onMounted(async () => {
  await store.restoreFromUrl()
  window.addEventListener('keydown', handleKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleKeydown)
})

function handleKeydown(e: KeyboardEvent) {
  const isMeta = e.metaKey || e.ctrlKey
  if (isMeta && e.key === '[') {
    e.preventDefault()
    store.goBack()
  } else if (isMeta && e.key === ']') {
    e.preventDefault()
    store.goForward()
  }
}

async function handleSetProject(path: string) {
  await store.loadProject(path)
}
</script>

<template>
  <el-container class="app-container">
    <el-header class="app-header" height="48px">
      <div class="header-left">
        <span class="logo">GoPodView</span>
        <AppBreadcrumb />
      </div>
      <div class="header-right">
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
    </el-header>

    <el-container class="app-body">
      <el-aside class="app-sidebar" width="260px">
        <FileTree @set-project="handleSetProject" />
      </el-aside>
      <el-main class="app-main">
        <PodGraph />
      </el-main>
    </el-container>
  </el-container>
</template>

<style scoped>
.app-container {
  height: 100vh;
  overflow: hidden;
}

.app-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--border-color);
  padding: 0 16px;
  background: var(--bg-primary);
  z-index: 10;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.logo {
  font-size: 18px;
  font-weight: 700;
  color: #409eff;
  letter-spacing: -0.5px;
}

.app-body {
  height: calc(100vh - 48px);
  overflow: hidden;
}

.app-sidebar {
  border-right: 1px solid var(--border-color);
  background: var(--bg-secondary);
  overflow-y: auto;
}

.app-main {
  padding: 0;
  overflow: hidden;
  position: relative;
}
</style>
