package system

import (
	"context"
	"errors"

	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"

	"gorm.io/gorm"
)

// AIGormRepository 定义当前领域访问持久化数据所需的仓储能力。
type AIGormRepository struct {
	db *gorm.DB
}

// NewAIRepository 负责创建并返回当前对象所需的实例。
// 参数：
//   - db：调用方传入的目标对象或配置实例。
//
// 返回值：
//   - interfaces.AIRepository：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func NewAIRepository(db *gorm.DB) interfaces.AIRepository {
	return &AIGormRepository{db: db}
}

// WithTx 基于现有依赖绑定事务上下文并返回新的可复用实例。
// 参数：
//   - tx：当前事务对象或事务句柄。
//
// 返回值：
//   - interfaces.AIRepository：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *AIGormRepository) WithTx(tx any) interfaces.AIRepository {
	if transaction, ok := tx.(*gorm.DB); ok {
		return &AIGormRepository{db: transaction}
	}
	return r
}

// CreateConversation 负责创建当前场景对应的数据或对象。
// 参数：
//   - ctx：链路上下文，用于取消、超时控制和日志透传。
//   - conversation：调用方传入的目标对象或配置实例。
//
// 返回值：
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *AIGormRepository) CreateConversation(ctx context.Context, conversation *entity.AIConversation) error {
	return r.db.WithContext(ctx).Create(conversation).Error
}

// GetConversationByID 用于获取当前场景需要的对象或数据。
// 参数：
//   - ctx：链路上下文，用于取消、超时控制和日志透传。
//   - conversationID：目标会话 ID。
//
// 返回值：
//   - *entity.AIConversation：当前函数返回的目标对象；失败时可能为 nil。
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *AIGormRepository) GetConversationByID(ctx context.Context, conversationID string) (*entity.AIConversation, error) {
	var conversation entity.AIConversation
	if err := r.db.WithContext(ctx).Where("id = ?", conversationID).First(&conversation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &conversation, nil
}

// ListConversationsByUser 用于查询并返回一组结果。
// 参数：
//   - ctx：链路上下文，用于取消、超时控制和日志透传。
//   - userID：当前用户 ID。
//
// 返回值：
//   - []*entity.AIConversation：当前函数返回的结果集合。
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *AIGormRepository) ListConversationsByUser(ctx context.Context, userID uint) ([]*entity.AIConversation, error) {
	var conversations []*entity.AIConversation
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("COALESCE(last_message_at, updated_at) DESC").
		Order("updated_at DESC").
		Find(&conversations).Error; err != nil {
		return nil, err
	}
	return conversations, nil
}

// UpdateConversation 负责更新当前场景对应的数据状态。
// 参数：
//   - ctx：链路上下文，用于取消、超时控制和日志透传。
//   - conversation：调用方传入的目标对象或配置实例。
//
// 返回值：
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *AIGormRepository) UpdateConversation(ctx context.Context, conversation *entity.AIConversation) error {
	return r.db.WithContext(ctx).Save(conversation).Error
}

// DeleteConversationCascade 负责删除当前场景对应的数据。
// 参数：
//   - ctx：链路上下文，用于取消、超时控制和日志透传。
//   - conversationID：目标会话 ID。
//
// 返回值：
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *AIGormRepository) DeleteConversationCascade(ctx context.Context, conversationID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("conversation_id = ?", conversationID).Delete(&entity.AIMessage{}).Error; err != nil {
			return err
		}
		if err := tx.Where("conversation_id = ?", conversationID).Delete(&entity.AIInterrupt{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", conversationID).Delete(&entity.AIConversation{}).Error
	})
}

// CreateMessage 负责创建当前场景对应的数据或对象。
// 参数：
//   - ctx：链路上下文，用于取消、超时控制和日志透传。
//   - message：调用方传入的目标对象或配置实例。
//
// 返回值：
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *AIGormRepository) CreateMessage(ctx context.Context, message *entity.AIMessage) error {
	return r.db.WithContext(ctx).Create(message).Error
}

// UpdateMessage 负责更新当前场景对应的数据状态。
// 参数：
//   - ctx：链路上下文，用于取消、超时控制和日志透传。
//   - message：调用方传入的目标对象或配置实例。
//
// 返回值：
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *AIGormRepository) UpdateMessage(ctx context.Context, message *entity.AIMessage) error {
	return r.db.WithContext(ctx).Save(message).Error
}

// ListMessagesByConversation 用于查询并返回一组结果。
// 参数：
//   - ctx：链路上下文，用于取消、超时控制和日志透传。
//   - conversationID：目标会话 ID。
//
// 返回值：
//   - []*entity.AIMessage：当前函数返回的结果集合。
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *AIGormRepository) ListMessagesByConversation(ctx context.Context, conversationID string) ([]*entity.AIMessage, error) {
	var messages []*entity.AIMessage
	if err := r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Order("created_at ASC").
		Find(&messages).Error; err != nil {
		return nil, err
	}
	return messages, nil
}

// CreateInterrupt 负责创建当前场景对应的数据或对象。
// 参数：
//   - ctx：链路上下文，用于取消、超时控制和日志透传。
//   - interrupt：调用方传入的目标对象或配置实例。
//
// 返回值：
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *AIGormRepository) CreateInterrupt(ctx context.Context, interrupt *entity.AIInterrupt) error {
	return r.db.WithContext(ctx).Create(interrupt).Error
}

// GetInterruptByID 用于获取当前场景需要的对象或数据。
// 参数：
//   - ctx：链路上下文，用于取消、超时控制和日志透传。
//   - interruptID：目标 interrupt ID。
//
// 返回值：
//   - *entity.AIInterrupt：当前函数返回的目标对象；失败时可能为 nil。
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *AIGormRepository) GetInterruptByID(ctx context.Context, interruptID string) (*entity.AIInterrupt, error) {
	var interrupt entity.AIInterrupt
	if err := r.db.WithContext(ctx).Where("interrupt_id = ?", interruptID).First(&interrupt).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &interrupt, nil
}

// UpdateInterrupt 负责更新当前场景对应的数据状态。
// 参数：
//   - ctx：链路上下文，用于取消、超时控制和日志透传。
//   - interrupt：调用方传入的目标对象或配置实例。
//
// 返回值：
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *AIGormRepository) UpdateInterrupt(ctx context.Context, interrupt *entity.AIInterrupt) error {
	return r.db.WithContext(ctx).Save(interrupt).Error
}
