package domain

import "time"

// AssistantConversation belongs to the operator who created it.
type AssistantConversation struct {
	ID        string    `gorm:"primaryKey" json:"id" required:"true"`
	UserID    uint      `gorm:"not null;index" json:"-"`
	Title     string    `gorm:"not null" json:"title" required:"true"`
	CreatedAt time.Time `json:"createdAt" required:"true"`
	UpdatedAt time.Time `json:"updatedAt" required:"true"`
}

type AssistantMessage struct {
	ID             string                `gorm:"primaryKey" json:"id" required:"true"`
	ConversationID string                `gorm:"not null;index" json:"conversationId" required:"true"`
	Conversation   AssistantConversation `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	Role           string                `gorm:"not null" json:"role" required:"true"`
	Content        string                `gorm:"not null" json:"content" required:"true"`
	Status         string                `gorm:"not null" json:"status" required:"true"`
	// History preserves complete model/tool turns without exposing framework types in the API.
	History   string    `json:"-"`
	CreatedAt time.Time `json:"createdAt" required:"true"`
}
