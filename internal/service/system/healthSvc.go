package system

import (
	"context"

	"personal_assistant/global"
	"personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/repository"
)

// HealthService 提供服务健康检查能力
type HealthService struct {
	repositoryGroup *repository.Group
}

// NewHealthService 创建健康服务
func NewHealthService(repositoryGroup *repository.Group) *HealthService {
	return &HealthService{repositoryGroup: repositoryGroup}
}

// Health 返回服务健康状态
func (s *HealthService) Health(ctx context.Context) (*response.HealthResponse, error) {
	res := &response.HealthResponse{
		Status: "UP",
		DB:     "DOWN",
		Redis:  "DOWN",
	}

	// Check DB
	if s.repositoryGroup != nil && s.repositoryGroup.Ping(ctx) == nil {
		res.DB = "UP"
	}

	// Check Redis
	if global.Redis != nil {
		if global.Redis.Ping(ctx).Err() == nil {
			res.Redis = "UP"
		}
	}

	if res.DB == "DOWN" || res.Redis == "DOWN" {
		res.Status = "DOWN"
	}

	return res, nil
}

// Ping 返回轻量探活结果
func (s *HealthService) Ping(ctx context.Context) (*response.PingResponse, error) {
	return &response.PingResponse{
		Message: "pong",
	}, nil
}
