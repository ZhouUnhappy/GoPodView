<script setup lang="ts">
import { computed } from 'vue'
import { useProjectStore } from '../../stores/project'

const store = useProjectStore()

interface BreadcrumbItem {
  label: string
  action: () => void
  active: boolean
}

const items = computed<BreadcrumbItem[]>(() => {
  const result: BreadcrumbItem[] = []

  result.push({
    label: 'All Pods',
    action: () => store.resetView(),
    active: store.viewLevel === 'global',
  })

  if (store.focusedPodPath) {
    const pod = store.podMap.get(store.focusedPodPath)
    result.push({
      label: pod?.fileName ?? store.focusedPodPath,
      action: () => store.focusPod(store.focusedPodPath!),
      active: store.viewLevel === 'focused',
    })
  }

  if (store.expandedPods.size > 0 && store.viewLevel !== 'focused') {
    result.push({
      label: 'Containers',
      action: () => { if (store.focusedPodPath) store.expandPod(store.focusedPodPath) },
      active: store.viewLevel === 'expanded' || store.viewLevel === 'code',
    })
  }

  return result
})
</script>

<template>
  <el-breadcrumb separator="/" class="app-breadcrumb">
    <el-breadcrumb-item
      v-for="(item, idx) in items"
      :key="idx"
    >
      <span
        :class="{ 'crumb-link': !item.active, 'crumb-active': item.active }"
        @click="!item.active && item.action()"
      >
        {{ item.label }}
      </span>
    </el-breadcrumb-item>
  </el-breadcrumb>
</template>

<style scoped>
.app-breadcrumb {
  font-size: 13px;
}

.crumb-link {
  cursor: pointer;
  color: #409eff;
}

.crumb-link:hover {
  text-decoration: underline;
}

.crumb-active {
  color: #303133;
  font-weight: 500;
}
</style>
