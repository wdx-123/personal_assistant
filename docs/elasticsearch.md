很好的问题！我来深入解析 Elasticsearch 的底层存储和删除机制：

## Elasticsearch 底层存储架构

### 1. 数据是如何存储的？

```
┌─────────────────┐
│   Document      │  → JSON 格式
└─────────────────┘
        ↓
┌─────────────────┐
│   Shard         │  → 分片（Lucene 索引）
└─────────────────┘
        ↓
┌─────────────────┐
│   Segment       │  → 不可变的索引段
└─────────────────┘
        ↓
┌─────────────────┐
│   .fdt, .fdx    │  → 字段数据文件
│   .tim, .doc    │  → 倒排索引文件
└─────────────────┘
```

### 2. 创建文档的底层流程

```go
// 你的创建代码
_, err := global.ESClient.Index(elasticsearch.ArticleIndex())
    .Request(a)
    .Refresh(refresh.True)
    .Do(context.TODO())
```

**底层发生了什么：**

1. **内存缓冲**：
   ```
   ┌─────────────┐
   │ Memory      │ ← 文档先写入内存缓冲区
   │ Buffer      │
   └─────────────┘
   ```

2. **事务日志**：
   ```
   ┌─────────────┐
   │ Translog    │ ← 同时写入事务日志（防丢数据）
   │ (WAL)       │
   └─────────────┘
   ```

3. **Refresh 操作**：
   ```go
   // refresh.True 的作用
   Refresh(refresh.True) // 强制刷新，创建新的 Lucene Segment
   ```

4. **最终存储**：
   ```
   ┌───────────────────┐
   │ Segment Files     │
   │  - .fdt (字段数据) │
   │  - .fdx (字段索引) │
   │  - .tim (词典)    │
   │  - .doc (倒排表)   │
   └───────────────────┘
   ```

### 3. 删除文档的底层机制

```go
// 你的批量删除代码
for _, id := range ids {
    request = append(request, types.OperationContainer{
        Delete: &types.DeleteOperation{Id_: &id}
    })
}
```

**底层删除原理（标记删除）：**

```
// 不是真的删除，而是标记删除
┌─────────────────────────┐
│ Segment 1               │
│ ┌─────────────────────┐ │
│ │ doc1: "hello world" │ │ ← 活跃文档
│ │ doc2: "marked del"  │ │ ← 标记为删除
│ │ doc3: "test data"   │ │ ← 活跃文档
│ └─────────────────────┘ │
└─────────────────────────┘
```

### 4. 真正的物理删除时机

**段合并（Segment Merge）时：**
```
┌─────────────────┐    ┌─────────────────┐
│ Segment A       │    │ Segment B       │
│ - doc1 (active) │ →  │ - doc1 (active) │
│ - doc2 (del)    │    │ - doc3 (active) │
│ - doc4 (active) │    └─────────────────┘
└─────────────────┘    
         ↓ 合并时丢弃被标记删除的文档
┌─────────────────┐
│ Segment C       │
│ - doc1 (active) │
│ - doc3 (active) │
│ - doc4 (active) │
└─────────────────┘
```

### 5. 底层 HTTP 请求对比

**创建请求：**
```http
POST /article_index/_doc
{
  "title": "Go教程",
  "content": "内容..."
}
```

**删除请求（批量）：**
```http
POST /article_index/_bulk
{"delete": {"_id": "123"}}
{"delete": {"_id": "456"}}
```

### 6. 文件系统层面的变化

**创建后：**
```
indices/
└── article_index/
    ├── 0/ (shard)
    │   ├── index/ (segments)
    │   │   ├── _0.fdt
    │   │   ├── _0.fdx
    │   │   └── segments_1
    │   └── translog/
    │       └── translog-1.tlog
```

**删除后：**
```
indices/
└── article_index/
    ├── 0/
    │   ├── index/
    │   │   ├── _0.fdt      # 包含删除标记
    │   │   ├── _0.fdx
    │   │   └── segments_1
    │   └── translog/
    │       └── translog-2.tlog  # 记录删除操作
```

### 7. 性能影响

```go
// 创建时的性能考虑
Refresh(refresh.True)  // 立即刷新 → 高性能损耗
Refresh(refresh.False) // 延迟刷新 → 高性能

// 删除时的性能考虑
批量删除 vs 单条删除 → 批量效率高很多
```

### 总结
- **创建**：写入内存 → Translog → Refresh → Segment 文件
- **删除**：标记删除 → Translog → 段合并时物理删除
- **底层**：都是通过 Lucene 段文件管理，不是直接操作磁盘文件

这就是为什么 ES 删除操作很快（只是标记），但磁盘空间不会立即释放的原因！

```go
Elasticsearch 查询层层嵌套的核心掌握要点
1. 基础结构层次（必须掌握）
   第一层：Request 容器
   &search.Request{
   Query: ...,     // 查询条件
   Sort: ...,      // 排序规则
   Size: ...,      // 返回数量
   From: ...,      // 分页起始
   Source: ...,    // 返回字段
   }
   第二层：Query 查询类型
   &types.Query{
   Match: ...,     // 全文搜索
   Term: ...,      // 精确匹配
   Bool: ...,      // 布尔组合
   Range: ...,     // 范围查询
   Wildcard: ...,  // 通配符查询
   }


/*Create存入的的方向
阶段1 ：Go结构体
   ┌─────────────────────────┐
   │ elasticsearch.Article   │
   │ {                       │
   │   Title: "Go教程",      │
   │   Content: "内容...",   │
   │   Tags: ["Go", "编程"]  │
   │ }                       │
   └─────────────────────────┘
↓ .Request(a)
阶段2：客户端req字段
// ES客户端内部处理
   func (r *Index) Do(ctx context.Context) (*IndexResponse, error) {
   // 1. 将req对象序列化为JSON
   body, err := json.Marshal(r.req)
   if err != nil {
      return nil, err
   }

    // 2. 构建HTTP请求
    httpReq := &http.Request{
        Method: "POST",
        URL:    fmt.Sprintf("/%s/_doc", r.index),
        Body:   bytes.NewReader(body), // JSON数据作为请求体
    }

    // 3. 发送到ES服务器
    return r.client.Do(httpReq)
}
↓ .Do() 内部json.Marshal
阶段3：ES服务器接收到HTTP请求后：
   {
      "method": "POST",
      "url": "/article_index/_doc",
      "body": {
      "title": "Go语言教程",
      "content": "这是文章内容...",
      "category": "编程"
   }
}
↓ HTTP POST到ES
阶段4：ES服务器将请求体中的JSON数据存储为文档的 _source 字段：
{
   "_index": "article_index",
   "_id": "generated_id_123",
   "_source": {
      "title": "Go语言教程",
      "content": "这是文章内容...",
      "category": "编程"
    }
}
*/
```