<script setup>
import ResourceOverlay from '../../../shared/ui/resource/ResourceOverlay.vue'
import { usageOutputCount,usageOperatorCount,usageOperators } from '../labels.js'
defineProps({visible:Boolean,material:Object})
defineEmits(['update:visible'])
</script>
<template>
 <ResourceOverlay :visible="visible" title="使用情况" :width="560" :footer="false" @close="$emit('update:visible',false)">
        <div v-if="material" class="usage-detail">
          <p class="usage-detail__total">成功成片 {{ usageOutputCount(material) }}</p>
          <!-- 普通运营的读模型只有计数没有姓名（服务端置空 operators），这一行就是
               他们能看到的「被引用情况」；缺字段不渲染，不把未知画成 0 人。 -->
          <p v-if="usageOperatorCount(material) > 0" class="usage-detail__total">使用人数 {{ usageOperatorCount(material) }}</p>
          <ul v-if="usageOperators(material).length" class="usage-detail__list">
            <li v-for="operator in usageOperators(material)" :key="operator.id" class="usage-detail__row">
              <span class="usage-detail__name">{{ operator.name }}</span>
              <span class="usage-detail__count">成片 {{ operator.output_count }}</span>
            </li>
          </ul>
        </div>
       </ResourceOverlay>
</template>
<style scoped>
.usage-detail {font-size:14px;color:var(--wt-text-secondary);}
.usage-detail__list { max-height: 280px; overflow-y: auto; padding:0; list-style:none; }
.usage-detail__row {display:flex;gap:16px;justify-content:space-between;padding:10px 0;border-bottom:1px solid var(--wt-border);}
</style>
