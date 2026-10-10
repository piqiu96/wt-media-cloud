<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import BrandLogo from '../../../shared/ui/BrandLogo.vue'
import { loadDownloadManifest } from '../downloadManifest.js'

const manifest = ref(null)
const error = ref('')
const activeNav = ref('product')
const sectionIds = ['hero', 'product', 'download', 'pricing', 'contact']
const navItems = [
  { id: 'product', label: '产品' },
  { id: 'download', label: '下载' },
  { id: 'pricing', label: '定价' },
  { id: 'contact', label: '联系' },
]
const features = [
  { icon: 'search', title: '内容发现', note: '发现优质内容' },
  { icon: 'folder', title: '素材管理', note: '统一管理素材' },
  { icon: 'send', title: '发布管理', note: '多平台协同发布' },
  { icon: 'chart-bar', title: '数据分析', note: '数据驱动运营' },
]
const platforms = [
  { key: 'windows-x64', icon: 'logo-windows', label: 'Windows', detail: 'Windows 10 / 11 · x64' },
  { key: 'macos-arm64', icon: 'logo-apple', label: 'macOS · Apple 芯片', detail: 'M1 及更新芯片' },
  { key: 'macos-x64', icon: 'logo-apple', label: 'macOS · Intel', detail: 'Intel 处理器' },
]
const downloads = computed(() => platforms.map((platform) => ({
  ...platform,
  url: manifest.value?.downloads?.[platform.key]?.url,
})))

let sectionObserver

function scrollToSection(id) {
  const section = document.getElementById(id)
  if (!section) return
  const maxScroll = document.documentElement.scrollHeight - window.innerHeight
  const desiredTop = section.getBoundingClientRect().top + window.scrollY - 110
  if (id === 'pricing' && desiredTop > maxScroll - 60) {
    window.scrollTo({ top: Math.max(0, maxScroll - 60), behavior: 'smooth' })
  } else {
    section.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }
  activeNav.value = id === 'hero' ? 'product' : id
}

function scrollToTop() {
  window.scrollTo({ top: 0, behavior: 'smooth' })
  activeNav.value = 'product'
}

function updateActiveNav() {
  const maxScroll = document.documentElement.scrollHeight - window.innerHeight
  const marker = window.scrollY >= maxScroll - 180
    ? window.innerHeight * 0.67
    : Math.min(window.innerHeight * 0.38, 360)
  let current = 'product'
  for (const id of sectionIds.slice(2)) {
    if (document.getElementById(id)?.getBoundingClientRect().top <= marker) current = id
  }
  if (window.scrollY + window.innerHeight >= document.documentElement.scrollHeight - 2) current = 'contact'
  activeNav.value = current
}

onMounted(async () => {
  await nextTick()
  sectionObserver = new IntersectionObserver(updateActiveNav, {
    rootMargin: '-110px 0px -20% 0px',
    threshold: [0, 0.25, 0.5, 0.75, 1],
  })
  sectionIds.forEach((id) => {
    const section = document.getElementById(id)
    if (section) sectionObserver.observe(section)
  })
  window.addEventListener('resize', updateActiveNav)
  updateActiveNav()
  try {
    manifest.value = await loadDownloadManifest()
  } catch {
    error.value = '下载信息暂不可用，请稍后再试。'
  }
})

onUnmounted(() => {
  sectionObserver?.disconnect()
  window.removeEventListener('resize', updateActiveNav)
})
</script>

<template>
  <main class="public-home">
    <header class="site-header">
      <a class="site-brand" href="#hero" aria-label="起飞，返回页面顶部" @click.prevent="scrollToTop"><BrandLogo /></a>
      <nav aria-label="官网导航">
        <a v-for="item in navItems" :key="item.id" :href="`#${item.id}`"
          :class="{ active: activeNav === item.id }"
          :aria-current="activeNav === item.id ? 'location' : undefined"
          @click.prevent="scrollToSection(item.id)">{{ item.label }}</a>
      </nav>
    </header>

    <section id="hero" class="hero" aria-labelledby="home-heading">
      <div class="hero-copy">
        <h1 id="home-heading">让流量<span>跑起来</span><i aria-hidden="true"></i></h1>
        <p>找内容、管素材、发内容、看数据，让内容运营超轻松！</p>
        <a class="hero-cta" href="#download" @click.prevent="scrollToSection('download')"><t-icon name="download" />立即下载</a>
        <div class="hero-notes" aria-label="产品支持">
          <span><t-icon name="desktop" />Windows / macOS 支持</span>
          <span><t-icon name="cloud" />本地与云端协同</span>
        </div>
      </div>
      <div class="hero-art" aria-hidden="true"><img src="/home-landing-reference.png" alt="" /></div>
    </section>

    <section id="product" class="product-section" aria-labelledby="product-heading">
      <div class="section-heading">
        <span class="section-kicker">PRODUCT FEATURES</span>
        <h2 id="product-heading">产品功能</h2>
        <p>从内容发现到数据分析，提供一站式内容运营解决方案。</p>
      </div>
      <div class="feature-grid">
        <article v-for="feature in features" :key="feature.title" class="feature-card">
          <span class="feature-icon"><t-icon :name="feature.icon" /></span>
          <h3>{{ feature.title }}</h3>
          <p>{{ feature.note }}</p>
        </article>
      </div>
    </section>

    <section id="download" class="download-section" aria-labelledby="download-heading">
      <div class="section-heading">
        <span class="section-kicker">DOWNLOAD</span>
        <h2 id="download-heading">下载桌面版</h2>
        <p>选择适合你的桌面版，开启更轻松的内容运营。</p>
        <p class="release-version" aria-live="polite">{{ manifest ? `当前正式版 ${manifest.version}` : '\u00a0' }}</p>
        <p v-if="error" class="download-error" role="status">{{ error }}</p>
      </div>
      <div class="download-grid">
        <a v-for="platform in downloads" :key="platform.key" class="download-card"
          :class="{ unavailable: !platform.url }" :href="platform.url || undefined"
          :aria-disabled="!platform.url" :tabindex="platform.url ? 0 : -1"
          target="_blank" rel="noopener noreferrer">
          <span class="download-icon"><t-icon :name="platform.icon" /></span>
          <strong>{{ platform.label }}</strong>
          <small>{{ platform.detail }}</small>
          <span class="download-action"><t-icon v-if="platform.url" name="download" />{{ platform.url ? '下载安装包' : '暂不可用' }}</span>
        </a>
      </div>
    </section>

    <section id="pricing" class="information-section" aria-labelledby="pricing-heading">
      <div class="information-card">
        <span class="information-icon"><t-icon name="currency-exchange" /></span>
        <div><h2 id="pricing-heading">定价</h2><p>具体方案与价格由管理员提供，请联系所在团队的平台管理员。</p></div>
        <a class="contact-link" href="#contact" @click.prevent="scrollToSection('contact')">联系管理员</a>
      </div>
    </section>

    <section id="contact" class="information-section" aria-labelledby="contact-heading">
      <div class="information-card">
        <span class="information-icon"><t-icon name="usergroup" /></span>
        <div><h2 id="contact-heading">联系管理员</h2><p>如需开通账号或咨询方案，请联系所在团队的平台管理员。</p></div>
      </div>
    </section>

    <footer class="site-footer"><BrandLogo /><span>© {{ new Date().getFullYear() }} 起飞</span></footer>
  </main>
</template>

<style scoped>
.public-home {
  --home-blue: var(--td-brand-color, #0052d9);
  --home-ink: #071b50;
  --home-muted: #5e78a0;
  position: relative;
  overflow: clip;
  min-height: 100vh;
  background: linear-gradient(180deg, #f7fbff, #edf7ff 58%, #f3faff);
  color: var(--home-ink);
  font-family: "PingFang SC", "Microsoft YaHei", sans-serif;
}
.site-header, .hero, .product-section, .download-section, .information-section, .site-footer {
  position: relative;
  box-sizing: border-box;
  width: min(1280px, calc(100% - 64px));
  margin-inline: auto;
}
.site-header {
  position: sticky;
  top: 16px;
  z-index: 100;
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: min(1440px, calc(100% - 48px));
  height: 72px;
  margin-top: 16px;
  padding: 0 28px;
  border: 1px solid #e3eef8;
  border-radius: 24px;
  background: #fffffff0;
  box-shadow: 0 6px 20px #407eb916;
}
.site-brand { display: inline-flex; align-items: center; color: inherit; text-decoration: none; }
.site-brand :deep(.brand-logo__mark) { width: 40px; height: 40px; background-size: 150%; }
.site-brand :deep(.brand-logo__copy strong) { font-size: 25px; }
.site-brand :deep(.brand-logo__copy small) { display: none; }
.site-header nav { display: flex; align-items: center; gap: 22px; }
.site-header nav a {
  padding: 9px 16px;
  border-radius: 12px;
  color: var(--home-ink);
  font-size: 16px;
  font-weight: 650;
  text-decoration: none;
  transition: color .18s, background-color .18s;
}
.site-header nav a:hover, .site-header nav a.active { background: #e9f4ff; color: var(--home-blue); }
.site-header a:focus-visible, .hero-cta:focus-visible, .download-card:focus-visible, .contact-link:focus-visible { outline: 2px solid var(--home-blue); outline-offset: 3px; }
#hero, #product, #download, #pricing, #contact { scroll-margin-top: 110px; }
.hero {
  display: grid;
  grid-template-columns: minmax(0, 1.08fr) minmax(0, .92fr);
  align-items: center;
  gap: 20px;
  width: min(1360px, calc(100% - 64px));
  min-height: 540px;
  padding: 32px 0 60px;
}
.hero-copy { z-index: 1; }
.hero h1 { position: relative; width: max-content; max-width: 100%; margin: 0 0 18px; font-size: clamp(56px, 5.4vw, 76px); font-weight: 950; letter-spacing: -.07em; line-height: 1.14; white-space: nowrap; }
.hero h1 span { background: linear-gradient(105deg, #0978ef, #00bfee 68%, #125af5); background-clip: text; -webkit-background-clip: text; color: transparent; }
.hero h1 i { position: absolute; right: 3%; bottom: -8px; width: 46%; height: 16px; border-bottom: 5px solid #18cde6; border-radius: 50%; transform: rotate(-5deg); }
.hero-copy > p { margin: 24px 0 30px; color: #405c8d; font-size: clamp(16px, 1.5vw, 20px); line-height: 1.6; white-space: nowrap; }
.hero-cta { display: inline-flex; align-items: center; justify-content: center; gap: 12px; width: 264px; height: 56px; border-radius: 16px; background: linear-gradient(105deg, #0759f8, #00bfe5); box-shadow: 0 8px 18px #1588df2b; color: #fff; font-size: 20px; font-weight: 700; text-decoration: none; }
.hero-cta:hover { filter: brightness(1.03); }
.hero-notes { display: flex; align-items: center; flex-wrap: wrap; gap: 0; margin-top: 26px; color: #2c4b7f; font-size: 14px; }
.hero-notes span { display: inline-flex; align-items: center; gap: 7px; padding: 0 18px; border-right: 1px solid #adcdf1; }
.hero-notes span:first-child { padding-left: 0; }
.hero-notes span:last-child { border: 0; }
.hero-notes .t-icon { color: var(--home-blue); font-size: 19px; }
.hero-art { position: relative; height: 460px; overflow: hidden; pointer-events: none; mask-image: radial-gradient(ellipse 65% 68% at 60% 50%, #000 35%, transparent 95%); }
.hero-art img { position: absolute; top: -94px; right: 0; width: 233%; max-width: none; height: auto; }
.section-heading { text-align: center; }
.section-kicker { color: var(--home-blue); font-size: 11px; font-weight: 700; letter-spacing: .22em; }
.section-heading h2 { margin: 8px 0 10px; font-size: clamp(34px, 3vw, 42px); line-height: 1.2; }
.section-heading > p { margin: 0; color: var(--home-muted); font-size: 16px; line-height: 1.6; }
.product-section { padding: 58px 0 88px; }
.feature-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 22px; margin-top: 34px; }
.feature-card { box-sizing: border-box; min-height: 208px; padding: 27px 28px; border: 1px solid #e6f0fa; border-radius: 22px; background: #fff; box-shadow: 0 5px 18px #5d9bd011; transition: transform .18s, border-color .18s; }
.feature-card:hover { transform: translateY(-2px); border-color: #bdddf5; }
.feature-icon { display: grid; place-items: center; width: 60px; height: 60px; border-radius: 50%; background: #e7f4ff; color: var(--home-blue); font-size: 30px; }
.feature-card h3 { margin: 20px 0 6px; font-size: 20px; line-height: 1.25; }
.feature-card p { margin: 0; color: var(--home-muted); font-size: 16px; }
.download-section { padding: 42px 0 86px; }
.section-heading > .release-version { min-height: 22px; margin-top: 3px; font-size: 14px; font-weight: 600; }
.section-heading > .download-error { color: var(--td-error-color, #d54941); }
.download-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 20px; margin-top: 26px; }
.download-card { display: flex; flex-direction: column; align-items: center; box-sizing: border-box; min-height: 238px; padding: 22px 24px 20px; border: 1px solid #e6f0fa; border-radius: 22px; background: #fff; box-shadow: 0 5px 18px #5d9bd011; color: var(--home-ink); text-align: center; text-decoration: none; transition: transform .18s, border-color .18s; }
.download-card:hover:not(.unavailable) { transform: translateY(-2px); border-color: #bdddf5; }
.download-card.unavailable { opacity: .65; pointer-events: none; }
.download-icon { display: grid; place-items: center; width: 68px; height: 68px; border-radius: 18px; background: #e9f5ff; color: var(--home-blue); font-size: 36px; }
.download-card strong { margin-top: 14px; font-size: 18px; line-height: 1.3; }
.download-card small { margin-top: 6px; color: var(--home-muted); font-size: 14px; }
.download-action { display: inline-flex; align-items: center; justify-content: center; gap: 7px; width: min(100%, 220px); min-height: 38px; margin-top: auto; border-radius: 10px; background: #e7f4ff; color: var(--home-blue); font-size: 15px; font-weight: 700; }
.information-section { padding-bottom: 18px; }
.information-card { display: flex; align-items: center; gap: 26px; box-sizing: border-box; width: min(900px, 100%); min-height: 126px; margin-inline: auto; padding: 22px 30px; border: 1px solid #e6f0fa; border-radius: 22px; background: #fff; box-shadow: 0 5px 18px #5d9bd011; }
.information-icon { display: grid; place-items: center; flex: none; width: 64px; height: 64px; border-radius: 50%; background: #e7f4ff; color: var(--home-blue); font-size: 30px; }
.information-card div { flex: 1; min-width: 0; }
.information-card h2 { margin: 0 0 8px; font-size: 28px; line-height: 1.2; }
.information-card p { margin: 0; color: var(--home-muted); font-size: 16px; line-height: 1.5; }
.contact-link { flex: none; padding: 10px 14px; border: 1px solid #b8d9f8; border-radius: 10px; color: var(--home-blue); font-size: 14px; font-weight: 600; text-decoration: none; }
.contact-link:hover { background: #edf7ff; }
.site-footer { display: flex; align-items: center; justify-content: space-between; min-height: 105px; margin-top: 100px; border-top: 1px solid #d9e9f8; color: var(--home-muted); font-size: 14px; }
.site-footer :deep(.brand-logo__mark) { width: 36px; height: 36px; }
.site-footer :deep(.brand-logo__copy strong) { font-size: 22px; }
.site-footer :deep(.brand-logo__copy small) { display: none; }
@media (max-width: 1050px) {
  .hero { grid-template-columns: minmax(0, 1fr) minmax(0, .85fr); min-height: 500px; }
  .hero-copy > p { white-space: normal; }
  .hero-art { height: 390px; }
  .hero-art img { top: -75px; }
  .feature-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .feature-card { min-height: 190px; }
}
@media (max-width: 700px) {
  .site-header, .hero, .product-section, .download-section, .information-section, .site-footer { width: calc(100% - 32px); }
  .site-header { height: 66px; padding: 0 12px; border-radius: 18px; }
  .site-brand :deep(.brand-logo__mark) { width: 34px; height: 34px; }
  .site-brand :deep(.brand-logo__copy strong) { font-size: 21px; }
  .site-header nav { gap: 0; }
  .site-header nav a { padding: 7px 8px; font-size: 13px; }
  .hero { display: flex; flex-direction: column; align-items: stretch; gap: 0; min-height: auto; padding: 75px 0 16px; }
  .hero h1 { font-size: clamp(40px, 10vw, 62px); }
  .hero-copy > p { margin: 20px 0 25px; font-size: 16px; }
  .hero-cta { width: 240px; }
  .hero-notes { gap: 8px; font-size: 11px; }
  .hero-notes span { padding: 0 9px; }
  .hero-art { width: 100%; height: 300px; margin-top: 6px; }
  .hero-art img { top: -52px; }
  .product-section { padding: 42px 0 70px; }
  .feature-grid { gap: 12px; margin-top: 26px; }
  .feature-card { min-height: 170px; padding: 18px; }
  .feature-icon { width: 48px; height: 48px; font-size: 25px; }
  .feature-card h3 { margin-top: 16px; font-size: 17px; }
  .feature-card p { font-size: 14px; }
  .download-section { padding: 12px 0 65px; }
  .download-grid { grid-template-columns: 1fr; gap: 12px; }
  .download-card { min-height: 195px; }
  .information-card { gap: 15px; min-height: 120px; padding: 18px; }
  .information-icon { width: 48px; height: 48px; font-size: 24px; }
  .information-card h2 { font-size: 23px; }
  .information-card p { font-size: 14px; }
  .contact-link { align-self: flex-start; padding: 7px 10px; font-size: 12px; }
  .site-footer { min-height: 90px; }
}
@media (max-width: 360px) {
  .site-header { padding: 0 8px; }
  .site-header nav a { padding-inline: 6px; font-size: 12px; }
  .hero h1 { font-size: 38px; }
  .hero-notes { font-size: 10px; }
  .feature-card { padding: 14px; }
}
</style>
