/* 同一评审入口的内容生产评审书；示意数据不进入工作台，也不代填人工结论。 */
(() => {
  'use strict';
  if (new URLSearchParams(location.search).get('scope') !== 'production') return;
  const E = value => String(value ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  const tag = (text, color = '') => '<span class="pb-tag ' + color + '">' + E(text) + '</span>';
  const button = (text, style = '') => '<span class="pb-btn ' + style + '">' + E(text) + '</span>';
  const link = text => '<span class="pb-link">' + E(text) + '</span>';
  const actions = values => '<div class="pb-actions">' + values.map(v => link(v)).join('') + '</div>';
  const field = (name, value, wide = false) => '<div class="pb-field' + (wide ? ' wide' : '') + '"><label>' + E(name) + '</label><div class="pb-input' + (!value ? ' dim' : '') + '">' + E(value || '请选择') + '</div></div>';
  const table = (heads, rows, wide = false) => '<div class="' + (wide ? 'pb-wide-table' : '') + '"><table><thead><tr>' + heads.map(h => '<th>' + E(h) + '</th>').join('') + '</tr></thead><tbody>' + rows.map(row => '<tr>' + row.map(cell => '<td>' + cell + '</td>').join('') + '</tr>').join('') + '</tbody></table></div>';
  const facts = rows => '<div class="pb-facts">' + rows.map(r => '<div class="pb-fact"><label>' + E(r[0]) + '</label><div>' + r[1] + '</div></div>').join('') + '</div>';
  const message = (text, type = '') => '<div class="pb-message ' + type + '">' + text + '</div>';
  const counts = rows => '<div class="pb-counts">' + rows.map(r => '<div><strong>' + E(r[1]) + '</strong>' + E(r[0]) + '</div>').join('') + '</div>';
  const tabs = (names, active = 0) => '<div class="pb-tabs">' + names.map((s, i) => i === active ? '<b>' + E(s) + '</b>' : '<span>' + E(s) + '</span>').join('') + '</div>';
  const config = (name, content, extra = '') => '<div class="pb-config"><div class="pb-config-head"><span>' + name + '</span>' + extra + '</div>' + content + '</div>';
  const media = (title, sub = '光影研习社') => '<div class="pb-media"><span class="pb-cover">▷</span><div><strong>' + E(title) + '</strong><small>' + E(sub) + '</small></div></div>';
  const pagination = (total = 28) => '<div class="pb-pagination">共 ' + total + ' 条　　20 条 / 页　　‹　' + tag('1', 'blue') + '　2　›</div>';
  const check = '<span class="pb-check"></span>';
  const navItems = [['materials','素材库'],['mine','我的素材'],['strategies','合成策略'],['tasks','合成任务'],['outputs','成片管理']];
  const shell = (route, title, content, primary = '') => '<div class="pb-app"><aside class="pb-app-nav"><div class="pb-app-brand">起飞<small>WT MEDIA · 内容运营</small></div><div class="pb-nav-label">工作台</div><div class="pb-nav-item">运营概览</div><div class="pb-nav-label">内容生产</div>' + navItems.map(n => '<div class="pb-nav-item ' + (n[0] === route ? 'active' : '') + '">' + n[1] + '</div>').join('') + '<div class="pb-nav-label">发布与运营</div><div class="pb-nav-item">发布管理</div><div class="pb-nav-item">作品管理</div><div class="pb-nav-item">互动管理</div></aside><div class="pb-app-top"><span>内容运营 / ' + E(title) + '</span><span>内容运营一组　 ·　 陈小雨　高级运营</span></div><div class="pb-app-content"><div class="pb-app-title"><div><h3>' + E(title) + '</h3><div class="pb-subtitle">页面方案示意 · 样例数据 · 不代表业务功能已实施</div></div><div class="pb-actions">' + primary + '</div></div>' + content + '</div></div>';
  const drawer = (background, title, content, footer = '', wide = false) => background.replace(/<\/div>$/, '<div class="pb-overlay"><div class="pb-drawer' + (wide ? ' wide' : '') + '"><div class="pb-layer-title">' + E(title) + '<span>×</span></div><div class="pb-layer-body">' + content + '</div>' + (footer ? '<div class="pb-layer-footer">' + footer + '</div>' : '') + '</div></div></div>');
  const dialog = (background, title, content, footer = '') => background.replace(/<\/div>$/, '<div class="pb-overlay"><div class="pb-dialog"><div class="pb-layer-title">' + E(title) + '<span>×</span></div><div class="pb-layer-body">' + content + '</div><div class="pb-layer-footer">' + footer + '</div></div></div></div>');
  const filter = (fields, more = '高级筛选') => '<div class="pb-filter">' + fields + '<div class="pb-actions">' + button('查询', 'primary') + button('重置') + '</div><div class="pb-advanced">' + E(more) + '⌄</div></div>';
  const panel = (body, total = 28, toolbar = '') => '<div class="pb-panel">' + (toolbar ? '<div class="pb-table-toolbar">' + toolbar + '</div>' : '') + body + pagination(total) + '</div>';
  const note = text => '<div class="pb-subtitle">' + text + '</div>';
  const reportTable = (heads, rows) => '<div class="pb-table-wrap"><table><thead><tr>' + heads.map(s => '<th>' + E(s) + '</th>').join('') + '</tr></thead><tbody>' + rows.map(row => '<tr>' + row.map(s => '<td>' + E(s) + '</td>').join('') + '</tr>').join('') + '</tbody></table></div>';
  const reportBlock = (title, content) => '<div class="pb-report"><h3>' + title + '</h3>' + content + '</div>';
  const list = values => '<ul>' + values.map(s => '<li>' + E(s) + '</li>').join('') + '</ul>';
  const detailSettings = () => config('片头引导 · 2 份', facts([['适用平台',tag('B站') + tag('百度')],['插入位置','15 秒'],['引导视频时长','0 秒（使用全长）'],['最大成片时长','60 秒']])) + config('轻量引导 · 1 份', '适用平台：B站　 ·　 插入位置 8 秒　 ·　 最大成片时长 45 秒');
  const materialTitles = ['三步掌握室内布光','手机拍出电影感','自然光人像的实用技巧'];

  function libraryList() {
    const rows = materialTitles.map((title, i) => [check,link('10800' + (8-i)),tag('抖音'),link('DEMO-81000' + (8-i)),media(title), '拍摄技巧','11-03 16:2' + i, '浏览 12.8万　点赞 3,620　收藏 832<small>更新于 11-04 08:30</small>',tag(i === 1 ? '已加入' : '未加入',i === 1 ? 'green' : ''),tag('未使用'), '12 个成片 · 4 人','11-04 08:00','陈小雨',actions(i === 1 ? ['详情'] : ['详情','加入'])]);
    return shell('materials','素材库',filter(field('素材搜索','ID / 标题 / 作者',true) + field('来源平台','抖音') + field('类目','拍摄技巧') + field('我的使用','未使用')) + panel(table(['','素材 ID','平台','平台作品 ID','素材信息','类目','发布时间 ↓','内容表现','加入状态','我的使用','团队使用情况','入库时间','操作人','操作'],rows,true),128, '<span>共享素材 · 团队使用仅供选材参考</span>' + button('批量加入我的素材')));
  }
  function libraryDetail() {
    return drawer(libraryList(),'素材详情',media(materialTitles[0]) + '<div style="height:22px"></div><h4>基本信息</h4>' + facts([['素材 ID',link('108008　⧉')],['平台作品 ID',link('DEMO-810008　⧉')],['平台','抖音'],['类目','拍摄技巧'],['发布时间','2026-11-03 16:20'],['原作品',link('查看平台作品 ↗')]]) + '<h4>内容表现</h4>' + counts([['浏览','12.8万'],['点赞','3,620'],['收藏','832']]) + note('更新于 2026-11-04 08:30 · 缺失值显示 —，0 正常保留') + '<div style="height:22px"></div><h4>使用情况</h4>' + facts([['我的使用','未使用'],['团队使用',link('12 个成功成片 · 4 人')]]) + '<h4>视频文件</h4>' + facts([['来源状态',tag('正常','green')],['云端文件','未核验，暂无可访问链接']]) + message('只读详情：关闭后恢复列表位置。加入在列表发起，不在此处准备或下载视频。'));
  }
  function libraryResult() {
    return dialog(libraryList(),'加入我的素材 · 处理结果', counts([['新增','3'],['恢复','1'],['跳过','1'],['失败','1']]) + table(['素材','结果','说明'],[[E(materialTitles[0]),tag('新增','green'),'已加入个人工作集'],[E(materialTitles[1]),tag('跳过'),'此前已加入'],['户外人像构图',tag('失败','red'),'保存未完成，可重试']]) + message('加入仅保存个人关系，不准备视频，不创建合成任务或成片。'),button('关闭') + button('仅重试 1 个失败项','primary'));
  }
  function mineList() {
    const states = [['可用','green','下载'],['准备中','orange',''],['失败','red','下载'],['可用','green','']];
    const rows = states.map((s,i) => [check,link('10800' + (8-i)),tag('抖音'),link('DEMO-81000' + (8-i)),media(materialTitles[i%3]), '拍摄技巧','11-03 16:20','浏览 12.8万 · 赞 3,620<small>藏 832 · 更新 08:30</small>',tag(s[0],s[1]) + (i === 2 ? '<small>本机下载失败，云端可用</small>' : i === 3 ? '<small>当前设备已有可用文件</small>' : ''),tag('未使用'),'12 个成片 · 4 人','11-04 08:10','11-04 08:00','陈小雨',actions(['详情',...(s[2] ? [s[2]] : []),'移出'])]);
    return shell('mine','我的素材',filter(field('素材搜索','ID / 标题 / 作者',true) + field('来源平台','抖音') + field('视频状态','全部状态') + field('我的使用','未使用')) + panel(table(['','素材 ID','平台','平台作品 ID','素材信息','类目','发布时间 ↓','内容表现','视频状态','我的使用','团队使用情况','加入时间','入库时间','操作人','操作'],rows,true),46,'<span>当前个人工作集</span>' + button('批量下载')));
  }
  function mineDetail() {
    return drawer(mineList(),'素材详情',media(materialTitles[0]) + '<div style="height:20px"></div><h4>基本信息</h4>' + facts([['素材 ID',link('108008　⧉')],['平台作品 ID',link('DEMO-810008　⧉')],['平台 / 类目','抖音 / 拍摄技巧'],['加入时间','2026-11-04 08:10']]) + '<h4>内容表现</h4>' + counts([['浏览','12.8万'],['点赞','3,620'],['收藏','832']]) + '<h4>使用情况</h4>' + facts([['我的使用','未使用'],['团队使用',link('12 个成功成片 · 4 人')]]) + '<h4>视频文件</h4>' + facts([['视频状态',tag('可用','green')],['云端视频',link('查看云端视频 ↗（仅示意）')],['本机文件',link('打开所在文件夹（位置预览）')],['核验范围','陈小雨 · 当前设备']]) + message('本页无下载 Footer。视频状态与云端链接同排，本机文件单独展示；换人或换设备后重新核验。'));
  }
  function mineDownload() {
    return dialog(mineList(),'批量下载 · 处理结果',counts([['成功','2'],['失败','1'],['跳过','2']]) + table(['素材','结果','原因'],[[materialTitles[0],tag('成功','green'),'已保存到当前设备'],[materialTitles[1],tag('失败','red'),'本次连接中断，云端文件仍可用于合成'],[materialTitles[2],tag('跳过'),'当前设备已存在可用文件'],['户外人像构图',tag('跳过'),'已有下载进行中']]) + message('下载进度与结果在离线原型中由演示控制推进；这不是实际下载记录。'),button('关闭') + button('仅重试失败项','primary'));
  }
  function strategyList() {
    return shell('strategies','合成策略',filter(field('策略搜索','ID / 策略名称',true) + field('类目','全部类目') + field('启停状态','全部') + field('负责人','全部人员')) + panel(table(['策略 ID','策略名称','模板 / 类目','负责人','执行计划','状态','操作'],[
      [link('ST-012'),'<strong>拍摄技巧 · 引导插入</strong><small>2 组成片设置 · 每条素材 3 份</small>','引导视频插入<small>拍摄技巧</small>','陈小雨','每天 09:00<small>北京时间 · 单次 10 条 / 每日 30 条素材</small>',tag('启用','green'),actions(['详情','编辑','立即执行','复制'])],
      [link('ST-011'),'<strong>剪辑教程 · 画中画</strong><small>1 组成片设置 · 每条素材 2 份</small>','引导画中画<small>剪辑教程</small>','林予安','仅手动执行',tag('启用','green'),actions(['详情','编辑','立即执行','复制'])],
      [link('ST-010'),'<strong>拍摄技巧 · 精选截取</strong><small>1 组成片设置 · 每条素材 1 份</small>','视频截取<small>拍摄技巧</small>','陈小雨','每 60 分钟<small>北京时间 · 单次 5 条素材</small>',tag('停用'),actions(['详情','编辑','复制'])]
    ]),12),button('新建策略','primary'));
  }
  function strategyDetail() {
    return drawer(strategyList(),'策略详情 · 拍摄技巧 · 引导插入',tabs(['策略信息','执行记录']) + facts([['策略 ID',link('ST-012　⧉')],['状态',tag('启用','green')],['类目 / 团队','拍摄技巧 / 内容运营一组'],['负责人','陈小雨'],['模板','引导视频插入'],['执行计划','每天 09:00 · 北京时间']]) + '<h4>默认成片设置</h4>' + detailSettings() + '<h4>选材条件 · D1 已确认</h4>' + message('先执行全部排除，再按选入优先级分配素材；规则位置不改变排除底线。') + '<div class="pb-rule">' + tag('排除','red') + '<span>发布时间超过 30 天</span></div><div class="pb-rule">' + tag('选入 1','blue') + '<span>点赞 ≥ 3,000 且 收藏 ≥ 500</span></div>' + note('规则示例用于解释，不代表当前策略数据；完整条件在详情阅读，不全部挤入列表。'),button('关闭'));
  }
  function strategyRun() {
    return drawer(strategyList(),'策略运行记录',tabs(['策略信息','执行记录'],1) + facts([['触发','立即执行 · 陈小雨'],['时间','2026-11-04 09:12'],['使用配置','本次触发时已保存配置'],['创建结果',tag('部分创建','orange')]]) + counts([['候选素材','8'],['选入素材','3'],['创建任务','8'],['创建失败','1']]) + table(['候选结果（素材口径）','数量','说明'],[['已选入','3','预计 3 × 3 = 9 条任务'],['规则排除','2','命中排除条件'],['历史已制作','1','规则执行跳过'],['数据不足','1','关键收藏指标缺失'],['超过额度','1','本次剩余名额不足']]) + message('3 条选入素材预计拆出 9 条任务，实际成功创建 8 条、失败 1 条。任务创建完成不等于成片制作成功。','warning') + '<div style="height:14px"></div>' + actions(['查看已创建的 8 条任务','查看 1 条创建失败原因']) + note('示意汇总；正式界面只展示真实保存的统计，不编造逐条原因。'),button('关闭'));
  }
  function strategyEditor() {
    return shell('strategies','编辑合成策略','<div class="pb-editor"><div class="pb-two-col"><div><h4>01 · 基本信息</h4><div class="pb-form-grid">' + field('策略名称 *','拍摄技巧 · 引导插入') + field('负责人 *','陈小雨') + field('类目 *（编辑只读）','拍摄技巧') + field('模板 *（编辑只读）','引导视频插入') + '</div><h4>02 · 默认成片设置</h4>' + config('片头引导　 ·　 2 份', '<div class="pb-form-grid">' + field('成片设置名称 *','片头引导') + field('适用平台 *','B站、小红书') + field('份数 *','2') + field('插入位置（秒）','15') + field('引导视频地址','已配置引导视频') + field('引导时长 / 最大时长','全长 / 60 秒') + '</div>',actions(['收起','下移','删除'])) + config('轻量引导　 ·　 1 份','B站 · 插入 8 秒 · 最大时长 45 秒',actions(['展开'])) + button('＋ 添加成片设置') + '</div><div><h4>03 · 执行设置</h4><div class="pb-form-grid">' + field('执行方式 *','定时执行') + field('执行频率 *','每天 09:00') + field('每次上限（素材）*','10 条') + field('每日上限（素材）*','30 条') + '</div>' + note('手动执行仅设每次上限；定时执行增加每日上限和计划。时区：北京时间，保存和启用不立即执行。') + '<div style="height:24px"></div><h4>04 · 选材条件</h4><div class="pb-rule">' + tag('先排除','red') + '<span>发布时间超过 30 天</span></div><div class="pb-rule">' + tag('优先级 1','blue') + '<span>点赞 ≥ 3,000 且 收藏 ≥ 500</span></div><div class="pb-rule">' + tag('优先级 2','blue') + '<span>浏览 ≥ 100,000</span></div>' + message('D1 已确认：排除规则统一先执行；仅选入规则之间有优先级。') + button('预览候选（建议）') + '</div></div><div class="pb-fixed-actions">' + note('保存仅影响后续新任务；已有任务使用创建时快照。') + '<div class="pb-actions">' + button('取消') + button('保存策略','primary') + '</div></div></div>');
  }
  function strategyPreview() {
    return drawer(strategyEditor(),'候选预览 · 只读方案',message('本预览按当前表单草稿解释规则，尚未保存。立即执行仍使用已保存配置。','warning') + counts([['候选','8'],['可选入','3'],['排除','2'],['数据不足','1']]) + table(['素材','结果','说明'],[[materialTitles[0],tag('可选入','green'),'优先级 1：点赞与收藏均满足'],[materialTitles[1],tag('排除','red'),'距发布时间超过 30 天'],[materialTitles[2],tag('数据不足','orange'),'收藏数缺失，无法确定是否命中']]) + message('预览不加入个人素材、不准备视频、不占用额度、不创建任务。实际执行仍须重新核验。'),button('关闭预览'));
  }
  function createBase() {
    return shell('tasks','合成任务',filter(field('任务搜索','ID / 名称',true) + field('状态','全部状态') + field('策略','全部策略')) + panel(table(['本次制作','素材 / 任务','进度','负责人','来源','时间','操作'],[[link('上午拍摄技巧制作'),'3 条 / 9 条','成功 5 / 9','陈小雨','人工创建','11-04 09:20',actions(['详情'])]])),button('新增合成任务','primary'));
  }
  function createTask() {
    return drawer(createBase(),'新增合成任务','<div class="pb-step"><h4>01 · 确定制作范围</h4><div class="pb-form-grid">' + field('任务名称 *','上午拍摄技巧制作') + field('类目 *','拍摄技巧') + field('负责人 *','陈小雨') + '</div></div><div class="pb-step"><h4>02 · 选择合成策略</h4><div class="pb-form-grid">' + field('合成策略 *','拍摄技巧 · 引导插入') + '</div>' + note('模板：引导视频插入 · 默认每条素材产出 3 份') + '</div><div class="pb-step"><h4>03 · 选择素材</h4>' + config('已选 3 条 · 均为拍摄技巧', materialTitles.map(s => '<div style="padding:3px 0">' + E(s) + '</div>').join(''),link('选择 / 调整素材')) + '</div><div class="pb-step"><h4>04 · 核对本次成片设置</h4>' + table(['成片设置','适用平台','每条素材份数','预计成片','参数','操作'],[['片头引导','B站 / 百度','2 份','6 个',tag('策略默认'),link('配置')],['轻量引导','B站','1 份','3 个',tag('已调整','blue'),link('配置')]]) + message('本次修改只影响本次任务；下一次新建仍带入策略默认值。') + '</div>', '<span style="margin-right:auto;font-size:13px">3 条素材 ×（2＋1 份）＝ <b>9 条任务</b><br><span class="pb-subtitle">2 条素材可用 · 1 条等待准备；平台数不额外翻倍</span></span>' + button('取消') + button('创建任务','primary'),true);
  }
  function createPicker() {
    return dialog(createBase(),'选择素材',note('类目：拍摄技巧 · 负责人：陈小雨') + tabs(['我的素材','素材库'],1) + filter(field('综合搜索','素材名称 / 作者 / ID',true) + field('发布时间','开始日期　至　结束日期')) + table(['选择','素材 ID','素材 / 作者','发布时间','浏览 / 赞 / 收藏','我的使用','团队使用'],materialTitles.map((title,i)=>[check,'10800'+i,media(title),'2026-11-03 16:20','12,800 / 3,620 / 832',i?'未使用':'已合成 2 个',link('已合成 4 个')])) + pagination(46),'<span style="margin-right:auto">已选 3 条 · 跨页保留</span>' + button('取消') + button('使用所选','primary'));
  }
  function createAgain() {
    return dialog(createTask(),'确认再次制作',message('所选 3 条素材中，2 条已有同策略制作记录。本次将创建新任务，保留原任务和成片。','warning') + facts([['本次任务数量','9 条'],['参数','本次成片设置快照'],['已有结果','保持不变'],['自动策略规则','仍跳过已有记录，不受本次确认影响']]) + note('D2 已确认。重复点击或响应重试不视为再次制作；失败任务的技术重试保留原任务 ID。'),button('返回检查') + button('确认再次制作','primary'));
  }
  function taskList() {
    return shell('tasks','合成任务',filter(field('任务搜索','任务 / 本次制作 ID 或名称',true) + field('状态','全部状态') + field('策略','全部策略') + field('负责人','全部人员')) + panel(table(['','批次 ID / 任务名称','素材 / 任务','完成情况','待处理','负责人 / 来源','创建时间','操作'],[
      [check,link('MAKE-024') + '<small>上午拍摄技巧制作</small>','3 条素材 / 9 条任务','成功 5 / 9<small>结束 7 / 9</small>',tag('失败 1','red') + tag('处理中 2','blue') + tag('取消 1'),'陈小雨<small>人工创建</small>','11-04 09:20',actions(['详情'])],
      [check,link('MAKE-023') + '<small>拍摄技巧 · 09:00 规则执行</small>','3 条素材 / 9 条任务','成功 9 / 9',tag('全部成功','green'),'陈小雨<small>策略定时</small>','11-04 09:00',actions(['详情'])],
      [check,link('TASK-001') + '<small>历史任务 · 未记录创建关联</small>','1 条素材 / 1 条任务','失败 1 / 1',tag('失败','red'),'林予安<small>人工创建</small>','11-03 18:00',actions(['详情'])]
    ]),24,'<div class="pb-view"><b>任务批次</b><span>任务列表</span></div><span>批量操作仅针对明确勾选范围</span>'),button('新增合成任务','primary'));
  }
  function taskDetail() {
    return drawer(taskList(),'任务详情 · TASK-024-06',tabs(['任务信息','执行记录']) + message('<b>失败 · 合成阶段</b><br>本次执行未取得可用源文件。修复文件后，可按允许的操作重试原任务。','error') + facts([['任务名称','上午拍摄技巧制作 · 片头引导'],['负责人','陈小雨'],['来源素材',link(materialTitles[1])],['来源策略',link('ST-012 · 拍摄技巧 · 引导插入')],['本次制作',link('MAKE-024')],['创建时间','2026-11-04 09:20']]) + '<h4>本任务配置快照</h4>' + config('片头引导 · 第 2 份 / 共 2 份',facts([['本任务产出','1 个成片'],['适用平台',tag('B站') + tag('百度')],['插入位置','15 秒'],['最大时长','60 秒']])) + '<h4>生成结果</h4>' + note('尚无成功成片；不根据模拟完成按钮直接判定正式执行成功。') + message('技术重试沿用任务 ID 和这份快照；需要调整参数时创建新任务。'),button('关闭') + button('重试原任务','primary'));
  }
  function taskLogs() {
    return drawer(taskList(),'任务详情 · 执行记录',tabs(['任务信息','执行记录'],1) + config('第 2 次尝试 · 失败',tag('合成阶段失败','red') + '<div class="pb-log"><div><b>09:26:00 · 开始执行</b><small>使用创建时配置快照</small></div><div><b>09:26:02 · 源文件检查</b><small>文件不可读取</small></div><div><b>09:26:03 · 本次执行失败</b><small>未生成成片 · 可用操作由实际能力决定</small></div></div>') + config('第 1 次尝试 · 失败',note('历史摘要存在，未保存逐步日志；不补造成功步骤。')) + message('记录为产品展示示例。正式执行只展示实际保存的步骤与结果。'),button('关闭'));
  }
  function outputList() {
    return shell('outputs','成片管理',filter(field('综合搜索','名称 / 素材 ID / 批次 ID',true) + field('类目','全部') + field('适用平台','全部') + field('发布情况','全部')) + panel(table(['素材 ID','素材 / 作者','类目','使用策略','实际成片','最近生成','操作'],materialTitles.map((title,i)=>['10800'+i,media(title),'拍摄技巧','片头引导 / 视频截取',String(6-i)+' 个','2026-11-04 09:22',actions(['详情'])])),3,'<div class="pb-view"><b>成片批次</b><span>成片列表</span></div><span>按素材 ID 汇总跨策略、跨任务批次的成功产出</span>'));
  }
  function outputDetail() {
    return drawer(outputList(),'成片详情','<h4>基本信息</h4>' + facts([['成片 ID','OUT-024-01'],['视频状态',tag('可用','green')],['成片名称','三步掌握室内布光 · 片头引导'],['负责人 / 生成时间','陈小雨 / 2026-11-04 09:22']]) + '<h4>视频信息</h4>' + facts([['视频文件',link('三步掌握室内布光 · 片头引导 · OUT-024-01.mp4')],['时长 / 分辨率 / 文件大小','— / — / —']]) + '<h4>制作来源</h4>' + facts([['来源素材',link(materialTitles[0])],['来源策略 / 成片设置','拍摄技巧 · 引导插入 / 片头引导'],['合成任务',link('TASK-024-01')],['任务批次',link('MAKE-024')]]) + '<h4>发布情况</h4>' + table(['适用平台','发布情况','关联任务'],[['B站',tag('未安排'),'—'],['百度',tag('已安排','blue'),link('PUB-031')]]),button('下载'));
  }
  function outputPublish() {
    return dialog(outputList(),'选择本次发布平台',message('已选 2 个成片，共同适用平台为B站、百度。请明确选择平台，再进入发布模块安排账号。') + '<div class="pb-form-grid">' + field('发布平台 *','B站') + '</div>' + facts([['带入发布','所选 2 个成片 ID 与对应文件'],['本页边界','此时尚未创建真实发布结果'],['无共同平台','保留已选项，提示按平台拆分'],['占用冲突','提交前再次核验，解释不可用项']]) + note('平台适用性、发布用途转交与真实发布成功是不同事实。详细发布配置在下一模块评审。'),button('取消') + button('继续安排发布','primary'));
  }

  const reports = [
    {
      id:'page-materials', code:'01', name:'素材库', route:'materials', status:'已优化基线 · 本轮复核',
      purpose:'判断内容是否值得选入个人工作集；团队使用作为参考，不代表素材被独占。',
      scenes:[['列表与筛选',libraryList,'宽表保留完整字段。加入状态、我的使用与团队使用是三个独立事实；加入动作在列表完成。'],['只读详情',libraryDetail,'小封面＋基本信息＋内容表现＋使用情况＋视频文件；不放固定 Footer，不在详情触发下载。'],['批量加入结果',libraryResult,'新增、恢复、跳过、失败分别计数；重试只处理失败，不重做成功项。']],
      current:'13 个业务字段、平台及类目多选、角色默认范围、只读详情与批量加入已接入当前原型。加入仅维护个人关系。',
      change:'本轮主要复核已优化成果与生产后续的衔接；不重新增加制作入口，也不把“团队使用过”改成禁用条件。',
      path:['按平台、类目和内容表现筛选','打开详情判断内容与来源','行内或批量加入我的素材','到合成任务新增，或在其选择器直接选库素材'],
      fields:'素材 ID → 平台 → 平台作品 ID → 素材信息 → 类目 → 发布时间 → 内容表现 → 加入状态 → 我的使用 → 团队使用情况 → 入库时间 → 操作人 → 操作（13 个业务字段；选择框另计）。',
      interactions:[
        ['默认与查询','我的使用默认未使用；普通／高级运营默认负责类目，管理员默认全部启用类目。发布时间倒序。输入为查询草稿，查询后生效。'],
        ['内容表现','浏览、点赞、收藏横向展示，更新时间独立。未知值为 —，0 正常显示，缺失排序置后。'],
        ['加入与恢复','未加入新增关系；已移出恢复关系；已加入跳过。不准备视频，不创建任务、成片或发布安排。'],
        ['团队使用','只统计成功成片数及对应人数；普通运营看汇总，管理角色看授权人员明细。失败任务、下载与发布次数不混入。'],
        ['详情与返回','两个 ID 提供复制，已知作品链接可访问。关闭详情恢复筛选、页码、选择和滚动位置。']
      ],
      exceptions:[['部分加入失败','成功与恢复项保留，失败显示原因且可单独重试。'],['来源失效／云端未知','来源状态与文件事实分开；未知不伪造链接，不直接判定已生成文件丢失。'],['无匹配／无权限','无匹配提供重置；权限不足不泄露团队人员明细。']],
      checks:['从库加入后，任务数、成片数及视频准备状态不变。','未知指标和 0 的显示、排序正确。','普通运营无法看到其他人员姓名。','关详情、换页、返回页面后上下文符合当前原型约定。'],
      focus:'如需补充，请优先指出“加入／我的使用／团队使用”的业务含义是否仍有歧义；本页暂不扩大功能范围。'
    },
    {
      id:'page-mine',code:'02',name:'我的素材',route:'mine',status:'已优化基线 · 本轮复核',
      purpose:'整理当前个人工作集，了解视频准备及本机文件情况；素材制作资格不以手工下载为前提。',
      scenes:[['列表与视频状态',mineList,'可用不等于当前设备已有文件；本机下载失败但云端可用时，仍可用于合成。'],['详情与本机文件',mineDetail,'视频状态与云端链接并排，本机位置独立；详情无下载按钮，演示只预览文件夹位置。'],['混合下载结果',mineDownload,'已有本机文件或进行中下载跳过；部分失败只重试失败项，成功项不回滚。']],
      current:'14 个业务字段、五种视频状态、个人列表默认未使用、列表下载、移出、当前用户与演示设备隔离已接入。',
      change:'沿用已优化方案。将这里的设备文件事实与后续成片管理统一理解；不强制运营先下载到本机再创建合成任务。',
      path:['查看当前个人工作集','按视频状态与使用情况筛选','需要本机副本时在列表下载','制作从合成任务发起；无需手工下载前置'],
      fields:'素材 ID → 平台 → 平台作品 ID → 素材信息 → 类目 → 发布时间 → 内容表现 → 视频状态 → 我的使用 → 团队使用情况 → 加入时间 → 入库时间 → 操作人 → 操作（14 个业务字段；选择框另计）。',
      interactions:[
        ['工作集与默认值','只展示当前有效个人关系，移出后去素材库重新加入恢复；不再保留“素材范围”筛选。平台／类目默认全部，个人使用默认未使用。'],
        ['视频状态','待准备／准备中／可用／下载中／失败；未知显示 —。准备中与下载中不重复发起。失败区分准备或本机下载原因。'],
        ['本机副本','本人＋当前电脑＋已核验文件共同决定下载与打开入口。历史下载记录不能证明当前文件仍在。'],
        ['详情','保留基本信息、内容表现、使用情况和视频文件；不在详情增加下载 Footer。文件夹操作在离线原型仅预览位置。'],
        ['移出与恢复','移出需确认，只移除个人关系。任务、成片、发布与已下载文件保留；恢复不自动准备或下载。'],
        ['批量下载','区分本页勾选与明确的筛选范围；结果给成功、失败、跳过。只重试可重试失败项，完成前保护关闭。']
      ],
      exceptions:[['本机下载失败','云端可用仍可生产；提示连接或本机原因，并保留下载重试。'],['换设备／文件丢失','重新核验，可再次下载；不显示旧电脑的有效文件夹入口。'],['移出时已有任务','原有任务继续，历史结果可追溯；不级联取消或删除。']],
      checks:['五种状态与下载按钮显隐一致。','同一用户换设备后不复用旧本机状态。','移出再从库恢复，不删除历史，也不启动准备。','任务选择器能选用云端可用但本机未下载的素材。'],
      focus:'本页继续作为已优化基线；请补充真实下载场景和移出规则中的遗漏，不重新拆出第三套素材状态。'
    },
    {
      id:'page-strategies',code:'03',name:'合成策略 · 列表与详情',route:'strategies',status:'现有能力保留 · 信息层级待优化',
      purpose:'管理可复用的制作配置与规则执行计划，看懂策略产生了哪些任务、为什么跳过素材。',
      scenes:[['策略列表',strategyList,'列表只保留执行时间、上限及产出摘要；完整规则进入详情。立即执行继续直接使用已保存配置。'],['策略信息',strategyDetail,'默认成片设置、执行设置与选材条件分区阅读。D1 已确认，但规则解释器仍需修改。'],['一次运行的结果',strategyRun,'候选／选入按素材计，创建按任务计；示例中 3 条素材预计 9 条任务，8 条创建成功、1 条失败。']],
      current:'已有列表、详情、策略信息／运行记录、启停、编辑、复制和立即执行。立即执行已按此前人工反馈取消参数窗及确认窗。',
      change:'压缩列表执行计划的冗长规则；详情完整解释默认值与执行逻辑；运行记录补清素材数、任务数、零任务及部分失败原因。',
      path:['在列表按类目、负责人、状态找策略','详情核对默认成片设置与规则','需要更改参数先编辑保存','立即执行直接触发已保存策略，行内加载并进入运行结果'],
      fields:'策略 ID、策略名称（设置组数／每素材份数摘要）、模板、类目、负责人、执行计划、启停状态、操作；团队归属在管理范围或详情表达，不为单团队列表重复加列。',
      interactions:[
        ['两条制作路径','策略执行从团队同类目素材按规则选材；人工创建由运营选材。两者复用默认设置，不把自动选材强制加入个人素材。'],
        ['立即执行','保留直接执行，不重新增加确认弹窗。只执行已保存配置；编辑参数走编辑，手选素材走新增任务。提交中防重复触发。'],
        ['启停','确认影响后请求；处理中保持原值并禁用重复切换，成功再更新、失败回退。停用不取消已有任务。'],
        ['D1 选材规则（已确认）','先统一排除，再按选入优先级；缺失指标不凭其他指标推算。数据不足的细化判定属于本轮建议。'],
        ['D2 重复生产（已确认）','立即执行与定时仍跳过已有同策略／同素材记录；人工明确确认才创建新任务。'],
        ['执行记录','触发动作可区分立即执行／定时；每次运行看候选、选入、跳过及创建结果，关联任务继续跟踪执行。'],
        ['额度与时间（建议）','单次与每日上限按素材计；立即执行与定时共享额度，人工自主选材不占规则额度。时区明确为北京时间，不推算不存在的下次运行时间。']
      ],
      exceptions:[['零任务','分别解释无候选、历史已制作、额度不足、素材不可用、归属失效或创建失败；不统一写“执行成功”。'],['部分创建失败','已创建任务保留，失败项独立展示；只展示实际保存的原因，不编造明细。'],['未知指标','缺失收藏不以点赞推算；缺失距发布时间不使用固定天数。'],['策略停用／权限变化','执行时重新校验并说明原因；预览和列表中的旧状态不保证提交可执行。']],
      checks:['点击立即执行不出现参数窗或确认窗。','交叠排除与选入规则按 D1 得到同义结果。','多份设置消耗的是素材额度，任务数量另计。','零任务与部分失败可解释，能进入相应任务。'],
      focus:'本页重点评审：列表摘要是否够用、运行结果是否能解释运营问题；D1／D2 不重复决策。'
    },
    {
      id:'page-strategy-editor',code:'04',name:'合成策略 · 新建与编辑',route:'strategies',status:'配置结构沿用 · 规则与预览为建议',
      purpose:'一次维护可复用的默认成片设置、自动选材条件与执行计划；改策略只影响未来任务。',
      scenes:[['四区配置表单',strategyEditor,'基本信息 → 默认成片设置 → 执行设置 → 选材条件。模板决定参数，份数、平台与设置名称分开。'],['只读候选预览',strategyPreview,'新增建议：解释当前表单草稿的规则结果，明确未保存；不成为立即执行的必经步骤，也不产生业务副作用。']],
      current:'已有基本信息、成片设置、执行设置和条件组；设置支持增删排序。编辑现有类目、模板与团队只读。',
      change:'保持四区配置，统一术语与校验；将排除从选入优先级中分离；建议添加可选的候选预览，并显式标注草稿或已保存配置。',
      path:['确定名称、类目、模板、负责人','配置至少一组成片设置与份数','配置手动或定时及素材额度','维护先排除后选入规则，可选预览后保存'],
      fields:'基本信息：名称、类目、模板、负责人及团队上下文。每组成片设置：名称、适用平台（必选至少一个）、份数、模板参数。执行方式必选：手动只设每次上限；定时另设每日上限、每日时间或间隔。条件：排除规则与有序选入规则。',
      interactions:[
        ['模板与参数','视频截取用起止范围；引导插入／画中画用引导视频、插入位置、引导时长、最大时长。0 的“全长／不限”含义就地说明。'],
        ['成片设置','至少一组，名称后紧接必选平台，份数 1–20；新增组使用模板初始值，复制本组显式沿用参数，后续各自独立，保留增删排序。'],
        ['编辑与复制','编辑不改变已绑定的类目、模板与团队；复制清除身份、运行历史与统计。本轮建议副本默认停用，保存不立即执行。'],
        ['计划','执行方式必选，手动保留每次上限，定时增加每日上限、时间／间隔及北京时间。切回手动不受隐藏每日上限约束；保存、启用或修改计划不立即执行。'],
        ['D1 与缺失值','排除始终先执行，再分配选入优先级。涉及未知字段且无法确定结论时建议归为数据不足；可确定的逻辑不必一律拒绝。'],
        ['候选预览（建议）','编辑中按表单草稿预览，明确尚未保存；只读且无个人加入、准备、额度或任务副作用。实际执行仍用已保存配置并重新核验。'],
        ['快照隔离','策略默认值供后续创建深拷贝；修改策略不得改写已有任务配置。任务本次覆盖也不回写策略。']
      ],
      exceptions:[['必填或参数非法','字段级提示并定位首个错误，保留全部输入；空名称、无设置／平台、非法份数或时间均不能保存。'],['切换依赖对象','新增时变更模板等会丢失配置的操作先说明影响；关闭未保存表单有丢弃保护。'],['负责人／类目失效','旧数据可读，新保存需重新核验；不静默转给当前操作人。'],['预览失败','保留配置，可修正后再预览；不把预览估计数当作执行承诺。']],
      checks:['三种模板逐项核对字段、单位及 0 的含义。','改策略后旧任务快照不变；改任务后策略默认值不变。','副本不继承运行历史或自动触发执行。','预览无业务副作用，草稿预览与保存后执行口径明确。'],
      focus:'请集中补充模板参数的实际约束、规则逻辑与复制默认停用是否合适。任务中的可调整字段以模板声明为准，固定参数禁用。'
    },
    {
      id:'page-compose-create',code:'05',name:'新增合成任务与素材选择器',route:'tasks',status:'工作台已接入 · D2／D3 待完整核对',
      purpose:'在一个连续流程选定本次素材与参数，提交前知道会生成多少任务、哪些需要等待。',
      scenes:[['四段创建流程',createTask,'沿用已接入工作台的四段流程。示意中的“创建任务”为建议用词；当前原型按钮仍为“保存”。'],['素材选择器',createPicker,'同类目内支持我的素材／素材库，跨页保留与去重；确认选用才加入任务负责人个人素材并准备。'],['人工再次制作',createAgain,'D2 已确认：人工明确确认后创建新任务。此确认与技术重试、防重复点击分别处理。']],
      current:'类目→策略→素材→本次设置已接入；任务名称当前必填，参数可调且不回写策略。选用库素材会加入负责人工作集并开始演示准备。',
      change:'保留当前创建体验。按钮建议“创建任务”；按 D2 统一再次制作，按 D3 展示准确任务数量；补可生产资格、部分失败及不确定提交结果的恢复。',
      path:['填写任务名称、类目与负责人','选择当前类目和负责人适用策略','在同页选择器选素材并确认使用','核对本次参数、份数与等待情况，创建后进入本次制作'],
      fields:'任务名称（当前必填）、类目、负责人（普通运营为本人）、可用策略、所选素材、每组成片设置及模板参数、素材数／每条份数／任务总数／等待素材数。',
      interactions:[
        ['类目与负责人','先确定范围；更换时确认清空失配策略、素材与设置，保留任务名称。已经确认的个人加入及准备不会因清空表单而撤销。'],
        ['策略切换','保留仍符合范围的素材，重置本次设置为新策略默认值；若本次参数已改，先说明覆盖影响。'],
        ['素材选择','我的素材／素材库共享所选集合，同一素材只计一次。取消选择器保留父表单；使用所选才确认加入负责人工作集及准备。'],
        ['参数覆盖','策略默认值深拷贝。本次每组设置作用于全部所选素材；每行配置打开参数弹窗，只允许模板声明的字段调整，其他参数禁用。'],
        ['D3 数量（已确认）','素材数 × 各设置份数之和＝任务数。3 条 ×（2＋1 份）＝9 条；平台标签不翻倍。拆开每条任务产出 1 个成片。'],
        ['D2 再次制作（已确认）','说明已有同策略素材记录，明确确认后创建新任务；旧任务／成片保留。失败重试保留原任务，重复请求不额外创建。'],
        ['提交结果','提交中防重复；全成功直达本次制作；部分失败保留已创建结果及失败范围；响应未知先查本次结果，不盲目再次提交。'],
        ['再次制作','带入来源素材与当前有效策略，在新建表单明确核对后创建新任务；若原策略停用或失效，提示重新选用有效策略。']
      ],
      exceptions:[['没有可用策略','解释类目／负责人／启停原因，保留任务名称和范围；不自动套用其他人的策略。'],['源作品失效','已有可用云端源文件时不机械拒绝；可准备与永久不可用分别处理，本机未下载不构成阻断。'],['取消主表单','未提交任务不创建；此前已确认选用的关系与准备继续保留。'],['创建部分失败','成功项不回滚，失败项与原因留下；重试仅失败范围，实际创建数单列。']],
      checks:['3 条素材 ×（2＋1）准确创建 9 条任务，两个平台不变成 18 条。','改本次参数不改策略；下一次新建恢复策略默认值。','跨来源去重、取消选择器、更换类目与负责人各自副作用正确。','再次制作、技术重试、提交重放三种操作不会混为一谈。'],
      focus:'本页已有基础体验，请补充实际必填项、单次参数覆盖范围与提交后希望看到的结果位置；D2／D3 沿用已确认方案。'
    },
    {
      id:'page-tasks',code:'06',name:'合成任务 · 列表、详情与恢复',route:'tasks',status:'已有分组基础 · 状态与恢复待细评',
      purpose:'先看一次制作的整体进度，再定位单条问题；处理失败时不重做已经成功的内容。',
      scenes:[['本次制作视图',taskList,'当前视图为任务批次，行内仅详情，进入后查看全局进度。成功数、结束数与待处理分别表达，全部结束不等于全部成功。'],['单条任务详情',taskDetail,'先状态与下一步，再任务信息和配置快照，成片只在生成结果出现。重试动作以原型评审后的权限为准。'],['逐次执行记录',taskLogs,'按尝试次数保留记录；没有历史步骤就说明缺失，不补造固定的准备成功日志。']],
      current:'默认按创建批次分组，另有全部任务；已有 pending／running／success／failed／cancelled、详情、失败重试及模拟完成／取消。',
      change:'默认视图为“任务批次／任务列表”；明确分组统计与筛选范围；取消等待实际停止确认，执行记录按尝试保留，演示操作独立标记。',
      path:['从本次制作查看总体成功与待处理','点击失败数带入本次失败范围','打开单任务查看原因与配置快照','修复后重试原任务，或调整参数创建新任务'],
      fields:'本次制作：标识／名称、素材数／任务数、成功与结束情况、待处理、负责人／来源、创建时间、操作。任务明细：任务 ID／名称／设置、素材、类目、策略、负责人、状态／阶段、真实进度、创建／完成时间。',
      interactions:[
        ['分组定义','一次人工提交或一次策略运行归为本次制作；不另建用户维护的业务实体。历史无可靠关联的任务独立显示，不按相近时间或名称猜批次。'],
        ['计数','成功成片数只统计成功任务。结束数可含成功、失败、取消；本次范围内的匹配数与总进度分母分别显示。'],
        ['状态与阶段','等待素材为任务的阶段或原因，避免再增加一套主状态。排队不伪造百分比，处理中只展示真实可得进度。'],
        ['重试','当前原型支持失败任务重试。技术重试沿用任务 ID 与配置快照、追加尝试记录；成功任务不提供普通重试。'],
        ['取消','当前原型是本地模拟直接取消；正式流程建议请求后等执行端确认停止，才显示已取消，失败保持原状态并提示。'],
        ['详情','任务信息／执行记录两页签足够；状态、失败原因、下一步在前。实际配置用创建时快照，当前策略只作追溯链接。'],
        ['批量','仅对明确所选 ID 处理适用项；成功项不重做，已结束项不取消。若整次操作，先说明总数、适用数及跳过原因。'],
        ['权限与不确定结果','普通运营重试权限、取消后能否重试及结果不明确时的核对路径，作为本轮待评审输入；不直接按旧 PRD 收紧现有按钮。']
      ],
      exceptions:[['部分成功','5 成功＋1 失败＋1 取消＋2 处理中显示成功 5/9、结束 7/9，不能显示全部成功。'],['准备反复失败','展示准备失败原因与恢复动作；本轮建议限制自动重准备次数，具体次数待业务核对。'],['结果不明确','建议先核对执行结果，避免直接重试产生额外成片；新增能力不能只改本地状态冒充完成。'],['历史快照或日志缺失','明确“未保存／不可见”，不拿当前配置和固定步骤补历史。']],
      checks:['混合终态的成功数与结束数正确。','筛选失败后批量操作不扩大到整次制作。','技术重试 ID、快照保持，尝试历史增加。','取消请求失败不提前显示取消；未知结果不盲目重试。'],
      focus:'本页需要集中补充：普通运营是否可以重试、取消后如何继续、何种情况下需要人工核对。当前能力与本轮建议已分开，未擅自改权限。'
    },
    {
      id:'page-outputs',code:'07',name:'成片管理',route:'outputs',status:'素材聚合已修正 · 待人工体验',
      purpose:'找到真实生成的文件，预览、下载或带入发布；能追溯制作来源，避免把下载和发布当成同一状态。',
      scenes:[['成片批次',outputList,'默认成片批次按素材 ID 汇总跨策略、跨任务批次的产出。详情看来源、平台及视频状态，不单列本机下载情况。'],['成片详情',outputDetail,'基本信息、视频信息、制作来源和发布情况四区；不显示视频占位，元数据未知显示 —。'],['批量发布衔接',outputPublish,'多个共同平台时明确选择，不静默取第一个；无共同平台保留选择，让运营按平台拆分。']],
      current:'已实施素材 ID 聚合的成片批次与逐个文件的成片列表，详情保留真实任务来源，下载按人和设备隔离。',
      change:'列表仅保留业务信息与统一视频状态，视频信息在详情直接展示；发布收敛到明确勾选文件后的去发布，沿用统一新建弹窗。',
      path:['搜索或按素材定位成片','看视频信息、来源及发布情况','下载当前设备副本，或带选中文件去发布','需要新效果时再次制作新任务，保留旧成片'],
      fields:'列表：成片 ID、名称／生成时间、来源素材、类目、负责人、适用平台、视频状态及操作。详情分为基本信息、视频信息、制作来源和发布情况；素材成片详情不单列下载情况。',
      interactions:[
        ['名称与文件名','成片名称为素材标题 · 成片设置名称，ID 独立展示；视频文件名保留 ID，避免同名文件混淆。'],
        ['视图与查询','默认按素材 ID 汇总成片，最近生成时间倒序；筛选先应用再聚合，不扩大数据范围，不混入其他筛选外成片。'],
        ['文件信息','时长、格式、分辨率、大小来自实际文件元数据；模板时间范围不冒充实际视频时长，未知显示 —。'],
        ['下载与打开','参考我的素材的本人＋设备＋文件核验；进行中定位既有下载，不重复创建。核验存在才提供可用打开入口。离线演示不能冒充系统能力。'],
        ['发布','沿用当前入口，带成片 ID 和文件去发布模块选择账号及方式。平台适用性、占用与成功结果分别表达。'],
        ['批量发布','有多个共同适用平台时显式选择；没有共同平台则保留选择、提示按平台拆分。提交前重新核验文件和占用。'],
        ['再次制作','创建新任务与新成片，旧文件保留；发布流程不暗中生成“平台适配版”。'],
        ['素材成片详情','同一素材跨策略、跨任务批次的产出统一查看；每个文件保留原任务来源，勾选后下载或去发布。']
      ],
      exceptions:[['预览链接失效','保留其余详情并支持刷新地址／重试；不因此删除成片记录。'],['源素材失效','成片文件仍存在则继续可用；文件本身丢失才报文件问题，不自动重做。'],['换人、换设备、文件丢失','当前设备下载状态重新核验，旧全局 downloaded 标记不能作为真实可用证明。'],['平台已占用／无共同平台','保留所选范围，解释不可用平台；其他适用平台独立判断，具体释放规则在发布模块评审。']],
      checks:['命名与下载文件名使用同一真实成片 ID。','未知元数据与预览失败不编造或抹掉其他信息。','同素材跨策略、跨任务批次只聚合一行，详情范围与筛选一致。','多平台批量发布明确选平台，无共同平台不丢选择。'],
      focus:'本页核对按素材汇总及真实任务来源、设备下载状态和勾选发布。发布细则留在下一模块的整体评审。'
    }
  ];

  const ids = reports.map(p => p.id);
  window.OPSProductionReview = {ids, title:'内容生产逐页产品与交互评审', revision:'2026-11-04-production-book'};
  document.body.classList.add('production-book');
  document.querySelector('.review-header').innerHTML = '<div><strong>内容生产 · 产品与交互评审</strong><small>2026-11-04 · 页面示意＋详细报告 · 待人工评审</small></div><nav class="review-actions"><a href="#page-review">评审总览</a><a href="#global-review">集中补充意见</a><a href="index.html#tasks" target="_blank" rel="noopener">打开当前原型 ↗</a></nav>';
  document.querySelector('[data-page-toc]').innerHTML = '<div class="pb-brand"><b>内容生产评审书</b><span>先阅读完整方案，再集中补充。<br>原型通过后才回写产品文档。</span></div><a href="#page-review">00　整体逻辑与评审说明</a>' + reports.map(p => '<a href="#' + p.id + '">' + p.code + '　' + E(p.name) + '</a>').join('') + '<a href="#global-review">08　集中评审与补充</a><div class="pb-nav-foot">大图是页面方案示意，操作体验请打开当前原型。<br><br><a href="review.html#page-review">完整模块与历史评审 ↗</a><a href="design/content-production.md" target="_blank" rel="noopener">内容生产工作稿 ↗</a></div>';

  function pageHeading(p, index) {
    const previous = index ? reports[index - 1] : null;
    const next = reports[index + 1];
    return '<div class="pb-heading"><div><div class="pb-kicker">CONTENT PRODUCTION / ' + p.code + ' OF 07</div><h2>' + p.code + ' · ' + E(p.name) + '</h2><p>' + E(p.purpose) + '</p></div><nav class="pb-pager"><a href="#' + (previous?.id || 'page-review') + '">‹ 上一页</a><a href="#' + (next?.id || 'global-review') + '">下一页 ›</a></nav></div><div class="pb-status"><span class="pb-pill">' + E(p.status) + '</span><span class="pb-pill proposal">方案示意 · 不代表已实施</span><a href="index.html#' + p.route + '" target="_blank" rel="noopener">打开当前可交互原型 ↗</a></div>';
  }
  function frame(p) {
    return '<div class="pb-frame"><div class="pb-frame-head"><div class="pb-scenes" role="group" aria-label="页面场景">' + p.scenes.map((s,i) => '<button type="button" data-pb-scene="' + i + '" aria-pressed="' + (i===0) + '">' + E(s[0]) + '</button>').join('') + '</div><div class="pb-frame-tools"><a href="#report-' + p.id + '">阅读详细报告 ↓</a><button type="button" class="pb-size" data-pb-size>实际尺寸</button></div></div><div class="pb-art-wrap"><div class="pb-art" role="img" aria-label="' + E(p.name + ' · ' + p.scenes[0][0] + '页面方案示意') + '">' + p.scenes[0][1]() + '</div></div><div class="pb-caption" aria-live="polite"><b>' + E(p.scenes[0][0]) + '：</b>' + E(p.scenes[0][2]) + '</div></div>';
  }
  reports.forEach((p, index) => {
    const meta = window.OPSReviewPages.find(v => v.id === p.id);
    meta.name = p.name;
    meta.detailProposal = true;
    meta.productionReport = true;
    meta.units = [{area:'整页产品逻辑',priority:'P1'},...p.interactions.map(v => ({area:v[0],priority:'P1'})),{area:'异常与恢复',priority:'P1'}];
    const section = document.getElementById(p.id);
    section.querySelector('[data-page-decision]')?.remove();
    section.querySelector('.page-review-pager')?.remove();
    section.querySelector('.page-review-top')?.remove();
    const history = section.innerHTML;
    section.dataset.productionPage = '';
    section.innerHTML = pageHeading(p,index) + frame(p) + '<div class="pb-section-links"><a href="#report-' + p.id + '">产品定位与流程</a><a href="#interaction-' + p.id + '">字段与交互</a><a href="#exceptions-' + p.id + '">异常与复核</a><a href="#feedback-' + p.id + '">补充本页意见</a></div>' +
      '<div id="report-' + p.id + '">' + reportBlock('产品定位与当前改动',reportTable(['区分','本页说明'],[['当前原型',p.current],['本轮建议',p.change],['评审重点',p.focus]]) + '<h4>主操作路径</h4><ol>' + p.path.map(s => '<li>' + E(s) + '</li>').join('') + '</ol>') + '</div>' +
      '<div id="interaction-' + p.id + '">' + reportBlock('字段与交互报告','<div class="pb-fields">' + E(p.fields) + '</div><div style="height:18px"></div>' + reportTable(['区域／动作','具体做法'],p.interactions)) + '</div>' +
      '<div id="exceptions-' + p.id + '">' + reportBlock('异常、恢复与体验复核',reportTable(['场景','响应与边界'],p.exceptions) + '<h4>人工体验复核</h4>' + list(p.checks)) + '</div>' +
      '<div class="review-note"><strong>本页集中补充：</strong>' + E(p.focus) + '</div>' +
      '<details class="review-human-evidence"><summary>此前页面方案与人工反馈（历史保留）</summary>' + history + '</details>' +
      '<details class="review-human-evidence"><summary>历史粗评结论与草稿（不代表本轮通过）</summary><div data-page-decision="' + p.id + '"></div></details>' +
      '<div id="feedback-' + p.id + '"></div><div class="pb-bottom-nav"><a href="#' + (reports[index-1]?.id || 'page-review') + '">← ' + E(reports[index-1]?.name || '整体逻辑') + '</a><a href="#' + (reports[index+1]?.id || 'global-review') + '">' + E(reports[index+1]?.name || '集中评审与补充') + ' →</a></div>';
  });

  const scenesTotal = reports.reduce((n,p) => n + p.scenes.length,0);
  document.getElementById('page-review').innerHTML = '<div class="pb-heading"><div><div class="pb-kicker">PRODUCT & INTERACTION REVIEW / 2026.11.04</div><h1>内容生产，从选材到可发布成片</h1><p>按 social-accounts 的评审书形式，逐页看页面与场景，再阅读每页的完整产品／交互说明。你集中评审和补充，我据此持续修订工作稿与原型。</p></div><nav class="pb-pager"><a href="#page-materials">开始逐页评审 →</a></nav></div><div class="pb-status"><span class="pb-pill confirmed">D1／D2／D3 已确认</span><span class="pb-pill">当前原型为可信基线</span><span class="pb-pill proposal">原型已修正 · 待人工评审</span></div>' +
    '<div class="pb-summary-grid"><div class="pb-summary-card"><strong>7 个评审单元</strong><span>六个业务页面；合成策略的列表详情、编辑拆开评审。素材两页沿用已优化基线。</span></div><div class="pb-summary-card"><strong>' + scenesTotal + ' 个页面场景</strong><span>列表、详情、表单、选择器、执行结果与异常。可切场景、适应宽度或按实际尺寸阅读。</span></div><div class="pb-summary-card"><strong>边评审边沉淀</strong><span>工作稿持续更新；方案示意与业务原型分开标注。原型人工通过后才回写产品文档。</span></div></div>' +
    reportBlock('两条生产路径，共用一套制作配置','<div class="pb-route"><div><b>人工选材</b>新建任务 → 选类目与策略 → 选素材 → 调整本次参数</div><div><b>规则选材</b>已保存策略 → 先排除再选入 → 核验历史与额度 → 创建任务</div><div><b>独立执行</b>素材 × 设置 × 份数 → 独立任务 → 成功后一个成片</div><div><b>成片使用</b>预览／下载／再次制作 → 带选中文件去发布</div></div>' + reportTable(['对象','产品职责'],[['素材库','团队共享内容与选材参考；加入仅建立个人关系。'],['我的素材','个人工作集与视频／本机文件情况；不要求先下载才可生产。'],['合成策略','模板参数默认值＋选材规则＋执行计划；编辑只影响后续任务。'],['合成任务','一次生产的独立执行与恢复；保留创建时快照和尝试历史。'],['成片','生成后的文件资产；下载、平台用途、真实发布结果分别表达。']])) +
    reportBlock('已确认规则，不重复出决策题',reportTable(['编号','用户确认','本轮必须落实'],[['D1','先统一排除，再按优先级选入','调整规则解释器；列表和预览不能只改说明文案。'],['D2','人工明确确认后可再次制作；自动与立即执行跳过历史','新生产创建新任务；技术重试沿用原任务，提交重放不重复创建。'],['D3','每素材 × 每设置 × 每份一条任务','3 ×（2＋1）＝9 条，成功最多 9 个成片；两个平台不翻倍。']])) +
    reportBlock('当前基线与本轮建议的边界','<p>当前可交互原型和用户明确反馈是本轮设计基线，PRD 只作输入。下列事项不会因为旧文档或新建议被静默改掉。</p>' + reportTable(['当前基线','本轮处理'],[['立即执行保留按钮','已有执行查看进度；有候选先预览并确认；无素材展示原因。'],['任务参数按模板开放范围调整','每行配置打开小弹窗，固定参数禁用；确定只更新本次任务。'],['失败重试当前可操作','普通运营是否可重试、取消后恢复、未知结果核对一并评审，不先收紧权限。'],['我的素材／素材库已基本优化','作为衔接基线复核；本轮重点是剩余策略、任务和成片。'],['发布入口统一','勾选成片去发布与发布任务新建共用弹窗，前者带入文件、后者自行选择。']])) +
    reportBlock('逐页目录',reportTable(['页面','本页重点','进入'],reports.map(p => [p.code + ' · ' + p.name,p.purpose,'查看左侧同名页面'])) + '<div class="pb-section-links">' + reports.map(p => '<a href="#' + p.id + '">' + E(p.code + ' ' + p.name) + ' →</a>').join('') + '</div>') +
    reportBlock('如何集中反馈','<p>每页先切换页面场景，再阅读下方的字段、动作、异常和复核标准。可在每页填写意见，也可最后到“集中评审与补充”，按页码一次补充。</p><p>建议反馈格式：<b>页码／场景 → 实际业务补充 → 希望的行为</b>。不必逐项回答问卷，已经确认的 D1／D2／D3 无需重选。</p><div id="review-toolbar" class="review-toolbar"></div><p class="pb-sticky-note">沿用原有人工评论、结论与草稿存储。历史粗评通过不代表本轮方案通过；本轮意见可在同一浏览器导出／导入。评审稿不会自动回写原型、工作稿或正式产品文档；收到你的补充后由我逐项更新。</p><div id="page-progress" class="pb-sticky-note" role="status"></div><div hidden>' + reports.map(p => '<span data-page-status="' + p.id + '"></span>').join('') + '</div>') +
    reportBlock('可执行落地顺序与验收边界',reportTable(['阶段','具体交付'],[['本轮集中评审','先补业务遗漏、参数范围、失败权限与文件／发布衔接；结论持续写入 design/content-production.md。'],['修改可交互原型','先策略与 D1，再创建／任务的 D2、D3，最后成片文件与发布衔接；复用现有工作台组件。'],['人工体验通过','按每页清单走查，方案确认与实际原型通过分别记录。'],['回写与正式实施','原型通过后才回写 PRD，再核对真实接口和所属工程；本稿不声称真实下载、任务调度或服务端权限已验证。']])) +
    '<div class="pb-bottom-nav"><a href="design/content-production.md" target="_blank" rel="noopener">查看持续更新的工作稿 ↗</a><a href="#page-materials">01 素材库 →</a></div>';

  const global = document.getElementById('global-review');
  const globalHistory = global.innerHTML;
  global.dataset.layer = 'pages';
  global.innerHTML = '<div class="pb-heading"><div><div class="pb-kicker">REVIEW & SUPPLEMENT / 08</div><h2>08 · 集中评审与补充</h2><p>完整看完后一次补充即可。下面是本轮值得集中讨论的主题，不是必须逐项回答的决策问卷。</p></div><nav class="pb-pager"><a href="#page-outputs">‹ 成片管理</a><a href="#page-review">返回总览</a></nav></div>' +
    reportBlock('本轮优先收敛的四组问题',reportTable(['关联页','可直接补充的内容'],[['03–04 · 策略','模板参数和 0 值含义、指标缺失判定、规则执行额度、复制默认停用与可选候选预览。'],['05 · 新建任务','名称及必填项、负责人工作集、副作用时点、提交部分失败后的处理位置。'],['06 · 任务恢复','普通运营重试权限、取消后能否重试、结果不明确时的实际核对方式。'],['07 · 成片','真实文件操作、设备下载状态、转交后可见入口、平台使用在列表的展示粒度。']]) + '<p>D1／D2／D3 已确认，当前原型已确认体验继续沿用。其余可以写业务场景、直接纠正文案，或给出整页调整方向。</p>') +
    reportBlock('后续更新约定','<p>收到补充后，先更新当前工作稿和本评审稿的对应页面，标明已确认及待落实内容；再按确认范围修改可交互原型。只有原型通过后，才回写产品文档。发布管理、作品管理和互动管理继续按模块推进。</p><p><a href="design/content-production.md" target="_blank" rel="noopener">内容生产当前工作稿 ↗</a> · <a href="design/publishing.md" target="_blank" rel="noopener">发布管理第一版整体做法 ↗</a></p>') +
    '<details class="review-human-evidence"><summary>此前整体方案与人工整体反馈（历史）</summary>' + globalHistory + '</details>';

  function scaleFrame(frame) {
    const wrap = frame.querySelector('.pb-art-wrap');
    const art = frame.querySelector('.pb-art');
    if (!wrap || !art || !wrap.clientWidth) return;
    const actual = frame.dataset.actual === 'true';
    const ratio = actual ? 1 : Math.min(1,wrap.clientWidth / 1600);
    art.style.transform = 'scale(' + ratio + ')';
    wrap.style.height = (940 * ratio) + 'px';
    wrap.style.overflowX = actual ? 'auto' : 'hidden';
    // 绝对定位元素的缩放边界决定横向滚动范围。
    art.style.position = actual ? 'relative' : 'absolute';
  }
  const resize = new ResizeObserver(entries => entries.forEach(entry => scaleFrame(entry.target.closest('.pb-frame'))));
  document.querySelectorAll('[data-production-page] .pb-art-wrap').forEach(wrap => resize.observe(wrap));
  document.addEventListener('click', event => {
    const sceneButton = event.target.closest('[data-pb-scene]');
    if (sceneButton) {
      const section = sceneButton.closest('[data-production-page]');
      const p = reports.find(v => v.id === section.id);
      const scene = p.scenes[Number(sceneButton.dataset.pbScene)];
      const frame = sceneButton.closest('.pb-frame');
      frame.querySelector('.pb-art').innerHTML = scene[1]();
      frame.querySelector('.pb-art').setAttribute('aria-label',p.name + ' · ' + scene[0] + '页面方案示意');
      frame.querySelector('.pb-caption').innerHTML = '<b>' + E(scene[0]) + '：</b>' + E(scene[2]);
      frame.querySelectorAll('[data-pb-scene]').forEach(b => b.setAttribute('aria-pressed',String(b === sceneButton)));
      scaleFrame(frame);
    }
    const sizeButton = event.target.closest('[data-pb-size]');
    if (sizeButton) {
      const frame = sizeButton.closest('.pb-frame');
      frame.dataset.actual = frame.dataset.actual === 'true' ? 'false' : 'true';
      sizeButton.textContent = frame.dataset.actual === 'true' ? '适应宽度' : '实际尺寸';
      scaleFrame(frame);
    }
  });
  // 历史表单和评论由 review.js 按原有 key 绑定，不在此读写 localStorage。
  addEventListener('hashchange',() => requestAnimationFrame(() => document.querySelectorAll('.pb-frame').forEach(scaleFrame)));
  requestAnimationFrame(() => document.querySelectorAll('.pb-frame').forEach(scaleFrame));
})();
