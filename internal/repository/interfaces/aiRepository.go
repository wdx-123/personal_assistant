package interfaces

import (
	"context"
	"time"

	"personal_assistant/internal/model/entity"
)

// AIRepository 定义当前领域访问持久化数据所需的仓储能力。
type AIRepository interface {
	CreateConversation(ctx context.Context, conversation *entity.AIConversation) error
	GetConversationByID(ctx context.Context, conversationID string) (*entity.AIConversation, error)
	GetConversationByIDForUpdate(ctx context.Context, conversationID string) (*entity.AIConversation, error)
	ListConversationsByUser(ctx context.Context, userID uint) ([]*entity.AIConversation, error)
	UpdateConversation(ctx context.Context, conversation *entity.AIConversation) error
	DeleteConversationCascade(ctx context.Context, conversationID string) error

	CreateMessage(ctx context.Context, message *entity.AIMessage) error
	UpdateMessage(ctx context.Context, message *entity.AIMessage) error
	ListMessagesByConversation(ctx context.Context, conversationID string) ([]*entity.AIMessage, error)

	CreateInterrupt(ctx context.Context, interrupt *entity.AIInterrupt) error
	GetInterruptByID(ctx context.Context, interruptID string) (*entity.AIInterrupt, error)
	GetInterruptByIDForUpdate(ctx context.Context, interruptID string) (*entity.AIInterrupt, error)
	ListInterruptsByUserAndStatuses(ctx context.Context, userID uint, statuses []string) ([]*entity.AIInterrupt, error)
	ListInterruptsForRecovery(
		ctx context.Context,
		statuses []string,
		updatedBefore time.Time,
		limit int,
	) ([]*entity.AIInterrupt, error)
	UpdateInterrupt(ctx context.Context, interrupt *entity.AIInterrupt) error

	WithTx(tx any) AIRepository
}
