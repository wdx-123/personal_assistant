# 可观测性平台前端设计方案 (Observability UI Design Spec)

本设计方案基于现有后端 API 能力，旨在打造一套**现代化、直观、高性能**的系统监控与链路排查界面。设计风格参考 Grafana/Jaeger/Datadog，强调信息的**分层展示**与**关联跳转**。

---

## 0. 全局导航与布局 (Global Layout)

采用左侧侧边栏或顶部导航栏结构。

*   **Menu Items**:
    *   📊 **Dashboard** (监控大盘)
    *   🔍 **Trace Explorer** (链路探索)
    *   ⚙️ **Settings** (配置，如采样率等 - 预留)

---

## 1. 监控大盘 (Dashboard) - 宏观视角

**目标**：一屏掌握系统健康状况，发现异常趋势。

### 1.1 UI 布局图示

```text
+-----------------------------------------------------------------------+
|  [Dashboard]  Time Range: [ Last 1 Hour v ]  Auto-Refresh: [ 10s v ]  |
|  Filter: Service [ All v ]  Method [ All v ]                          |
+-----------------------------------------------------------------------+
|                                                                       |
|  [ Score Cards ]                                                      |
|  +----------------+  +----------------+  +----------------+           |
|  | RPS (Total)    |  | Error Rate     |  | Avg Latency    |           |
|  | 1,245 req/s    |  | 0.52 %         |  | 45 ms          |           |
|  +----------------+  +----------------+  +----------------+           |
|                                                                       |
+-----------------------------------------------------------------------+
|  [ Charts Row 1 ]                                                     |
|  +-------------------------------------+  +-------------------------+ |
|  | Request Volume (Requests per min)   |  | Error Rate Trend (%)    | |
|  | [Line Chart: 2xx/4xx/5xx]           |  | [Bar Chart: By Service] | |
|  +-------------------------------------+  +-------------------------+ |
+-----------------------------------------------------------------------+
|  [ Charts Row 2 ]                                                     |
|  +-------------------------------------+  +-------------------------+ |
|  | Latency Trend (P99/Avg)             |  | Top Slow Routes         | |
|  | [Line Chart]                        |  | [List/Table]            | |
|  +-------------------------------------+  +-------------------------+ |
+-----------------------------------------------------------------------+
```

### 1.2 组件详细设计

#### A. 筛选栏 (Filter Bar)
*   **Time Range**: 最近 15m, 1h, 6h, 24h, 7d, 自定义。
*   **Auto-Refresh**: Off, 5s, 10s, 30s, 1m。
*   **Service Dropdown**: 从 API 获取所有服务名列表。

#### B. 核心指标卡 (Score Cards)
*   **RPS**: 计算所选时间段内的平均请求速率。
*   **Error Rate**: (Error Count / Total Count) * 100%。
*   **Avg Latency**: `total_latency_ms / request_count`。

#### C. 图表区 (Charts)
*   **Request Volume**:
    *   Type: Stacked Area Chart / Line Chart
    *   X轴: 时间
    *   Y轴: 请求数
    *   Series: 成功(Green), 客户端错误(Yellow), 服务端错误(Red)
*   **Latency Trend**:
    *   Type: Line Chart
    *   Series: P99 (预估), Avg
*   **Top Slow Routes**:
    *   Type: Table
    *   Columns: Route, Method, Avg Latency, Max Latency
    *   Action: 点击路由跳转到 **Trace Explorer** 并自动填入筛选条件。

### 1.3 数据源 (API Mapping)

*   **API**: `POST /system/observability/metrics/query`
*   **Payload 构造**:
    ```json
    {
      "granularity": "minute", // 根据 Time Range 自动调整 (1h->minute, 7d->hour)
      "start_at": "2023-10-27T10:00:00Z",
      "end_at": "2023-10-27T11:00:00Z",
      "service": "user-service" // 可选
    }
    ```
*   **前端处理**: 接收 `list` 数组，按 `bucket_start` 分组渲染图表。

---

## 2. 链路探索器 (Trace Explorer) - 搜索视角

**目标**：多维度筛选，快速定位具体的问题请求（如“查找过去 1 小时内 user-service 的所有 5xx 错误”）。

### 2.1 UI 布局图示

```text
+-----------------------------------------------------------------------+
| [Trace Explorer]                                                      |
+-----------------------------------+-----------------------------------+
|  < Search Sidebar >               |  < Result List >                  |
|                                   |                                   |
|  Trace ID / Request ID:           |  Found 25 traces...               |
|  [_______________________]        |                                   |
|                                   |  +-----------------------------+  |
|  Service: [ All v ]               |  | [ERR] POST /api/v1/login    |  |
|  Stage:   [ All v ]               |  | TraceID: a1b2...            |  |
|  Status:  [ Error v ]             |  | Service: auth-service       |  |
|                                   |  | Duration: 520ms [======--]  |  |
|  Min Duration (ms):               |  | Time: 10:23:45              |  |
|  [ 500 ]                          |  +-----------------------------+  |
|                                   |                                   |
|  Time Range:                      |  +-----------------------------+  |
|  [ Last 1 Hour v ]                |  | [OK] GET /api/v1/users      |  |
|                                   |  | TraceID: c3d4...            |  |
|  [ Search Button ]                |  | Service: user-service       |  |
|                                   |  | Duration: 45ms  [=       ]  |  |
|                                   |  | Time: 10:23:42              |  |
|                                   |  +-----------------------------+  |
|                                   |                                   |
|                                   |  < Pagination: [1] 2 3 ... >      |
+-----------------------------------+-----------------------------------+
```

### 2.2 组件详细设计

#### A. 侧边栏筛选 (Search Sidebar)
*   **ID Search**: 输入 TraceID 或 RequestID 精确查找。
*   **Status**: All, Success, Error (重点高亮 Error)。
*   **Advanced**: Min Duration (虽然 API 暂未显式支持，建议前端设计上保留，后端跟进支持)。

#### B. 结果列表 (Result List)
*   **List Item Design**:
    *   **Header**: Method + Path (如 `POST /login`)。
    *   **Badges**: Status (Red/Green), Service Name (Blue)。
    *   **Metadata**: Trace ID (点击复制), Start Time (Relative, e.g., "5 mins ago").
    *   **Visualization**: Duration Bar (长度代表相对耗时，颜色代表状态)。
*   **Interaction**: 点击任意 Item 跳转到 **Trace Detail** 页。

### 2.3 数据源 (API Mapping)

*   **场景 1 (精确)**: 输入 ID 后，调用 `GET /traces/trace/:id` 或 `GET /traces/request/:id`，成功后直接跳转详情页。
*   **场景 2 (列表)**:
    *   **API**: `POST /system/observability/traces/query`
    *   **Payload**:
        ```json
        {
          "service": "auth-service",
          "status": "error",
          "start_at": "...",
          "end_at": "...",
          "limit": 20,
          "offset": 0,
          "include_payload": false, // 列表页关闭 Payload 以提升性能
          "include_error_detail": false
        }
        ```

---

## 3. 链路详情页 (Trace Detail) - 微观视角

**目标**：还原一次请求的完整执行路径，展示 Span 层级、耗时分布及堆栈信息。

### 3.1 UI 布局图示

```text
+-----------------------------------------------------------------------+
| [<- Back]  Trace ID: a1b2c3d4...  (User-Service)                      |
| Start: Oct 27 10:23:45  |  Duration: 520ms  |  Spans: 12  |  Errs: 1  |
+-----------------------------------------------------------------------+
|                                                                       |
|  [ Waterfall View (Gantt Chart) ]                                     |
|                                                                       |
|  Service        Operation               | Timeline (0ms -> 520ms)     |
|  -------------------------------------------------------------------  |
|  user-service   POST /login             | [=========================] |
|    auth-svc     verify_token            |   [=======]                 |
|      redis      get_session             |     [===]                   |
|    user-svc     db_query                |             [==========]    |
|      mysql      SELECT * FROM users...  |               [======]      |
|                                         |                             |
+-----------------------------------------+-----------------------------+
|                                         |                             |
| [ Span Detail Panel (Right Drawer) ]    |                             |
|                                         |                             |
| Title: mysql: SELECT * FROM users...    |                             |
| ------------------------------------    |                             |
| [Tags]                                  |                             |
| db.system: mysql                        |                             |
| db.statement: SELECT * FROM ...         |                             |
|                                         |                             |
| [Payload]                               |                             |
| (Empty)                                 |                             |
|                                         |                             |
| [Error]                                 |                             |
| Code: 1064                              |                             |
| Msg: You have an error in SQL syntax    |                             |
| Stack: ...                              |                             |
|                                         |                             |
+-----------------------------------------------------------------------+
```

### 3.2 组件详细设计

#### A. 头部摘要 (Header Summary)
*   展示总耗时、开始时间、涉及服务数量。
*   如果有 Error，显示醒目的红色警告。

#### B. 瀑布图 (Waterfall)
*   **Tree Structure**: 左侧展示 Span 的父子层级关系 (缩进)。
*   **Timeline**: 右侧展示 Span 的开始时间和持续时长条。
*   **Color Coding**:
    *   按 Service 分色：不同服务使用不同颜色，直观展示服务边界。
    *   按 Status 分色：Error Span 标红。
*   **Interaction**: Hover 显示简要信息，Click 唤起右侧详情抽屉。

#### C. Span 详情抽屉 (Span Detail Drawer)
*   **Tabs**:
    *   **Overview**: Start Time, Duration, Service, Operation Name.
    *   **Tags**: 展示 `tags` map 中的所有键值对 (如 `http.method`, `db.statement`)。
    *   **Payload**: 展示 `request_snippet` / `response_snippet` (JSON 高亮)。
    *   **Error**: 仅在出错时显示。展示 `error_code`, `message`, `error_stack` (支持折叠/展开)。

### 3.3 数据源 (API Mapping)

*   **API**: `GET /system/observability/traces/trace/:trace_id` (或 Request ID)
*   **Params**: `include_payload=true`, `include_error_detail=true` (详情页需要完整数据)。
*   **前端处理**:
    *   后端返回的是 `List<Span>` (扁平数组)。
    *   前端需要根据 `span_id` 和 `parent_span_id` 将扁平数组转换为 **树形结构 (Tree)** 以渲染瀑布图。
    *   计算每个 Span 相对于 Root Span 的 `offset` (开始时间偏移量) 来绘制时间条。

---

## 4. 技术选型建议 (Tech Stack)

*   **Framework**: React / Vue 3
*   **UI Library**: Ant Design / Arco Design / Tailwind CSS
*   **Charts**:
    *   Metrics: **ECharts** 或 **Recharts** (适合时间序列)。
    *   Trace Waterfall: 建议使用专门的 Trace 可视化库，或自行封装 (基于 HTML Canvas 或 SVG)。虽然 ECharts 有 Gantt 图，但定制 Span 点击交互较繁琐，简单的 `div` + `flex` 布局或 SVG 更易控。
*   **Date Library**: Day.js (轻量，处理时区和格式化)。
