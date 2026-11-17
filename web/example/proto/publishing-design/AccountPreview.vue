<script setup>
import Overlay from "./PrototypeOverlay.vue";
import ResourceDescriptions from "../../../src/shared/ui/resource/ResourceDescriptions.vue";
import ResourceSection from "../../../src/shared/ui/resource/ResourceSection.vue";
import { DescriptionsItem as Field } from "tdesign-vue-next";
defineProps({ account: Object });
defineEmits(["close"]);
</script>
<template>
  <Overlay :visible="!!account" title="发布账号详情" kind="drawer" :width="800" :footer="false" @update:visible="$emit('close')">
    <template v-if="account">
      <ResourceSection title="账号信息"><ResourceDescriptions>
        <Field label="账号名称">{{ account.name }}</Field><Field label="账号 ID">{{ account.id }}</Field>
        <Field label="平台 UID">{{ account.uid || '尚未同步' }}</Field><Field label="发布平台">{{ account.platform }}</Field>
        <Field label="类目" full>{{ account.categories?.join('、') || '未分类' }}</Field>
        <Field label="账号状态">{{ account.enabled ? '启用' : '停用' }}</Field><Field label="登录状态">{{ account.loginValid === false ? '登录已失效' : account.loginValid === true ? '登录有效' : '尚未检查' }}</Field>
      </ResourceDescriptions></ResourceSection>
      <ResourceSection title="绑定窗口"><ResourceDescriptions>
        <Field label="窗口名称">{{ account.windowName || (account.window ? '窗口 ' + account.window : '未绑定窗口') }}</Field>
        <Field label="窗口编号">{{ account.window || '未绑定' }}</Field>
        <Field label="窗口 ID" full>{{ account.profileId || account.browserProfileId || '尚无记录' }}</Field>
        <Field label="窗口状态">{{ account.windowStatus || '尚未检查' }}</Field><Field label="运营备注" full>{{ account.remark || '无' }}</Field>
      </ResourceDescriptions></ResourceSection>
    </template>
  </Overlay>
</template>
