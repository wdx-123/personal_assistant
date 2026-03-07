package metrics

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"personal_assistant/internal/model/entity"

	"go.uber.org/zap"
)

// Backend 指标后端默认实现：
//   - 内存按维度+时间桶聚合（buckets）以降低写入放大
//   - 周期性 flush 到 Store（通常是 DB）进行增量持久化
//
// 适用场景：
//   - 写多读少的 HTTP 指标（QPS/错误率/平均耗时/最大耗时等）
//   - 对实时性有一定要求，但允许秒级延迟
//
// 可靠性语义：
//   - 进程内聚合：进程崩溃会丢失尚未 flush 的内存指标（可接受/需评估）
//   - flush 失败：将本次待写数据 merge 回内存桶，后续重试（“至少一次”写入倾向）
//
// 并发模型：
//   - RecordHTTP 高频调用，仅在 addMetric 时短锁保护 map 写入
//   - flush 会 swapBuckets 将当前桶整体换出，缩短持锁时间并避免长时间阻塞写入
type Backend struct {
	// store 为持久化存储接口（例如 MySQL/ClickHouse/Prometheus 适配层）。
	// 为空时：RecordHTTP 会直接 no-op（避免影响业务链路），QueryMetrics 返回 nil。
	store Store

	// log 用于记录 flush/rollup 等后台任务错误；为 nil 时使用 zap.NewNop()。
	log *zap.Logger

	// opt 为运行参数（flush 周期、批量大小、保留策略、服务名兜底等）。
	opt Options

	// mu 保护 buckets 读写（map 非并发安全）。
	mu sync.Mutex

	// buckets 是内存聚合桶：key=维度+时间桶，value=该桶累计指标。
	// value 使用 entity.ObservabilityMetric，便于与 Store 的写入结构对齐。
	buckets map[string]*entity.ObservabilityMetric

	// startOnce 确保 Start 只启动一次后台 flush goroutine。
	startOnce sync.Once
}

// NewBackend 创建 Backend，并设置生产默认值。
// 默认值说明：
//   - FlushInterval：默认 1s，保证指标秒级落库；可按吞吐/实时性调整。
//   - DBBatchSize：默认 500，平衡 DB 写入吞吐与单次失败重试成本。
//   - FineRetentionDays/DayRetentionDays：默认 45/365，满足常见排障与看板周期。
//   - ServiceName：为空则兜底为 unknown_service，避免空维度导致聚合丢失。
func NewBackend(store Store, log *zap.Logger, opt Options) *Backend {
	// 日志兜底：避免调用方未传 log 导致 nil deref。
	if log == nil {
		log = zap.NewNop()
	}
	// flush 周期兜底：不允许非正值，避免 ticker panic/失效。
	if opt.FlushInterval <= 0 {
		opt.FlushInterval = time.Second
	}
	// 批量写入大小兜底：过小会导致写放大，过大单次失败成本高；默认 500 为折中。
	if opt.DBBatchSize <= 0 {
		opt.DBBatchSize = 500
	}
	// 细粒度保留兜底：minute/5m 通常保留较短周期，默认 45 天。
	if opt.FineRetentionDays <= 0 {
		opt.FineRetentionDays = 45
	}
	// 日粒度保留兜底：用于长期趋势分析，默认 365 天。
	if opt.DayRetentionDays <= 0 {
		opt.DayRetentionDays = 365
	}
	// 服务名兜底：用于补齐 record.Service，避免出现空维度影响聚合/查询。
	if strings.TrimSpace(opt.ServiceName) == "" {
		opt.ServiceName = "unknown_service"
	}
	// 初始化 buckets：map 必须分配，否则首次写入会 panic。
	return &Backend{
		store:   store,
		log:     log,
		opt:     opt,
		buckets: make(map[string]*entity.ObservabilityMetric),
	}
}

// Start 启动周期性刷盘（flush）后台任务。
// 设计要点：
//   - 仅在 Enabled 且 store 非空时启动（否则无意义且可能引入 goroutine 泄漏风险）。
//   - 使用 startOnce 防止多次启动多个 ticker。
//   - 在 ctx.Done() 时执行一次 flush，尽量减少停机丢失（仍不保证完全不丢）。
//
// 注意：
//   - 这里 flush 使用 context.Background()，避免上游 ctx 取消导致 flush 被提前取消。
//   - 如果你希望 flush/rollup 能被优雅停止并设置超时，可在这里改成派生 ctx（但这属于逻辑变更）。
func (b *Backend) Start(ctx context.Context) {
	// 未启用或无 store：直接返回，不启动后台 goroutine。
	if !b.opt.Enabled || b.store == nil {
		return
	}
	b.startOnce.Do(func() {
		// 后台 goroutine：按 FlushInterval 周期 flush 内存桶到持久层。
		go func() {
			// ticker 用于周期性触发 flush。
			ticker := time.NewTicker(b.opt.FlushInterval)
			defer ticker.Stop()
			for {
				select {
				// 服务退出/上游取消：最后 flush 一次，并退出 goroutine。
				case <-ctx.Done():
					b.flush(context.Background())
					return
				// 周期触发：执行一次 flush。
				case <-ticker.C:
					b.flush(context.Background())
				}
			}
		}()
	})
}

// RecordHTTP 记录一次 HTTP 请求采样。
// 该方法位于业务请求路径上，目标是“尽量轻量”：
//   - 做少量规范化（时间/方法/路由/服务名/状态类/耗时）
//   - 根据口径判断是否错误
//   - 将记录写入内存聚合桶（1m/5m 两套粒度）
//
// 失败/禁用语义：
//   - Enabled=false 或 store=nil 或 record=nil：直接返回 nil（不影响主流程）。
func (b *Backend) RecordHTTP(ctx context.Context, record *HTTPRecord) error {
	// 关闭开关/无 store/空 record：不采集，不报错（可观测性系统应尽量不影响业务）。
	if !b.opt.Enabled || b.store == nil || record == nil {
		return nil
	}

	// 时间兜底：Timestamp 为空则使用当前时间（默认本地时区；如需统一 UTC 建议上游传 UTC）。
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now()
	}

	// Method 规范化：统一大写并去空白；为空则使用 UNKNOWN，避免维度为空造成聚合异常。
	record.Method = strings.ToUpper(strings.TrimSpace(record.Method))
	if record.Method == "" {
		record.Method = "UNKNOWN"
	}

	// RouteTemplate 规范化：去空白；为空则使用 /unknown。
	// 生产建议：尽量使用“模板路由”而非具体 path，避免高基数导致存储爆炸。
	record.RouteTemplate = strings.TrimSpace(record.RouteTemplate)
	if record.RouteTemplate == "" {
		record.RouteTemplate = "/unknown"
	}

	// Service 规范化：为空则使用配置 ServiceName，保证维度稳定。
	record.Service = strings.TrimSpace(record.Service)
	if record.Service == "" {
		record.Service = b.opt.ServiceName
	}

	// ErrorCode 去空白：用于错误维度聚合（高基数风险需由上游控制）。
	record.ErrorCode = strings.TrimSpace(record.ErrorCode)

	// StatusClass 兜底：若未提供则由 StatusCode/100 推导（例如 200->2，503->5）。
	if record.StatusClass <= 0 {
		record.StatusClass = record.StatusCode / 100
	}

	// Latency 兜底：不允许负数，避免破坏统计（total/max）。
	if record.LatencyMs < 0 {
		record.LatencyMs = 0
	}

	// 错误口径：HTTP status >=400 或存在业务错误码即认为错误。
	// 注意：这个口径会把 4xx 也算“错误”，是否符合你们看板/告警需求需要统一定义。
	isError := record.StatusCode >= 400 || record.ErrorCode != ""

	// 写入两个粒度的内存桶：
	//   - 1m：用于更实时与更细粒度的看板/排障
	//   - 5m：用于中等时间跨度趋势，数据点更少、查询更轻
	b.addMetric("1m", record.Timestamp.Truncate(time.Minute), record, isError)
	b.addMetric("5m", record.Timestamp.Truncate(5*time.Minute), record, isError)
	return nil
}

// addMetric 将单次采样累加到指定粒度/时间桶的聚合行中。
// 关键点：
//   - key 由 granularity + bucketStart + 维度字段拼接，确保同一桶同一维度落在同一行。
//   - 在锁内完成创建/更新，保证并发安全。
//   - bucketStart 使用 UTC 存储，避免跨时区部署导致桶边界不一致（key 也用 UTC 格式）。
func (b *Backend) addMetric(
	granularity string,
	bucketStart time.Time,
	record *HTTPRecord,
	isError bool,
) {
	// key 采用明确分隔符拼接，便于唯一性与调试。
	// 注意：维度字段若包含 "|" 会影响解析，但这里不做反解析，仅需唯一即可；上游应避免该字符。
	key := fmt.Sprintf(
		"%s|%s|%s|%s|%s|%d|%s",
		granularity,
		bucketStart.UTC().Format(time.RFC3339),
		record.Service,
		record.RouteTemplate,
		record.Method,
		record.StatusClass,
		record.ErrorCode,
	)

	// 加锁保护 map：map 并发写会 panic。
	b.mu.Lock()
	defer b.mu.Unlock()

	// 如果桶不存在则初始化一行（维度字段+桶信息）。
	row, ok := b.buckets[key]
	if !ok {
		row = &entity.ObservabilityMetric{
			Granularity:   granularity,
			BucketStart:   bucketStart.UTC(),
			Service:       record.Service,
			RouteTemplate: record.RouteTemplate,
			Method:        record.Method,
			StatusClass:   record.StatusClass,
			ErrorCode:     record.ErrorCode,
		}
		b.buckets[key] = row
	}

	// 计数累加：每条 record 贡献一次请求计数。
	row.RequestCount++

	// 错误计数累加：按 isError 口径累加。
	if isError {
		row.ErrorCount++
	}

	// 总耗时累加：用于平均耗时计算（TotalLatencyMs / RequestCount）。
	row.TotalLatencyMs += record.LatencyMs

	// 最大耗时更新：用于观察突刺与尾延迟（注意这里只记录 max，不记录分位数）。
	if record.LatencyMs > row.MaxLatencyMs {
		row.MaxLatencyMs = record.LatencyMs
	}
}

// flush 将当前内存桶数据刷入持久层。
// 流程：
//  1. swapBuckets：快速换出当前 buckets，减少持锁时间，避免阻塞写入路径。
//  2. persistIncrement：分批调用 store.IncrementBatch 增量写入。
//  3. 若持久化失败：记录日志，并 mergeBack 把数据合并回内存桶，等待下次 flush 重试。
//
// 可靠性说明：
//   - mergeBack 是一种“失败回滚”策略，避免 flush 失败导致指标丢失。
//   - 但若进程在失败后崩溃，仍可能丢失未成功落库的数据（内存方案的固有限制）。
func (b *Backend) flush(ctx context.Context) {
	// 将当前桶 swap 出来（得到一份快照 rows），并把 buckets 清空成新 map。
	rows := b.swapBuckets()
	if len(rows) == 0 {
		return
	}
	// 批量落库失败：记录错误并将 rows 合并回内存桶，以便后续重试。
	if err := b.persistIncrement(ctx, rows); err != nil {
		b.log.Error("observability metrics flush failed", zap.Error(err), zap.Int("rows", len(rows)))
		b.mergeBack(rows)
	}
}

// swapBuckets 将当前 buckets 的内容复制为 rows，并把 buckets 重置为空 map。
// 这样 flush 的 DB 写入不会长时间持有 mu，减少对 RecordHTTP 的影响。
// 注意：
//   - 这里对每个 row 做浅拷贝（copyRow := *row），避免后续 mergeBack 或并发更新影响快照数据。
func (b *Backend) swapBuckets() []*entity.ObservabilityMetric {
	b.mu.Lock()
	defer b.mu.Unlock()

	// 预分配 rows 容量，减少 append 扩容。
	rows := make([]*entity.ObservabilityMetric, 0, len(b.buckets))

	// 拷贝每行：避免持有指向内部 map value 的指针导致数据竞态。
	for _, row := range b.buckets {
		copyRow := *row
		rows = append(rows, &copyRow)
	}

	// 重置 buckets：释放旧 map 引用，下一轮写入从空桶开始累加。
	b.buckets = make(map[string]*entity.ObservabilityMetric)
	return rows
}

// mergeBack 将未成功落库的 rows 合并回当前 buckets。
// 用于 flush 失败后的回滚与重试；合并逻辑按 key（粒度+桶+维度）进行累加。
// 注意：
//   - mergeBack 可能与并发 RecordHTTP 同时发生，因此内部在更新 buckets 时需要加锁。
//   - 这里没有做去重：如果 persistIncrement 部分成功、部分失败且返回错误，可能导致“已落库部分被重复 mergeBack”，
//     从而产生重复计数风险。要彻底避免需要更复杂的失败分批处理/幂等写入（这属于逻辑改动）。
func (b *Backend) mergeBack(rows []*entity.ObservabilityMetric) {
	for _, row := range rows {
		if row == nil {
			continue
		}

		// 这里构造 record 只是为了复用 key 拼接逻辑；LatencyMs 不参与 key。
		record := &HTTPRecord{
			Timestamp:     row.BucketStart,
			Service:       row.Service,
			RouteTemplate: row.RouteTemplate,
			Method:        row.Method,
			StatusClass:   row.StatusClass,
			ErrorCode:     row.ErrorCode,
			LatencyMs:     0,
		}

		// 与 addMetric 保持一致的 key 格式，确保能合并到同一桶。
		key := fmt.Sprintf(
			"%s|%s|%s|%s|%s|%d|%s",
			row.Granularity,
			row.BucketStart.UTC().Format(time.RFC3339),
			record.Service,
			record.RouteTemplate,
			record.Method,
			record.StatusClass,
			record.ErrorCode,
		)

		// 合并需要锁保护。
		b.mu.Lock()
		cached, ok := b.buckets[key]
		if !ok {
			// 若当前桶不存在：直接放回一份拷贝，避免引用外部 row 指针。
			copyRow := *row
			b.buckets[key] = &copyRow
			b.mu.Unlock()
			continue
		}

		// 若存在：按同口径做累加合并。
		cached.RequestCount += row.RequestCount
		cached.ErrorCount += row.ErrorCount
		cached.TotalLatencyMs += row.TotalLatencyMs
		if row.MaxLatencyMs > cached.MaxLatencyMs {
			cached.MaxLatencyMs = row.MaxLatencyMs
		}
		b.mu.Unlock()
	}
}

// persistIncrement 将 rows 分批调用 store.IncrementBatch 进行增量写入。
// 分批原因：
//   - 避免一次提交过大导致 DB 连接/事务压力过高。
//   - 失败时更容易定位问题（但当前实现是一旦某批失败直接返回，上一批可能已成功）。
func (b *Backend) persistIncrement(ctx context.Context, rows []*entity.ObservabilityMetric) error {
	if len(rows) == 0 {
		return nil
	}
	// 按 DBBatchSize 分批写入。
	for i := 0; i < len(rows); i += b.opt.DBBatchSize {
		end := i + b.opt.DBBatchSize
		if end > len(rows) {
			end = len(rows)
		}
		// 增量写入：由 store 保证原子累加/并发安全（通常 SQL upsert + 累加）。
		if err := b.store.IncrementBatch(ctx, rows[i:end]); err != nil {
			return err
		}
	}
	return nil
}

// QueryMetrics 查询指标聚合点。
// 行为说明：
//   - 该方法只读 store（落库数据），不读取内存 buckets；因此可能有最多 FlushInterval 的延迟。
//   - store.Query 返回 entity.ObservabilityMetric，再转换为对外的 MetricPoint。
//   - 若 store=nil 返回 nil（无数据），不返回错误以保持“可观测性不影响业务”的原则。
func (b *Backend) QueryMetrics(
	ctx context.Context,
	granularity string,
	start time.Time,
	end time.Time,
	service string,
	routeTemplate string,
	method string,
	statusClass int,
	errorCode *string,
	limit int,
) ([]*MetricPoint, error) {
	// 无 store：无法查询，返回空。
	if b.store == nil {
		return nil, nil
	}

	// 交由存储层执行筛选与 limit，避免拉全量回内存再过滤。
	rows, err := b.store.Query(
		ctx,
		granularity,
		start,
		end,
		service,
		routeTemplate,
		method,
		statusClass,
		errorCode,
		limit,
	)
	if err != nil {
		return nil, err
	}

	// 转换为 API 返回结构。
	points := make([]*MetricPoint, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		points = append(points, &MetricPoint{
			Granularity:    row.Granularity,
			BucketStart:    row.BucketStart,
			Service:        row.Service,
			RouteTemplate:  row.RouteTemplate,
			Method:         row.Method,
			StatusClass:    row.StatusClass,
			ErrorCode:      row.ErrorCode,
			RequestCount:   row.RequestCount,
			ErrorCount:     row.ErrorCount,
			TotalLatencyMs: row.TotalLatencyMs,
			MaxLatencyMs:   row.MaxLatencyMs,
		})
	}
	return points, nil
}

// RollupAndCleanup 执行“聚合（rollup）+ 清理（retention）”。
// 典型用途：
//   - minute/5m 数据用于短期排障与实时看板，保留较短（FineRetentionDays）。
//   - day/week 数据用于长期趋势，保留更长（DayRetentionDays/WeekRetentionDays）。
//
// 执行步骤：
//  1. flush：先把内存桶刷入存储，减少 rollup 漏算。
//  2. rollup 细 -> 天：将 1m 聚合成 1d（截止到 cutoffFine）。
//  3. rollup 天 -> 周：将 1d 聚合成 1w（这里仅聚合到 now-7d，避免当天/本周未完成桶抖动）。
//  4. 删除过期数据：按粒度清理历史数据，控制成本。
func (b *Backend) RollupAndCleanup(ctx context.Context, now time.Time) error {
	// 未启用或无 store：不执行任何后台任务。
	if !b.opt.Enabled || b.store == nil {
		return nil
	}
	// now 兜底：便于调用方不传或测试时传固定时间。
	if now.IsZero() {
		now = time.Now()
	}

	// 先 flush：减少“内存未落库导致 rollup 缺数据”的概率。
	b.flush(ctx)

	// cutoffFine：细粒度数据保留截止点（早于该时间的数据会被 rollup 并随后删除）。
	cutoffFine := now.AddDate(0, 0, -b.opt.FineRetentionDays)

	// 1m -> 1d：将 minute 聚合到 day，范围 [start, cutoffFine]（start=0 表示由 store 自行处理全量或从最早可用处开始）。
	// 注意：这里 start 传 time.Time{}，依赖 store.Aggregate 的实现约定。
	if err := b.rollup(ctx, "1m", "1d", time.Time{}, cutoffFine); err != nil {
		return err
	}

	// 1d -> 1w：将 day 聚合到 week。
	// end 使用 now-7d：通常是为了避免聚合“未完整的一周”导致数据反复变化（具体口径需与 store 对齐）。
	if err := b.rollup(ctx, "1d", "1w", time.Time{}, now.AddDate(0, 0, -7)); err != nil {
		return err
	}

	// 删除过期细粒度：1m/5m 都按 FineRetentionDays 清理（与上面 cutoffFine 一致）。
	if err := b.store.DeleteBeforeByGranularity(ctx, "1m", cutoffFine); err != nil {
		return err
	}
	if err := b.store.DeleteBeforeByGranularity(ctx, "5m", cutoffFine); err != nil {
		return err
	}

	// 删除过期日粒度：按 DayRetentionDays 清理。
	if b.opt.DayRetentionDays > 0 {
		if err := b.store.DeleteBeforeByGranularity(ctx, "1d", now.AddDate(0, 0, -b.opt.DayRetentionDays)); err != nil {
			return err
		}
	}

	// 删除过期周粒度：按 WeekRetentionDays 清理。
	if b.opt.WeekRetentionDays > 0 {
		if err := b.store.DeleteBeforeByGranularity(ctx, "1w", now.AddDate(0, 0, -b.opt.WeekRetentionDays)); err != nil {
			return err
		}
	}
	return nil
}

// rollup 将 fromGranularity 的数据聚合到 toGranularity，并写回存储。
// 工作流程：
//  1. 调用 store.Aggregate 计算聚合结果（通常由 SQL GROUP BY 完成）。
//  2. 将聚合结果分批 UpsertAbsoluteBatch（幂等覆盖写入），避免重复 rollup 导致重复累加。
//
// 约定：
//   - end 为空则不执行（避免误聚合）。
//   - start 可以为零值，具体含义由 store.Aggregate 实现定义（常见做法：从最早数据开始或从上次游标开始）。
func (b *Backend) rollup(
	ctx context.Context,
	fromGranularity string,
	toGranularity string,
	start time.Time,
	end time.Time,
) error {
	// end 为空：无有效时间范围，直接返回。
	if end.IsZero() {
		return nil
	}

	// 在存储层聚合：返回聚合后的指标行（已按 toGranularity 的 bucketStart/维度聚合）。
	rows, err := b.store.Aggregate(ctx, fromGranularity, toGranularity, start, end)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	// 分批 upsert：减少单次写入压力；UpsertAbsoluteBatch 应为幂等覆盖写入。
	for i := 0; i < len(rows); i += b.opt.DBBatchSize {
		batchEnd := i + b.opt.DBBatchSize
		if batchEnd > len(rows) {
			batchEnd = len(rows)
		}
		if err := b.store.UpsertAbsoluteBatch(ctx, rows[i:batchEnd]); err != nil {
			return err
		}
	}
	return nil
}

// 编译期断言 Backend 实现了 MetricsBackend 接口，防止未来改动导致漏实现而运行期才报错。
var _ MetricsBackend = (*Backend)(nil)
