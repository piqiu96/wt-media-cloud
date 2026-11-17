// Read-only fixture for defaults returned by the backend platform config file.
// This is prototype data, never an operator-editable browser setting.
export const platformPublishingDefaults = Object.freeze({
  'B站': Object.freeze({ version: 1, partition: '游戏', tags: '{{类目}}', descriptionTemplate: '', visibility: '公开' }),
  '小红书': Object.freeze({ version: 1, partition: '', tags: '', descriptionTemplate: '', visibility: '公开' }),
  '百家号': Object.freeze({ version: 1, partition: '', tags: '', descriptionTemplate: '{{标题}}', visibility: '公开' }),
});
