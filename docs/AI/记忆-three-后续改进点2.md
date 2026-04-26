后续优化点，核心就是把现在的 **“能用版上下文压缩”**，升级成 **“更准、更稳、更可观测、更省 token 的成熟版”**。

现在第 4 步的压缩方式是：

```text
memory message + recent turns
```

也就是：

```text
会话摘要 + 用户 facts + 最近几轮原始消息
```

这个方案能用，但比较粗。后续可以从 7 个方向优化。

---

## 1. 引入真正的 LLM Compressor

现在的 summary 主要来自第 3 步 writeback 的规则版摘要，偏“拼接 + 截断”。

后续应该做成：

```text
旧 summary + 最近若干轮消息
-> LLM 重新压缩
-> 新 summary / key_points / open_loops
```

也就是让 LLM 真正理解：

```text
哪些是已确定结论
哪些是用户当前目标
哪些是未完成问题
哪些旧内容可以删掉
哪些新内容必须保留
```

现在是“机械压缩”，后续是“语义压缩”。

---

## 2. summary 要优先保留最新信息

你之前也发现了一个问题：

```text
旧 summary + 最近一轮
-> 从前往后截断
```

如果旧 summary 很长，新一轮内容可能被截掉。

后续应该改成：

```text
最新内容优先
旧 summary 可被压缩或裁剪
```

更合理的策略是：

```text
recent facts / open loops / latest decisions 优先保留
老旧背景信息降权
```

否则 AI 会记住很早以前的东西，反而忘了刚刚聊到哪里。

---

## 3. recent turns 不要只按轮数，也要按 token 预算

现在是：

```text
RecentRawTurns = 8
```

这个简单，但不精确。

因为有些一轮很短，有些一轮特别长。

后续更成熟的做法是：

```text
先保留最近 N 轮
再按 token budget 裁剪
```

或者：

```text
recent turns 最多占 3000 tokens
memory message 最多占 1500 tokens
当前 query 永远保留
```

也就是从“按轮数”升级成“按 token 预算”。

---

## 4. facts 要做优先级排序

现在 facts 可能只是按更新时间/limit 读取。

后续应该按重要性排序，比如：

```text
用户显式偏好 > 当前目标 > 最近学习画像 > 普通历史偏好
```

还可以结合：

```text
source_kind
confidence
updated_at
namespace
expires_at
```

比如当前用户问的是面试，那就优先放：

```text
user_preference
current_goal
interview_related_profile
```

不相关的 fact 可以不带。

否则 facts 多了以后，也会挤占上下文。

---

## 5. synthetic memory message 最好不要长期用 assistant role

现在计划里说：

```text
synthetic memory message 暂用 assistant role
```

这可以临时用，但不完美。

因为这条 message 不是 assistant 真正说过的话，它更像系统背景资料。

后续如果 runtime 支持，最好改成：

```text
system / developer / memory / context
```

类似：

```text
system memory block
```

这样模型更容易理解：

> 这是系统恢复的背景，不是普通聊天历史。

---

## 6. 增加可观测和调试信息

后续一定要能看到：

```text
本轮是否触发压缩
原始历史 token 多少
压缩后 token 多少
保留了几轮 recent turns
带入了哪些 facts
带入了哪条 summary
哪些内容被裁掉
```

否则你会很难排查：

```text
为什么 AI 忘了前面内容？
为什么某个用户偏好没生效？
为什么上下文还是很长？
为什么 summary 没更新？
```

可以记录成 trace：

```text
memory_recall_started
memory_summary_loaded
memory_facts_loaded
context_compressed
context_compress_skipped
memory_prompt_built
```

这对面试和线上排查都很加分。

---

## 7. 后续接入 RAG / documents

第 4 步只恢复：

```text
summary + facts
```

还不读：

```text
documents / Qdrant / RAG
```

后续第 5、6 步可以把 `AIMemoryDocument` 做成：

```text
document -> chunk -> embedding -> Qdrant
```

然后第 7 步混合召回时，完整上下文来源会变成：

```text
conversation summary
+ stable facts
+ recent turns
+ RAG documents
+ realtime tools
```

这时候才是比较完整的 memory retrieval。

---

## 最重要的优化顺序

我建议你按这个顺序做：

```text
1. 修 summary 策略，避免新内容被旧摘要挤掉
2. 增加压缩可观测，能看到压缩前后发生了什么
3. recent turns 从按轮数升级成 token budget
4. facts 做优先级排序和 namespace 过滤
5. 引入 LLM Compressor，生成高质量 summary/key_points/open_loops
6. synthetic memory message 改成更合适的 role/block
7. 接入 RAG documents 和 Qdrant 召回
```

---

## 面试可以这样说

你可以这样讲：

> 第 4 步第一版采用工程上稳定的压缩策略，也就是 conversation summary + stable facts + recent turns。后续我会重点优化 summary 质量和上下文预算。比如引入 LLM Compressor，让它基于旧 summary 和最近若干轮重新生成结构化摘要；recent turns 不再只按轮数，而是按 token budget 裁剪；facts 会按来源、更新时间、namespace 和重要性排序；同时增加可观测，记录压缩前后 token、保留了哪些 facts、summary 是否命中。等 RAG 阶段完成后，再把 document 召回结果纳入混合上下文。

最简单记：

> **现在是“规则版压缩能用”，后续要升级成“LLM 语义压缩 + token 预算 + facts 排序 + RAG 混合召回”。**
