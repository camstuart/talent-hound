package models

import (
	"errors"
	"strings"
	"time"
)

// Contact is a person at a company. The PoC's warm-path support is the count
// and listing of known contacts at a company — nothing more.
type Contact struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CompanyID uint      `gorm:"not null;index" json:"companyId"`
	FullName  string    `gorm:"not null" json:"fullName"`
	Title     string    `json:"title"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Validate normalises c in place and reports the first problem found. That the
// referenced company exists is the service's job, not the model's.
func (c *Contact) Validate() error {
	var err error
	if c.FullName, err = requireText("contact full name", c.FullName); err != nil {
		return err
	}
	if c.CompanyID == 0 {
		return errors.New("contact must belong to a company")
	}
	c.Title = strings.TrimSpace(c.Title)
	c.Email = strings.TrimSpace(c.Email)
	c.Phone = strings.TrimSpace(c.Phone)
	c.Source = strings.TrimSpace(c.Source)
	return nil
}
