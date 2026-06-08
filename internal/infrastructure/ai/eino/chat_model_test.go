package eino

import (
	"context"
	"testing"

	einomodel "github.com/cloudwego/eino/components/model"
)

func TestNewChatModelUsesCustomFactoryWhenProvided(t *testing.T) {
	expected := &fakeToolCallingChatModel{}

	model, err := NewChatModel(context.Background(), Options{
		ChatModelFactory: func(context.Context, Options) (einomodel.BaseChatModel, error) {
			return expected, nil
		},
	})
	if err != nil {
		t.Fatalf("NewChatModel() error = %v", err)
	}
	if model != expected {
		t.Fatalf("model = %#v, want %#v", model, expected)
	}
}

func TestNewChatModelRequiresModel(t *testing.T) {
	_, err := NewChatModel(context.Background(), Options{APIKey: "test"})
	if err == nil {
		t.Fatal("NewChatModel() error = nil, want missing model error")
	}
}
