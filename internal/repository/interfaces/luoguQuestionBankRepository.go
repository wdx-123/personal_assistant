package interfaces

import (
	"context"

	"personal_assistant/internal/model/entity"
)

// LuoguQuestionBankRepository 洛谷题库仓储接口
type LuoguQuestionBankRepository interface {
	// GetAllPIDMap 获取所有题目的 PID 到 ID 的映射
	GetAllPIDMap(ctx context.Context) (map[string]uint, error)
	// BatchCreate 批量创建题目
	BatchCreate(ctx context.Context, questions []*entity.LuoguQuestionBank) error
	// GetByPID 根据PID获取题目
	GetByPID(ctx context.Context, pid string) (*entity.LuoguQuestionBank, error)
	// 从 Redis 缓存里根据洛谷题号 PID（比如 P1000）查到你本地题库表的 ID，并告诉你“有没有命中缓存”。
	GetCachedID(ctx context.Context, pid string) (uint, bool, error)
	// CacheID 写缓存
	CacheID(ctx context.Context, pid string, id uint) error
	// EnsureQuestionID 确保某个洛谷题目（按 PID 唯一）在数据库里存在，并拿到它的本地 ID（如果已存在就查出来），最后顺便写入 Redis 缓存
	EnsureQuestionID(ctx context.Context, question *entity.LuoguQuestionBank) (uint, error)
	// ListPIDIDAfterID 增量取数据-从题库表里“按自增 ID 向后分页”查询一批数据
	ListPIDIDAfterID(ctx context.Context, lastID uint, limit int) ([]*entity.LuoguQuestionBank, error)
}
