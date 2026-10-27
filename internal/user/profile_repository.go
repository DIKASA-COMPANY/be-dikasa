package user

import (
	"context"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ListGenders returns ordered profile reference choices.
func (r *GORMRepository) ListGenders(ctx context.Context) ([]Gender, error) {
	rows := []Gender{}
	e := r.db.WithContext(ctx).Order("name,id").Find(&rows).Error
	return rows, e
}

// PatchProfile applies only supplied fields while serializing avatar replacement.
func (r *GORMRepository) PatchProfile(ctx context.Context, a *User, m ProfileMutation, old *string) (*User, error) {
	var result *User
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current User
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, "id = ?", a.ID).Error; e != nil {
			return e
		}
		changes := map[string]any{}
		set := func(k string) bool { return m.Fields == nil || m.Fields[k] }
		if set("first_name") {
			changes["first_name"] = a.FirstName
		}
		if set("last_name") {
			changes["last_name"] = a.LastName
		}
		if set("phone_number") {
			changes["phone_number"] = a.PhoneNumber
		}
		if set("email") {
			changes["email"] = a.Email
		}
		if m.Fields["date_of_birth"] {
			changes["date_of_birth"] = a.DateOfBirth
		}
		if m.Fields["gender_id"] {
			changes["gender_id"] = a.GenderID
		}
		if m.Fields["personal_address"] {
			changes["personal_address"] = a.PersonalAddress
		}
		if m.Avatar != nil || (set("remove_avatar") && m.RemoveAvatar) {
			same := (old == nil && current.AvatarURL == nil) || (old != nil && current.AvatarURL != nil && *old == *current.AvatarURL)
			if !same {
				return ErrConflict
			}
			changes["avatar_url"] = a.AvatarURL
		}
		if len(changes) > 0 {
			if e := normalizeUserWriteError(tx.Model(&User{}).Where("id = ?", a.ID).Updates(changes).Error); e != nil {
				return e
			}
		}
		var e error
		result, e = NewGORMRepository(tx).FindByID(ctx, a.ID)
		return e
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ReplaceCredential atomically rejects a stale previously verified credential.
func (r *GORMRepository) ReplaceCredential(ctx context.Context, id, column string, old *string, next string) error {
	if column != "pin_hash" && column != "password_hash" {
		return ErrInvalidProfile
	}
	q := r.db.WithContext(ctx).Model(&User{}).Where("id = ?", id)
	if old == nil {
		q = q.Where(column + " IS NULL")
	} else {
		q = q.Where(column+" = ?", *old)
	}
	result := q.Update(column, next)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrConflict
	}
	return nil
}
