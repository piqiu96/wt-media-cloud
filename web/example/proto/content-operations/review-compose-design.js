/* 第 06 页的当前工作台体验说明；不替换历史人工意见，不保存或代填结论。 */
(() => {
  const page = window.OPSReviewPages.find(p => p.id === 'page-compose-create');
  const section = document.getElementById(page.id);
  const top = section.querySelector('.page-review-top');
  const decision = section.querySelector('[data-page-decision]');
  const pager = section.querySelector('.page-review-pager');
  top.remove(); decision.remove(); pager.remove();
  const oldContent = section.innerHTML;
  page.detailProposal = true;
  page.name = '新增合成任务及素材选择器';
  const areas = ['类目与负责人','合成策略及成片设置','素材选择器','选用与转存时点','校验与提交结果'];
  page.units = areas.map(area => ({area,priority:'P1'}));
  section.innerHTML = `
    <span class="review-number">第一轮 · 已接入完整工作台 · 2026-11-03 · 体验核对</span>
    <h2>06 · 新增合成任务及素材选择器</h2>
    <p class="review-lead">先确定本次制作类目，再选择适用策略和素材，保存前看清任务数量及等待项。</p>
    <div class="review-note"><strong>已回到完整工作台：</strong><a href="index.html#tasks" target="_blank" rel="noopener">打开合成任务 ↗</a>，点击“新增合成任务”体验已接入的创建流程及素材选择器。<a href="compose-design.html" target="_blank" rel="noopener">独立设计预览 ↗</a>保留作参考。<br>用户已表示合成任务基本满足要求；本轮核对任务与策略的联动。本页体验确认后进入下一页，人工结论仍由用户保存。</div>

    <h3>已经确认的业务前提</h3>
    <ul><li>人工新增的主入口保留在“合成任务”，不在“我的素材”增加通用新建入口。</li><li>必须先选一个类目，再选择策略和素材；两者都限定为该类目。因此删除“混合类目无共同策略”的问题与恢复方案。</li><li>素材选择器提供“我的素材”和“素材库”。从素材库选用时自动加入我的素材，无需跨页手动加入。</li><li>本轮仅优化本页及关联选择器；其他页面按已确认的顺序逐页推进。</li></ul>

    <h3>已确认补充：本次参数调整与成片命名</h3>
    <p><a href="compose-design.html#settings" target="_blank" rel="noopener">查看成片设置与命名预览 ↗</a></p>
    <ul><li><strong>模板定义字段：</strong>合成模板来自人工在平台验证有效的制作方法；所选模板决定策略及任务展示哪些合成参数。</li><li><strong>策略提供默认值：</strong>高级运营配置默认方案，普通运营可以按素材和效果调整本次引导视频、插入时间等参数；不默认增加逐字段授权流程。</li><li><strong>按组成片设置生效：</strong>每组使用一套参数，作用于本次所选素材。不同组可使用不同参数；实际参数保存在本次任务，不回写策略，不改变其他运营的任务。</li><li><strong>配置与产出分别命名：</strong>配置中的“成片名称”改为“成片设置名称”。最终显示名称为“素材标题 · 成片设置名称 · 成片 ID”，下载文件名采用同一名称及扩展名。</li><li><strong>直接复用成片 ID：</strong>使用现有成片记录自身的完整 ID，不另造编号，不用素材 ID、任务 ID、操作者或本批序号替代。多运营、同名配置、同组多份及再次生成的独立成片，均由各自成片 ID 区分。</li><li><strong>创建时只预览规则：</strong>尚未产生实际成片时，不伪造正式 ID。预览中的 8731、8732 明确标为示例，生成后带入实际成片 ID。</li></ul>
    <p class="review-small">本轮已接入第 06 页，并同步必要的关联展示：策略页标明默认值、任务详情显示实际参数、成片名称及下载名使用实际成片 ID。策略的选材条件、执行计划和默认参数未被任务调整改写。策略页和成片管理页的全面优化仍按逐页节奏推进。</p>

    <h3>推荐的页面组织</h3>
    <p><strong>保留一个创建抽屉，素材通过一个选择弹窗完成。</strong>抽屉依次展示“确定制作范围 → 选择合成策略 → 选择制作素材 → 核对本次产出”。这四块是同页分区，不是四步向导；便于来回对照策略、素材和数量。</p>
    <div class="ex-table-scroll"><table class="ex-table"><thead><tr><th>方案</th><th>收益与代价</th><th>判断</th></tr></thead><tbody>
    <tr><td>同页抽屉＋素材选择器</td><td>保留任务列表上下文，配置与产出可以同时核对；列表在独立选择器内展开。</td><td>推荐。抽屉目标宽 960px，固定底部操作区。</td></tr>
    <tr><td>类目／策略／素材分步向导</td><td>单步信息少，但微调素材或策略时需要频繁上一步、下一步。</td><td>本页暂不采用。</td></tr>
    <tr><td>单独创建页面</td><td>适合更复杂制作，但切离列表，对当前单次创建偏重。</td><td>保留为复杂能力增长后的选择。</td></tr></tbody></table></div>

    <h3>字段、默认值与联动</h3>
    <div class="ex-table-scroll"><table class="ex-table"><thead><tr><th>区域</th><th>建议规则</th><th>发生改变时</th></tr></thead><tbody>
    <tr><td>类目 · 必填</td><td>空白新增不默认选中，明确提示“请选择类目”。未选择前禁用策略和素材入口，并解释原因。</td><td>已有策略或素材时，更换前说明清空范围；取消更换则完整保留原配置。</td></tr>
    <tr><td>任务负责人</td><td>默认当前操作者。沿用现有权限：普通运营只读，高级运营可调整。预览演示高级运营。</td><td>变更后按新负责人的策略及个人素材范围重选；已有依赖配置时确认清空，任务名称保留。</td></tr>
    <tr><td>任务名称 · 必填</td><td>由运营填写，提示例子；同次创建的任务共用此名称，避免把一批任务误认为一个执行项。</td><td>不因类目或负责人变化抹掉已输入名称。空白保存时定位字段、保留其他填写。</td></tr>
    <tr><td>合成策略 · 必选</td><td>只展示当前类目、负责人下启用的策略；不自动替运营选中。选中后展示制作方法摘要及成片设置。</td><td>更换策略保留同类目素材，重置为新策略默认成片设置；若已调整设置，先确认覆盖影响。</td></tr>
    <tr><td>无可用策略</td><td>区分“未选类目”与“此范围没有策略”。显示类目及负责人，提示检查范围或先到合成策略新增。</td><td>已有表单不被自动关闭，也不把无结果当作系统异常。</td></tr>
    <tr><td>本次成片设置</td><td>默认展开摘要，详细配置折叠。配置名称改为“成片设置名称”；普通运营可修改本次模板参数。保留份数、适用平台及添加／移除／排序能力。</td><td>每组参数作用于本次所选素材；修改不回写策略。份数沿用每项 1–20 的整数；数量实时更新。模板决定字段，策略提供默认值。</td></tr>
    <tr><td>数量与提交</td><td>任务数＝素材数 × 各成片设置份数之和。展示可排队、等待素材两类数量，创建成功与合成完成分开表述。</td><td>示例：3 条素材 ×（1＋1）份＝6 条任务，其中 4 条可排队、2 条等待转存。</td></tr>
    </tbody></table></div>
    <p class="review-small">工作台沿用模板定义的字段，并保留适用平台、成片设置添加／移除／排序能力。任务创建时复制策略默认值，保存本次参数；随后修改策略也不会改写已有任务。地址仅为演示，不请求真实视频。</p>

    <h3>素材选择器：选得快，也看得清</h3>
    <ol><li><strong>类目固定：</strong>顶部显示本次类目，选择器内不再放第二个类目切换。需要改变类目时回到创建表单。</li>
    <li><strong>来源切换：</strong>默认“我的素材”，可切换“素材库”；已有个人素材在素材库内明确标记。两处选中的是同一素材，不重复计数。</li>
    <li><strong>筛选与选择分开：</strong>综合搜索支持标题、作者和 ID；保留我的使用状态筛选。查询、重置、翻页、切换来源都不静默清空已选。</li>
    <li><strong>选中范围明确：</strong>表头勾选仅作用于本页可用项；另设“选择全部匹配可用项（N）”，明确覆盖筛选后的全部页。底部持续显示跨页总数，“只看已选”用于核对和取消。</li>
    <li><strong>信息层级：</strong>素材 ID、标题／作者、发布时间、表现数据、我的使用、源视频状态。保留发布时间及浏览量排序。</li>
    <li><strong>可用性可解释：</strong>已转存可选；未转存可选并说明确认后转存；转存中可选并说明任务等待。转存失败、来源失效禁选，提供具体原因与恢复方向。</li>
    <li><strong>调整与取消：</strong>打开时复制当前选择；关闭／取消只丢弃本次选择器内的改动，父表单保持原选项；“使用所选”才带回。支持取消全部选择。</li></ol>

    <h3>已采用的三个交互决策</h3>
    <div class="ex-table-scroll"><table class="ex-table"><thead><tr><th>决策</th><th>当前工作台</th><th>原因与影响</th></tr></thead><tbody>
    <tr><td>D1 · 页面组织</td><td>同页创建抽屉＋素材选择弹窗；四个区域连续展示。</td><td>避免返回多个步骤核对配置，任务列表上下文得以保留。</td></tr>
    <tr><td>D2 · 个人素材与转存何时生效</td><td>点击“使用所选”时加入当前负责人的我的素材，并触发未转存视频转存；单纯勾选不生效。</td><td>尽早保存来源，同时给用户一次明确提交点。此后取消创建任务，也不撤销已经确认的加入和转存；选择器底部须提前告知。</td></tr>
    <tr><td>D3 · 提交按钮</td><td>按钮“保存”，旁边固定说明“保存后创建 N 条任务并加入执行队列”。</td><td>保持常规表单用词，同时明确保存会开始执行流程；若仍容易误解，可改为“创建并加入队列”。</td></tr>
    </tbody></table></div>

    <h3>异常与恢复规则</h3>
    <div class="ex-table-scroll"><table class="ex-table"><thead><tr><th>场景</th><th>必须达到的体验</th><th>当前覆盖</th></tr></thead><tbody>
    <tr><td>未选类目／无策略／无素材</td><td>解释缺失前提，保存不可用；已有输入保留。</td><td>可操作。</td></tr>
    <tr><td>更换范围、策略覆盖、放弃编辑</td><td>仅有实际数据损失时确认，说明具体影响；继续编辑时不丢数据。</td><td>可操作。</td></tr>
    <tr><td>来源失效、转存失败、查询无结果</td><td>原因可见，其他已选项保留；不强迫重新选择整批。</td><td>素材库不可用项与搜索无结果可查看。</td></tr>
    <tr><td>同素材已由同策略创建过任务</td><td>沿用现有重复制作提示，允许运营明确继续；“素材用过”不等于禁止再次制作。</td><td>工作台已保留，浏览器验证可确认继续创建新任务。</td></tr>
    <tr><td>提交失败、提交中重复点击</td><td>失败保留配置并允许重试；提交中按钮加载、禁止重复提交。不把请求中误报成功。</td><td>独立设计预览保留失败重试示例；工作台为本地同步保存，真实请求过程待运行软件验证。</td></tr>
    <tr><td>提交前策略停用或素材状态改变</td><td>再次校验并定位具体失效项；保留其余配置，允许替换或取消选择。</td><td>保存前复核策略和素材可用性；远端实时变化待运行软件验证。</td></tr>
    </tbody></table></div>

    <h3>本页体验确认清单</h3>
    <ol><li>进入空白新增：明确先选类目，不自动选择策略，不出现跨类目混选或无共同策略提示。</li><li>选“剪辑教程”：策略与素材始终限定在该类目；改成“拍摄技巧”先确认清空，取消则保留。</li><li>从我的素材切到素材库、翻页、筛选：已选总数保持；同一素材不重复计数。</li><li>取消选择器：父表单选项不变；使用所选：正确带回、新加入与转存数量可解释。</li><li>未转存素材选用后显示转存中；任务待素材，不宣称已合成。</li><li>查看不可用原因、无结果提示、无可用策略状态，能知道下一步怎么处理。</li><li>修改份数，核对总任务数及等待数；空任务名称、非法份数不能提交。</li><li>放弃创建后，已确认加入的个人素材仍保留；仅临时勾选的素材不产生选用记录。</li><li>创建后可定位本次批量记录与任务详情，任务参数应与创建表单一致；真实请求失败恢复待运行软件验证。</li><li>修改本次插入时间后，对照策略详情仍为原默认值；再次创建时带入原默认值。按实际成片 ID 区分同名产出。</li></ol>
    <div class="review-note"><strong>本轮核对：</strong>任务参数与策略默认值相互隔离，并不代表微调后的平台效果相同。插入时机、引导内容等调整会改变本次成片，应结合数据判断效果。请在完整工作台体验；本页体验通过前不推进其他页面。历史人工意见继续保留，当前意见可使用页尾补充区。</div>
    <details class="review-human-evidence"><summary>第八版粗评及更早人工意见（历史依据，包含已被本轮纠正的顺序）</summary>${oldContent}</details>`;
  section.prepend(top);
  section.append(decision, pager);
  const currentLink = top.querySelector('a[target]');
  currentLink.href = 'index.html#tasks'; currentLink.textContent = '打开完整工作台 · 合成任务 ↗';
  const home = document.getElementById('page-review');
  home.querySelector('h1').textContent = '逐页优化 · 第一轮：新增合成任务及素材选择器';
  home.querySelector('.review-number').textContent = '已确认推进范围与节奏 · 第 06 页已接入完整工作台';
  home.querySelector('.page-stage-flow').innerHTML = '<div><b>01 · 本页方案已接入</b><span>类目优先、素材选择器、本次参数与成片命名</span></div><span>→</span><div class="current"><b>02 · 当前：完整工作台体验</b><span>核对任务与策略联动 → 体验确认后进入下一页</span></div>';
  const notices = home.querySelectorAll('.review-note');
  notices[0].innerHTML = '<strong>本轮已回到完整工作台。</strong>新增合成任务及素材选择器已接入，正在核对任务与策略联动；沿用“每页方案确认后实施、体验确认后进入下一页”的节奏。<a href="#page-compose-create">进入第 06 页详细评审 →</a>。其他页面保留第八版粗评记录，不自动标记通过。';
  notices[1].innerHTML = '<strong>合成创建规则已纠正：</strong>入口位于合成任务；先选一个类目，再选同类目策略和素材。素材库选用自动加入我的素材。“先选素材”和“混合类目无共同策略”仅作为历史对照。';
  const row = home.querySelector('[data-page-status="page-compose-create"]').closest('tr');
  row.cells[0].querySelector('strong').textContent = '06 · 新增合成任务及素材选择器';
  row.cells[1].textContent = '第一轮细评：创建表单、选择器、联动、选用时点及异常恢复';
  row.cells[2].textContent = '已接入工作台 · 体验核对';
  document.querySelector('.review-header small').textContent = '2026-11-03 · 第 06 页已接入工作台 · 体验核对';
  document.querySelector('.review-header [data-review-mode-link="pages"]').textContent = '逐页评审';
  document.querySelector('[data-page-toc] a[href="#page-compose-create"]').textContent = '06 · 新增合成任务及素材选择器';
  home.querySelector('h3').textContent = '历史粗评通过条件（保留参考）';
  home.querySelector('ol + .review-small').textContent = '当前按用户确认的逐页节奏推进：本页方案确认 → 接入工作台 → 本页体验确认 → 下一页。已有粗评记录独立保留。';
})();
