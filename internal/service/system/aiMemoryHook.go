package system

import (
	"context"
	"time"

	"personal_assistant/global"
	aidomain "personal_assistant/internal/domain/ai"
	"personal_assistant/internal/model/entity"

	"go.uber.org/zap"
)

const aiMemoryWritebackTimeout = 10 * time.Second

func (s *AIService) triggerMemoryWriteback(
	ctx context.Context,
	conversation *entity.AIConversation,
	userMessage *entity.AIMessage,
	assistantMessage *entity.AIMessage,
	principal aidomain.AIToolPrincipal,
) {
	if s == nil || s.memoryWriteback == nil || !aiMemoryEnabled() ||
		conversation == nil || userMessage == nil || assistantMessage == nil {
		return
	}
	input := aiMemoryWritebackInput{
		ConversationID:     conversation.ID,
		UserID:             conversation.UserID,
		OrgID:              cloneMemoryUintPtr(conversation.OrgID),
		UserMessageID:      userMessage.ID,
		AssistantMessageID: assistantMessage.ID,
		Principal:          principal,
	}

	run := func(execCtx context.Context) {
		if err := s.memoryWriteback.OnTurnCompleted(execCtx, input); err != nil && global.Log != nil {
			global.Log.Error(
				"AI memory writeback failed",
				zap.String("conversation_id", conversation.ID),
				zap.String("user_message_id", userMessage.ID),
				zap.String("assistant_message_id", assistantMessage.ID),
				zap.Error(err),
			)
		}
	}

	if global.Config != nil && global.Config.AI.Memory.WritebackAsync {
		go func() {
			writeCtx, cancel := context.WithTimeout(context.Background(), aiMemoryWritebackTimeout)
			defer cancel()
			run(writeCtx)
		}()
		return
	}

	if ctx == nil {
		ctx = context.Background()
	}
	writeCtx, cancel := context.WithTimeout(ctx, aiMemoryWritebackTimeout)
	defer cancel()
	run(writeCtx)
}
