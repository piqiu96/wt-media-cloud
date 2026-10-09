<script setup>
import { computed, onMounted, ref } from 'vue'
import BrandLogo from '../../../shared/ui/BrandLogo.vue'
import { loadDownloadManifest } from '../downloadManifest.js'

const manifest = ref(null)
const error = ref('')
const platforms = [
  { key: 'windows-x64', icon: 'logo-windows', label: 'Windows', detail: 'Windows 10 / 11 · x64' },
  { key: 'macos-arm64', icon: 'logo-apple', label: 'macOS · Apple 芯片', detail: 'M1 及更新芯片' },
  { key: 'macos-x64', icon: 'logo-apple', label: 'macOS · Intel', detail: 'Intel 处理器' },
]
const downloads = computed(() => platforms.map((platform) => ({
  ...platform,
  url: manifest.value?.downloads?.[platform.key]?.url,
})))

onMounted(async () => {
  try {
    manifest.value = await loadDownloadManifest()
  } catch {
    error.value = '下载信息暂不可用，请稍后再试。'
  }
})
</script>

<template>
  <main class="public-home">
    <div class="sky-glow sky-glow-left" aria-hidden="true"></div>
    <div class="sky-glow sky-glow-right" aria-hidden="true"></div>

    <header class="site-header">
      <BrandLogo class="site-brand" />
      <nav aria-label="官网导航">
        <a href="#features">产品功能</a>
        <a href="#downloads">下载桌面版</a>
        <router-link to="/login">登录平台</router-link>
      </nav>
    </header>

    <section class="hero" aria-labelledby="home-heading">
      <div class="hero-copy">
        <span class="eyebrow">起飞 · 内容运营平台</span>
        <h1 id="home-heading">让流量<span>跑起来</span><i aria-hidden="true"></i></h1>
        <p>找内容、管素材、发内容、看数据，<br class="desktop-break" />让内容运营超轻松！</p>
        <a class="hero-cta" href="#downloads"><t-icon name="download" />立即下载 <span aria-hidden="true">→</span></a>
        <div class="hero-notes" aria-label="产品支持">
          <span><t-icon name="desktop" />Windows / macOS 支持</span>
          <span><t-icon name="cloud" />本地与云端协同</span>
          <span><t-icon name="flash" />轻量易用</span>
        </div>
      </div>
      <div class="hero-art" aria-hidden="true">
        <div class="orbit orbit-outer"></div>
        <div class="orbit orbit-inner"></div>
        <div class="art-card art-card-media"><t-icon name="image" /></div>
        <div class="art-card art-card-chart"><t-icon name="chart-bar" /></div>
        <div class="art-card art-card-play"><t-icon name="play-circle" /></div>
        <div class="hero-emblem"></div>
        <div class="art-spark spark-one">✦</div><div class="art-spark spark-two">✦</div>
      </div>
    </section>

    <div class="wave wave-back" aria-hidden="true"></div>
    <div class="wave wave-front" aria-hidden="true"></div>

    <section id="features" class="features" aria-label="产品功能">
      <article v-for="feature in [
        { icon: 'search', title: '内容发现', note: '发现优质内容' },
        { icon: 'folder', title: '素材管理', note: '统一管理素材' },
        { icon: 'send', title: '发布管理', note: '多平台协同发布' },
        { icon: 'chart-bar', title: '数据分析', note: '数据驱动运营' },
      ]" :key="feature.title" class="feature-card">
        <span class="feature-icon"><t-icon :name="feature.icon" /></span>
        <span><strong>{{ feature.title }}</strong><small>{{ feature.note }}</small></span>
      </article>
    </section>

    <section id="downloads" class="downloads" aria-labelledby="downloads-heading">
      <span class="section-kicker">DESKTOP APP</span>
      <h2 id="downloads-heading">选择适合你的桌面版</h2>
      <p>下载起飞，开始更轻松的内容运营。</p>
      <p v-if="manifest" class="release-version">当前正式版 {{ manifest.version }}</p>
      <p v-if="error" class="download-error" role="status">{{ error }}</p>
      <div class="download-grid">
        <a v-for="platform in downloads" :key="platform.key" class="download-card" :class="{ unavailable: !platform.url }"
          :href="platform.url || undefined" :aria-disabled="!platform.url" :tabindex="platform.url ? 0 : -1"
          target="_blank" rel="noopener noreferrer">
          <span class="download-icon"><t-icon :name="platform.icon" /></span>
          <strong>{{ platform.label }}</strong><small>{{ platform.detail }}</small>
          <span class="download-action">{{ platform.url ? '下载安装包' : '暂不可用' }} <span v-if="platform.url" aria-hidden="true">↗</span></span>
        </a>
      </div>
    </section>

    <footer class="site-footer"><BrandLogo compact /><span>© {{ new Date().getFullYear() }} 起飞</span><router-link to="/login">登录平台</router-link></footer>
  </main>
</template>

<style scoped>
.public-home{position:relative;overflow:hidden;min-height:100vh;background:linear-gradient(137deg,#f5fbff 0%,#eaf6ff 43%,#d5efff 72%,#f8fcff 100%);color:#071b50;font-family:"PingFang SC","Microsoft YaHei",sans-serif}
.sky-glow{position:absolute;pointer-events:none;border-radius:50%;filter:blur(22px)}.sky-glow-left{width:65vw;height:45vw;left:-24vw;top:-18vw;background:#fff}.sky-glow-right{width:75vw;height:65vw;right:-36vw;top:-26vw;background:#8fcbff9c}
.site-header,.hero,.features,.downloads,.site-footer{position:relative;z-index:1;width:min(1480px,calc(100% - 96px));margin-inline:auto}
.site-header{display:flex;align-items:center;justify-content:space-between;min-height:112px}.site-header nav{display:flex;gap:clamp(24px,4vw,72px);align-items:center}.site-header a,.site-footer a{color:#071b50;text-decoration:none;font-size:17px;font-weight:700}.site-header a:hover,.site-footer a:hover{color:#036fff}
.hero{display:grid;grid-template-columns:minmax(0,1fr) minmax(400px,.95fr);min-height:560px;align-items:center;gap:18px}.hero-copy{padding:32px 0 70px}.eyebrow,.section-kicker{display:inline-block;color:#176ff3;font-size:14px;font-weight:800;letter-spacing:.16em}.hero h1{position:relative;white-space:nowrap;margin:20px 0 17px;font-size:clamp(56px,5.3vw,100px);font-weight:950;letter-spacing:-.06em;line-height:1.13;text-shadow:0 5px 13px #70b5ed2b}.hero h1 span{background:linear-gradient(105deg,#0978ef,#00bfee 68%,#125af5);-webkit-background-clip:text;background-clip:text;color:transparent}.hero h1 i{position:absolute;bottom:-4px;right:4%;width:48%;height:18px;border-bottom:6px solid #18cde6;border-radius:50%;transform:rotate(-5deg)}.hero-copy>p{font-size:clamp(17px,1.6vw,24px);line-height:1.65;color:#405c8d;margin:22px 0 28px}
.hero-cta{display:inline-flex;align-items:center;justify-content:center;gap:14px;width:300px;height:70px;border-radius:18px;background:linear-gradient(105deg,#0759f8,#00c4eb);box-shadow:0 14px 28px #1588df4a;color:#fff;font-size:26px;font-weight:800;text-decoration:none;transition:transform .2s,box-shadow .2s}.hero-cta:hover{transform:translateY(-3px);box-shadow:0 19px 32px #1588df62}.hero-cta span{margin-left:10px}.hero-notes{display:flex;flex-wrap:wrap;gap:0;margin-top:28px;color:#2c4b7f;font-size:14px}.hero-notes span{display:inline-flex;align-items:center;gap:8px;padding:0 18px;border-right:1px solid #adcdf1}.hero-notes span:first-child{padding-left:0}.hero-notes span:last-child{border:0}.hero-notes .t-icon{color:#086df4;font-size:21px}
.hero-art{position:relative;height:510px;isolation:isolate}.orbit{position:absolute;border:3px solid #fff9;border-radius:50%;box-shadow:inset 0 0 28px #fff,0 0 45px #8fd5ff}.orbit-outer{width:92%;height:50%;left:2%;top:28%;transform:rotate(-27deg)}.orbit-inner{width:77%;height:65%;left:12%;top:16%;transform:rotate(24deg);border-color:#b7f3ff}.hero-emblem{position:absolute;width:min(70%,440px);aspect-ratio:1;top:6%;left:17%;border-radius:50%;border:7px solid #fffc;background:#fff url('/qifei-reference-logo.png') center 54% / 150% no-repeat;box-shadow:0 0 0 14px #eaffff80,0 25px 70px #50a5e469, inset 0 0 25px #b4e8ff}.art-card{position:absolute;display:grid;place-items:center;width:115px;height:84px;border:3px solid #fff;border-radius:17px;background:linear-gradient(130deg,#e6f9ffba,#ffffff91);box-shadow:0 13px 36px #2b9ee65a;color:#38a6f9;font-size:52px;transform:rotate(-9deg)}.art-card-media{left:0;top:20%}.art-card-chart{right:0;top:9%;transform:rotate(8deg)}.art-card-play{left:4%;bottom:10%;width:82px;height:75px;font-size:39px}.art-spark{position:absolute;color:#fff;font-size:49px;text-shadow:0 0 18px #17bce9}.spark-one{right:6%;top:46%}.spark-two{left:10%;bottom:4%}
.wave{position:absolute;z-index:0;left:-5%;width:110%;height:160px;pointer-events:none}.wave-back{top:625px;border-radius:48% 52% 0 0 / 90% 90% 0 0;background:#d6efffcc;transform:rotate(-2deg)}.wave-front{top:674px;border-radius:45% 55% 0 0 / 80% 90% 0 0;background:#ffffffb0;transform:rotate(2deg)}
.features{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:17px;margin-top:-12px}.feature-card{display:flex;align-items:center;gap:22px;min-height:126px;box-sizing:border-box;padding:20px;border:1px solid #fff;border-radius:21px;background:#fffffff0;box-shadow:0 12px 35px #7bb9eb1d}.feature-icon{display:grid;place-items:center;flex:none;width:75px;height:75px;border-radius:50%;background:linear-gradient(135deg,#edf8ff,#d6ebff);color:#086af6;font-size:40px}.feature-card span:last-child{display:grid;gap:8px}.feature-card strong{font-size:21px}.feature-card small{color:#8095b6;font-size:16px}
.downloads{padding:120px 0 130px;text-align:center}.downloads h2{margin:9px 0;font-size:clamp(32px,3vw,48px)}.downloads>p{color:#637da5;font-size:17px}.release-version{font-weight:700}.download-error{color:#b33d4c!important}.download-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:20px;margin-top:42px}.download-card{display:flex;flex-direction:column;align-items:center;gap:10px;padding:31px 20px;border:1px solid #e3f1ff;border-radius:21px;background:#fff;box-shadow:0 18px 39px #80b8e830;color:#092966;text-decoration:none;transition:transform .2s,box-shadow .2s}.download-card:hover:not(.unavailable){transform:translateY(-5px);box-shadow:0 23px 44px #80b8e852}.download-card.unavailable{opacity:.65;pointer-events:none}.download-icon{display:grid;place-items:center;width:64px;height:64px;border-radius:18px;background:#e9f5ff;color:#086df5;font-size:35px}.download-card strong{font-size:20px}.download-card small{font-size:14px;color:#8296b4}.download-action{margin-top:10px;color:#086ef3;font-weight:800}.site-footer{display:flex;align-items:center;gap:20px;padding:25px 0;border-top:1px solid #cbe4f7;color:#718eaa}.site-footer a{margin-left:auto;font-size:14px}
@media(max-width:1050px){.site-header,.hero,.features,.downloads,.site-footer{width:min(100% - 50px,1100px)}.hero{grid-template-columns:1fr .85fr}.hero h1{font-size:clamp(49px,5.8vw,70px)}.hero-art{height:400px}.features{grid-template-columns:repeat(2,minmax(0,1fr));margin-top:25px}.wave{top:555px}}
@media(max-width:700px){.site-header,.hero,.features,.downloads,.site-footer{width:calc(100% - 36px)}.site-header{min-height:80px}.site-header nav{gap:16px}.site-header nav a{font-size:13px}.site-header nav a:first-child{display:none}.hero{display:flex;flex-direction:column;gap:0;min-height:auto}.hero-copy{width:100%;padding:24px 0 0}.hero h1{font-size:clamp(42px,11vw,66px);margin:14px 0}.hero-copy>p{font-size:16px;margin:15px 0 22px}.desktop-break{display:none}.hero-cta{width:240px;height:58px;font-size:21px}.hero-notes{font-size:11px;gap:10px}.hero-notes span{padding:0 9px}.hero-art{width:100%;height:330px}.hero-emblem{width:250px;left:calc(50% - 125px);top:15px}.art-card{width:65px;height:55px;font-size:29px}.art-card-chart{right:2%}.art-card-media{left:0}.art-card-play{left:4%;bottom:8%;width:60px;height:50px}.wave{top:630px}.features{grid-template-columns:repeat(2,minmax(0,1fr));gap:10px;margin-top:4px}.feature-card{min-height:95px;padding:10px;gap:8px}.feature-icon{width:43px;height:43px;font-size:25px}.feature-card strong{font-size:15px}.feature-card small{font-size:11px}.downloads{padding:80px 0}.download-grid{grid-template-columns:1fr;gap:12px;margin-top:25px}.download-card{padding:18px}.site-footer{font-size:12px}}
@media(max-width:390px){.site-header nav a:nth-child(2){display:none}.features{grid-template-columns:1fr 1fr}.feature-card{min-height:75px}.feature-card small{display:none}.hero-notes span:last-child{display:none}}
</style>
