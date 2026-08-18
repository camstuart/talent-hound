package models

import (
	"strings"
	"time"
)

// Candidate is a person in the recruiter's talent pool. It is a shared record:
// several initiatives may reference it and none of them owns it.
//
// The fields here are exactly the structured candidate fields the PRD names.
// Employment history, education, skills, achievements, and qualifications are
// deliberately absent — they become evidence-backed profile aspects later.
type Candidate struct {
	ID                     uint       `gorm:"primarykey" json:"id"`
	FullName               string     `gorm:"not null" json:"fullName"`
	PreferredName          string     `json:"preferredName"`
	Emails                 StringList `json:"emails"`
	Phones                 StringList `json:"phones"`
	Location               string     `json:"location"`
	WorkRights             string     `json:"workRights"`
	Availability           Date       `json:"availability"`
	DesiredEmploymentType  string     `json:"desiredEmploymentType"`
	DesiredWorkArrangement string     `json:"desiredWorkArrangement"`
	// Compensation or rate expectations.
	Compensation Compensation `gorm:"embedded;embeddedPrefix:comp_" json:"compensation"`
	// Where these facts came from, or whose authority they carry.
	SourceNote    string    `json:"sourceNote"`
	LastConfirmed Date      `json:"lastConfirmed"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// Validate normalises c in place and reports the first problem found.
func (c *Candidate) Validate() error {
	var err error
	if c.FullName, err = requireText("candidate full name", c.FullName); err != nil {
		return err
	}
	c.PreferredName = strings.TrimSpace(c.PreferredName)
	c.Location = strings.TrimSpace(c.Location)
	c.WorkRights = strings.TrimSpace(c.WorkRights)
	c.DesiredEmploymentType = strings.TrimSpace(c.DesiredEmploymentType)
	c.DesiredWorkArrangement = strings.TrimSpace(c.DesiredWorkArrangement)
	c.SourceNote = strings.TrimSpace(c.SourceNote)
	c.Emails = c.Emails.Clean()
	c.Phones = c.Phones.Clean()
	if err := c.Availability.Validate("candidate availability"); err != nil {
		return err
	}
	if err := c.LastConfirmed.Validate("candidate last-confirmed date"); err != nil {
		return err
	}
	return c.Compensation.Validate("candidate compensation")
}
