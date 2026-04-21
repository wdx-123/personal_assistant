package eino

import (
	"context"
	"testing"
)

func TestNewChatModelRequiresModel(t *testing.T) {
	_, err := NewChatModel(context.Background(), Options{APIKey: "test"})
	if err == nil {
		t.Fatal("NewChatModel() error = nil, want missing model error")
	}
}
