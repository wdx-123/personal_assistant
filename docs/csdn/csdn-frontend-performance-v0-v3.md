# 把首屏做到 2x+：一个 Vue3 + Vite 项目的前端性能优化实战（v0 -> v3）

最早我以为首页慢主要是接口问题，真往前端链路里拆，才发现拖首屏的不是一个点，而是三条链路叠在一起：`资源体积`、`背景视觉资源`、`首页初始化请求时序`。所以这次没有走“修一个点试试看”的路子，而是连续做了三轮：`v1` 先提速，`v2` 补视觉过渡，`v3` 再把“快”和“顺”之间的平衡做细，目标就是让 `/login` 和 `/home` 冷启动明显更快。

## v0：问题不是单点差，而是把能延后的东西都塞进了首屏

核心问题可以概括成一句话：**本来可以延后的资源和请求，被默认都放到了首屏阶段。**

| v0 问题 | 用户感知 | 根因 |
| --- | --- | --- |
| `gzip` 没开 | 弱网下 JS/CSS 传输吃亏 | 部署层没把压缩打开 |
| `manualChunks` 过宽 | 首页容易卷进不该首屏加载的依赖 | `id.includes('vue')` 这类宽匹配误伤第三方库 |
| 背景轮播全局挂载 | 非首页页面也背上背景资源成本 | 背景组件挂在根组件，没有按路由裁剪 |
| 首页初始化请求过多 | 页面刚出来就一直在 loading | 排行榜、组织、曲线、菜单刷新一起抢首屏 |
| 菜单刷新走阻塞链路 | 刷新和首次进入更容易卡住 | 动态路由依赖实时菜单结果 |

## v1：第一轮先解决“快不快”

第一轮我只做影响首屏速度的事：`Vite 分包修正`、`Nginx gzip + 强缓存`、`首页请求时序改造`、`请求去重/取消`。

### 1. 分包先收敛，不让首页替后台页面买单

我先改的是 `vite.config.ts`，重点不是把包拆得越碎越好，而是把真正的首屏运行时和后台模块隔开。

```ts
const chunkByPackage = (id: string) => {
  if (id.includes("/node_modules/vue/") || id.includes("/node_modules/@vue/")) {
    return "vue-core";
  }
  if (
    id.includes("/node_modules/vue-router/") ||
    id.includes("/node_modules/pinia/") ||
    id.includes("/node_modules/@vueuse/")
  ) {
    return "router-pinia";
  }
  if (id.includes("/node_modules/axios/")) {
    return "http";
  }
  return "vendor";
};
```

这样之后，登录页和首页不再默认替 `console`、权限管理、工作台这些模块付首屏成本，当前 `dist/index.html` 也只预载 `vue-core`、`router-pinia`、`vendor` 和 `http`。

### 2. 压缩和缓存交给 Nginx

```nginx
gzip on;
gzip_vary on;
gzip_min_length 1024;

location = /index.html {
    add_header Cache-Control "no-cache, no-store, must-revalidate" always;
}

location ~* ^/(?:js|css|avif|woff|woff2|ttf)/ {
    expires 1y;
    add_header Cache-Control "public, max-age=31536000, immutable" always;
}
```

### 3. 首页不再“一上来全拉”

第二个问题是请求时序。首页最早不是请求失败，而是请求太积极，什么都想第一时间完成。后来我把链路拆成了两层：

- 菜单刷新改成“缓存优先 + 后台刷新”。
- 组织列表从首页 `onMounted` 移到“切换组织”弹窗打开时再拉。
- 排行榜从 `refreshAll(scope, orgId)` 改成 `refreshPlatform(platform, scope, orgId)`，只刷当前平台。

### 4. 请求层支持去重和取消

用户快速切平台、切组织时，如果没有取消机制，旧请求回来以后很容易把新状态覆盖掉。现在请求层补了 `dedupeKey` 和 `cancelPrevious`，排行榜也从 `refreshAll` 改成按平台 `refreshPlatform`：

```ts
export interface RequestConfig extends InternalAxiosRequestConfig {
  dedupeKey?: string
  cancelPrevious?: boolean
}

if (config.cancelPrevious && existingController) existingController.abort()

const refreshPlatform = async (platform, scope = 'current_org', orgId?) => {
  if (platform === 'luogu') return fetchLuoguRankingList(1, false, scope, orgId)
  if (platform === 'leetcode') return fetchLeetcodeRankingList(1, false, scope, orgId)
  return fetchLanqiaoRankingList(1, false, scope, orgId)
}
```

## v2：性能不能把体验做丑

第一轮之后，页面体感已经好很多了，但背景切换还是偏硬。所以 `v2` 开始单独处理背景链路：首屏先给一张极轻量的 `poster`，后台再预加载 4 张正式轮播图，等它们到齐以后整体淡入轮播层。资源策略也固定下来了：

- `poster`：`320px + blur + AVIF q28`
- 正式轮播图：`1920px + AVIF q52`

这一版没有直接做“第一张图从模糊慢慢变清晰”，因为 `v2` 当时先求稳，不想让轮播在资源没齐时出现残缺状态。

## v3：能直接清晰就别先糊

`v2` 做完以后，我又发现一个体验问题：它太保守了。缓存命中或者网络较好时，第一张高清图其实能很快就位，这时候还强行先给 `poster`，反而多了一道流程。所以 `v3` 的核心就一句话：**能直接清晰就别先糊，不能直接清晰再优雅过渡。**

```ts
type CarouselState = 'booting' | 'poster' | 'revealing' | 'ready'

const FAST_READY_BUDGET_MS = 180
const REVEAL_DURATION_MS = 2200
const CAROUSEL_INTERVAL_MS = 6700

const visibleCarouselImages = computed(() =>
  carouselImages
    .map((src, index) => ({ src, index }))
    .filter(({ index }) => imageStates.value[index] === 'loaded')
)
```

`v3` 主要做了四件事：

- 给第一张高清图一个 `180ms` 的快速探测窗口，能解码就跳过 `poster`。
- 探测失败才回退到绿色 `poster`。
- `poster` 退场统一拉到 `2.2s`。
- 不再等 4 张图全部就绪，只在 `visibleCarouselImages` 这个已加载成功集合里轮播，可用图片数大于 1 才启动轮播。

## 结果：哪些是实测，哪些是预估

- `npm run build` 已通过。
- `src/assets/background/generated/poster.avif` 当前约 `764B`，这个值会随着背景重新生成有小幅波动。
- 4 张正式轮播图合计约 `1.10MB`。
- `BackgroundCarousel` chunk 约 `4.43kB / gzip 2.39kB`。
- `HomeView` chunk 约 `10.13kB / gzip 4.38kB`。
- `LeaderboardCard` chunk 约 `11.45kB / gzip 3.56kB`。
- 当前 `dist/index.html` 只预载 `vue-core`、`router-pinia`、`vendor`、`http`。

| 版本 | 关键动作 | 解决什么 | 代价是什么 |
| --- | --- | --- | --- |
| v1 | 分包、压缩、缓存、请求时序、请求取消 | 先把首页真正拖慢的链路拆掉 | 需要重排首页数据流和路由守卫 |
| v2 | `poster -> 高清轮播` 双层策略 | 解决“快了但切换生硬” | 逻辑偏保守，必须等资源更完整 |
| v3 | 首图快速路径、渐进轮播集合 | 兼顾缓存命中和慢网场景 | 背景状态机更复杂 |

- `/login` 冷启动：`2.5x - 4x`
- `/home` 冷启动：`2.2x - 3.2x`
- 二次访问或重复进入：`5x+`

这里要强调一句，真实工程不是神话故事。当前构建里 `ConsoleSidebar` 这个 chunk 仍然大约有 `1.07MB`，Vite 也还会给大 chunk 警告，后台控制台部分还有继续拆的空间。

## 如果面试官问我，我会怎么讲

- 首屏慢不是一个点，而是资源传输、背景图策略和首页请求链路叠加出来的问题。
- 第一轮先降首屏负担，第二轮补视觉过渡，第三轮做快速路径和渐进轮播。
- 难点不是单纯压资源，而是判断哪些数据必须首屏拿，哪些应该延后。
- 结果是首屏体感明显改善了，但我也保留了后续优化点，没有把项目讲成“已经完美”。
