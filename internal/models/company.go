package models

import (
	"strings"
	"time"
)

// Company is the minimal PoC company record: enough to hang contacts off and to
// name a role's employer. Relationship strength and interaction history are P1.
type Company struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Name      string    `gorm:"not null" json:"name"`
	Website   string    `json:"website"`
	Location  string    `json:"location"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Validate normalises c in place and reports the first problem found.
func (c *Company) Validate() error {
	var err error
	if c.Name, err = requireText("company name", c.Name); err != nil {
		return err
	}
	if c.Website, err = requireAbsoluteURL("company website", c.Website); err != nil {
		return err
	}
	c.Location = strings.TrimSpace(c.Location)
	c.Source = strings.TrimSpace(c.Source)
	return nil
}
