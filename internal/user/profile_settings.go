package user

import (
	"context"
	"errors"
	"strings"
	"time"
)

type genderRepository interface {
	ListGenders(context.Context) ([]Gender, error)
}
type atomicSettingsRepository interface {
	PatchProfile(context.Context, *User, ProfileMutation, *string) (*User, error)
	ReplaceCredential(context.Context, string, string, *string, string) error
}

// ListGenders returns database reference choices without hard-coded identifiers.
func (s *Service) ListGenders(ctx context.Context) ([]Gender, error) {
	if r, ok := s.repository.(genderRepository); ok {
		return r.ListGenders(ctx)
	}
	return nil, errors.New("gender repository unavailable")
}

// applyPersonalDetails validates optional supplied fields without changing omitted fields.
func (s *Service) applyPersonalDetails(ctx context.Context, a *User, m ProfileMutation) error {
	if m.Fields["date_of_birth"] {
		a.DateOfBirth = nil
		if v := strings.TrimSpace(m.DateOfBirth); v != "" {
			d, e := time.Parse("2006-01-02", v)
			loc, _ := time.LoadLocation("Asia/Jakarta")
			today := s.now().In(loc).Format("2006-01-02")
			if e != nil || v > today {
				return ErrInvalidProfile
			}
			a.DateOfBirth = &d
		}
	}
	if m.Fields["personal_address"] {
		a.PersonalAddress = nil
		v := strings.TrimSpace(m.PersonalAddress)
		if len([]rune(v)) > 1000 {
			return ErrInvalidProfile
		}
		if v != "" {
			a.PersonalAddress = &v
		}
	}
	if m.Fields["gender_id"] {
		a.GenderID = nil
		a.Gender = nil
		v := strings.TrimSpace(m.GenderID)
		if v != "" {
			choices, e := s.ListGenders(ctx)
			if e != nil {
				return e
			}
			found := false
			for _, g := range choices {
				if g.ID == v {
					value := g
					a.GenderID = &v
					a.Gender = &value
					found = true
					break
				}
			}
			if !found {
				return ErrInvalidProfile
			}
		}
	}
	return nil
}
