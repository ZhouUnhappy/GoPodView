<script setup lang="ts">
import { useProjectStore } from '../../stores/project'

const store = useProjectStore()
</script>

<template>
  <div class="depth-control" v-if="store.viewLevel !== 'global'">
    <span class="depth-label">依赖层级</span>
    <el-input-number
      :model-value="store.dependencyDepth"
      @update:model-value="(val: number | undefined) => store.setDependencyDepth(val ?? 1)"
      :min="1"
      :max="10"
      size="small"
      controls-position="right"
      style="width: 100px"
    />
  </div>
</template>

<style scoped>
.depth-control {
  display: flex;
  align-items: center;
  gap: 6px;
}

.depth-label {
  font-size: 12px;
  color: #909399;
  white-space: nowrap;
}
</style>
