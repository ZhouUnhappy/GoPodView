<script setup lang="ts">
import { useProjectStore } from '../../stores/project'

const store = useProjectStore()
</script>

<template>
  <div class="graph-control" v-if="store.pods.length">
    <span class="control-label">外部依赖</span>
    <el-switch
      :model-value="store.showExternalDeps"
      @update:model-value="(val: boolean) => store.setShowExternalDeps(val)"
      inline-prompt
      active-text="开"
      inactive-text="关"
    />
  </div>
</template>

<style scoped>
.graph-control {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.92);
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.08);
  backdrop-filter: blur(8px);
}

.control-label {
  font-size: 12px;
  color: #52606d;
  font-weight: 600;
  white-space: nowrap;
}
</style>
