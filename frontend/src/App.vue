<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref } from 'vue'
import { Fold, Expand } from '@element-plus/icons-vue'
import { useProjectStore } from './stores/project'
import FileTree from './components/FileTree/FileTree.vue'
import PodGraph from './components/PodGraph/PodGraph.vue'
import AppBreadcrumb from './components/Breadcrumb/AppBreadcrumb.vue'

const store = useProjectStore()
const isCollapsed = ref(false)

const toggleSidebar = () => {
  isCollapsed.value = !isCollapsed.value
}

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
    </el-header>

    <el-container class="app-body">
      <el-aside v-show="!isCollapsed" class="app-sidebar" width="260px">
        <FileTree @set-project="handleSetProject" />
      </el-aside>
      <div class="sidebar-toggle" @click="toggleSidebar">
        <el-icon :size="14">
          <component :is="isCollapsed ? Expand : Fold" />
        </el-icon>
      </div>
      <el-main class="app-main" :class="{ 'app-main-expanded': isCollapsed }">
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

.logo {
  font-size: 18px;
  font-weight: 700;
  color: #409eff;
  letter-spacing: -0.5px;
}

.app-body {
  height: calc(100vh - 48px);
  overflow: hidden;
  display: flex;
}

.app-sidebar {
  border-right: 1px solid var(--border-color);
  background: var(--bg-secondary);
  overflow-y: auto;
  transition: width 0.3s ease, opacity 0.3s ease;
}

.app-main {
  padding: 0;
  overflow: hidden;
  position: relative;
  flex: 1;
  transition: flex 0.3s ease;
}

.sidebar-toggle {
  width: 16px;
  background: var(--border-color);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background-color 0.2s;
  color: var(--text-secondary);
}

.sidebar-toggle:hover {
  background: #409eff;
  color: #fff;
}
</style>
