<script setup>
import './resource-name-link.css'
defineProps({ as: { type:String, default:'button' }, href:String })
</script>
<template><component :is="as" class="resource-name-link" :href="as === 'a' ? href : undefined" :type="as === 'button' ? 'button' : undefined"><slot /></component></template>
