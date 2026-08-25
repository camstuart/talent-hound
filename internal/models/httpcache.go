package models

import "time"

// HTTPCache is one provider answer remembered by its validator, so a repeat
// request can be conditional and cost no quota. It holds what the provider
// said about a public handle — nothing the recruiter typed.
type HTTPCache struct {
	URL       string    `gorm:"primarykey" json:"url"`
	ETag      string    `gorm:"not null" json:"etag"`
	Body      []byte    `gorm:"not null" json:"-"`
	FetchedAt time.Time `gorm:"not null" json:"fetchedAt"`
}

// TableName keeps the table's name from being pluralised into nonsense.
func (HTTPCache) TableName() string { return "http_cache" }
