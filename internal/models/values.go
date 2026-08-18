package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// dateLayout is the only date format accepted or stored. Calendar facts —
// availability, published, closing, retrieved, last confirmed — are days in the
// world, not instants in this database, so they never carry a time or a zone.
const dateLayout = "2006-01-02"

// Date is a calendar day in YYYY-MM-DD form. The empty string means "not
// stated", which is different from a zero date.
type Date string

// Validate reports whether d is empty or a real calendar day. field names the
// value in the returned error.
func (d Date) Validate(field string) error {
	if d == "" {
		return nil
	}
	if _, err := time.Parse(dateLayout, string(d)); err != nil {
		return fmt.Errorf("%s must be a date in YYYY-MM-DD form, got %q", field, string(d))
	}
	return nil
}

// Before reports whether both dates are stated and d falls before other.
func (d Date) Before(other Date) bool {
	if d == "" || other == "" {
		return false
	}
	return string(d) < string(other) // YYYY-MM-DD sorts lexically
}

// StringList is a list of short strings — email addresses, phone numbers —
// persisted as a JSON array in one TEXT column.
type StringList []string

// Scan implements sql.Scanner.
func (l *StringList) Scan(src any) error {
	var raw []byte
	switch v := src.(type) {
	case nil:
		*l = nil
		return nil
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into a string list", src)
	}
	if len(raw) == 0 {
		*l = nil
		return nil
	}
	return json.Unmarshal(raw, l)
}

// Value implements driver.Valuer. A nil list is stored as an empty array so the
// column never holds NULL.
func (l StringList) Value() (driver.Value, error) {
	if l == nil {
		l = StringList{}
	}
	b, err := json.Marshal([]string(l))
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Clean trims each entry and drops the empty ones, preserving order.
func (l StringList) Clean() StringList {
	out := make(StringList, 0, len(l))
	for _, s := range l {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// CompensationPeriod is the unit a compensation figure is quoted in.
type CompensationPeriod string

// The accepted compensation periods.
const (
	PeriodHour  CompensationPeriod = "hour"
	PeriodDay   CompensationPeriod = "day"
	PeriodWeek  CompensationPeriod = "week"
	PeriodMonth CompensationPeriod = "month"
	PeriodYear  CompensationPeriod = "year"
)

// Valid reports whether p is one of the accepted periods.
func (p CompensationPeriod) Valid() bool {
	switch p {
	case PeriodHour, PeriodDay, PeriodWeek, PeriodMonth, PeriodYear:
		return true
	}
	return false
}

// compMaxAmount is a typo guard, not a business rule: nobody quotes a rate of a
// billion, and a stray keypress otherwise persists silently.
const compMaxAmount = 1_000_000_000

// Compensation is a stated pay expectation or offer. Amounts are whole currency
// units — integers, because money in a float is a 3am page — and either bound
// may be absent. An entirely empty Compensation means "not stated".
type Compensation struct {
	Min      *int64             `json:"min"`
	Max      *int64             `json:"max"`
	Currency string             `json:"currency"`
	Period   CompensationPeriod `json:"period"`
}

// Stated reports whether any part of the compensation was given.
func (c Compensation) Stated() bool {
	return c.Min != nil || c.Max != nil || c.Currency != "" || c.Period != ""
}

// Validate checks a stated compensation. An unstated one is always valid.
func (c *Compensation) Validate(field string) error {
	c.Currency = strings.ToUpper(strings.TrimSpace(c.Currency))
	c.Period = CompensationPeriod(strings.ToLower(strings.TrimSpace(string(c.Period))))
	if !c.Stated() {
		return nil
	}
	for name, amount := range map[string]*int64{"minimum": c.Min, "maximum": c.Max} {
		if amount == nil {
			continue
		}
		if *amount < 0 {
			return fmt.Errorf("%s %s must not be negative", field, name)
		}
		if *amount > compMaxAmount {
			return fmt.Errorf("%s %s of %d looks like a typo", field, name, *amount)
		}
	}
	if c.Min != nil && c.Max != nil && *c.Min > *c.Max {
		return fmt.Errorf("%s minimum %d is greater than maximum %d", field, *c.Min, *c.Max)
	}
	// ISO 4217 alphabetic codes are three letters; the full code list is not
	// worth vendoring for a PoC that never converts currencies.
	if len(c.Currency) != 3 || strings.Trim(c.Currency, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") != "" {
		return fmt.Errorf("%s currency must be a three-letter ISO 4217 code, got %q", field, c.Currency)
	}
	if !c.Period.Valid() {
		return fmt.Errorf("%s period must be one of hour, day, week, month, year, got %q", field, c.Period)
	}
	return nil
}

// requireText trims v and rejects it when nothing is left, so a value of spaces
// fails like an empty one. Unicode content is otherwise untouched: normalising
// or folding a name is a data-quality bug the recruiter only spots in their own.
func requireText(field, v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", fmt.Errorf("%s must not be empty", field)
	}
	return v, nil
}

// requireAbsoluteURL trims v and, when anything is left, requires an absolute
// http or https URL. A bare domain is rejected rather than silently prefixed.
func requireAbsoluteURL(field, v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", nil
	}
	u, err := url.Parse(v)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("%s must be an absolute http or https URL, got %q", field, v)
	}
	return v, nil
}
