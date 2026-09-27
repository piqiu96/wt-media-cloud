import { ref } from 'vue'

// 下载中心只有一份面板，而想打开它的入口不止一个：顶栏按钮，以及素材库／我的素材里
// 刚发起下载的那一下（点了下载却要运营自己去顶栏找入口，是最容易漏掉的一步）。
//
// 用模块级单例而不是 provide/inject：AppLayout 与页面之间隔着路由视图，页面拿不到
// 顶栏那一层的作用域，provide 到不了；emit 更不行，两者不是父子。
const visible = ref(false)

export function useDownloadCentre() {
  return {
    visible,
    open() { visible.value = true },
    close() { visible.value = false },
  }
}
