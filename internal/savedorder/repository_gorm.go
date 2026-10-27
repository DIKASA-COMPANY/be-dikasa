package savedorder

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GORMRepository stores drafts and command results in PostgreSQL transactions.
type GORMRepository struct{ db *gorm.DB }

// NewGORMRepository uses the same database as receipt checkout.
func NewGORMRepository(db *gorm.DB) *GORMRepository { return &GORMRepository{db} }

// Get returns only the requested outlet's draft and its ordered snapshots.
func (r *GORMRepository) Get(ctx context.Context, outlet, id string) (*Order, error) {
	return readOrder(r.db.WithContext(ctx), outlet, id, false)
}

// readOrder always scopes the header before loading its lines; checkout retains its row lock.
func readOrder(db *gorm.DB, outlet, id string, lock bool) (*Order, error) {
	var o Order
	q := db.Where("saved_orders.id = ? AND saved_orders.outlet_id = ?", id, outlet)
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	e := q.Preload("Items", func(d *gorm.DB) *gorm.DB { return d.Order("position ASC") }).First(&o).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if e != nil {
		return nil, e
	}
	var name string
	db.Table("users").Select("TRIM(first_name || ' ' || last_name)").Where("id = ?", o.CreatedBy).Scan(&name)
	o.CreatorName = name
	o.Summarize()
	return &o, nil
}

// List returns stable, paginated open drafts with snapshots and totals.
func (r *GORMRepository) List(ctx context.Context, outlet, search string, page, size int) ([]Order, int64, error) {
	q := r.db.WithContext(ctx).Model(&Order{}).Where("outlet_id = ? AND status = 'open'", outlet)
	if search != "" {
		q = q.Where("label ILIKE ?", "%"+search+"%")
	}
	var n int64
	if e := q.Count(&n).Error; e != nil {
		return nil, 0, e
	}
	var rows []Order
	e := q.Preload("Items", func(d *gorm.DB) *gorm.DB { return d.Order("position ASC") }).Order("updated_at DESC, id ASC").Offset((page - 1) * size).Limit(size).Find(&rows).Error
	for i := range rows {
		rows[i].Summarize()
		r.db.WithContext(ctx).Table("users").Select("TRIM(first_name || ' ' || last_name)").Where("id = ?", rows[i].CreatedBy).Scan(&rows[i].CreatorName)
	}
	if rows == nil {
		rows = []Order{}
	}
	return rows, n, e
}

// Execute serializes matching retry keys, then locks drafts before changing them.
func (r *GORMRepository) Execute(ctx context.Context, outlet, actor, op, id string, m Mutation) (result *Order, err error) {
	outlet = canonicalID(outlet)
	actor = canonicalID(actor)
	if id != "" {
		id = canonicalID(id)
	}
	m.RequestID = canonicalID(m.RequestID)
	if m.OrderTypeID != "" {
		m.OrderTypeID = canonicalID(m.OrderTypeID)
	}
	m.Items = append([]Item(nil), m.Items...)
	// Remove transport-owned fields before hashing; IDs and positions are server-owned.
	for i := range m.Items {
		m.Items[i].ProductID = canonicalID(m.Items[i].ProductID)
		if m.Items[i].ProductVariantID != nil {
			v := canonicalID(*m.Items[i].ProductVariantID)
			m.Items[i].ProductVariantID = &v
		}
		m.Items[i].ID = ""
		m.Items[i].SavedOrderID = ""
		m.Items[i].OutletID = ""
		m.Items[i].Position = i
	}
	encoded, _ := json.Marshal(struct {
		Actor, Operation, ID string
		Mutation             Mutation
	}{actor, op, id, m})
	hash := sha256.Sum256(encoded)
	digest := hex.EncodeToString(hash[:])
	lockHash := sha256.Sum256([]byte(outlet + ":" + m.RequestID))
	key := int64(binary.BigEndian.Uint64(lockHash[:8]))
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := tx.Exec("SELECT pg_advisory_xact_lock(?)", key).Error; e != nil {
			return e
		}
		var previous struct {
			PayloadHash string
			Response    []byte
		}
		e := tx.Table("saved_order_commands").Where("outlet_id = ? AND request_id = ?", outlet, m.RequestID).Take(&previous).Error
		if e == nil {
			if previous.PayloadHash != digest {
				return ErrRequestUsed
			}
			var o Order
			if e = json.Unmarshal(previous.Response, &o); e != nil {
				return e
			}
			result = &o
			return nil
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		var o *Order
		if op == "create" {
			o = &Order{ID: uuid.NewString(), OutletID: outlet, CreatedBy: actor, Revision: 1, Status: "open"}
		} else {
			o, e = readOrder(tx, outlet, id, true)
			if e != nil {
				return e
			}
			if o.Status == "paid" {
				return ErrAlreadyPaid
			}
			if o.Status != "open" || o.Revision != m.ExpectedRevision {
				return ErrConflict
			}
			o.Revision++
		}
		if op == "delete" {
			o.Status = "deleted"
		} else {
			if e = validateReferences(tx, outlet, m); e != nil {
				return e
			}
			o.Label = strings.TrimSpace(m.Label)
			if o.Label == "" {
				o.Label = "Order " + o.ID[:8]
			}
			o.OrderTypeID = m.OrderTypeID
			o.Items = append([]Item(nil), m.Items...)
			for i := range o.Items {
				o.Items[i].ID = uuid.NewString()
				o.Items[i].SavedOrderID = o.ID
				o.Items[i].OutletID = outlet
				o.Items[i].Position = i
			}
		}
		o.UpdatedAt = time.Now().UTC()
		if op == "create" {
			o.CreatedAt = o.UpdatedAt
			e = tx.Omit("Items", "CreatorName").Create(o).Error
		} else {
			e = tx.Model(&Order{}).Where("id = ? AND outlet_id = ?", o.ID, outlet).Updates(map[string]any{"label": o.Label, "order_type_id": o.OrderTypeID, "revision": o.Revision, "status": o.Status, "updated_at": o.UpdatedAt}).Error
		}
		if e != nil {
			return e
		}
		if op != "delete" {
			if e = tx.Where("saved_order_id = ?", o.ID).Delete(&Item{}).Error; e != nil {
				return e
			}
			if e = tx.Create(&o.Items).Error; e != nil {
				return e
			}
		}
		if e = tx.Table("users").Select("TRIM(first_name || ' ' || last_name)").Where("id = ?", o.CreatedBy).Scan(&o.CreatorName).Error; e != nil {
			return e
		}
		o.Summarize()
		response, e := json.Marshal(o)
		if e != nil {
			return e
		}
		e = tx.Exec("INSERT INTO saved_order_commands(outlet_id,request_id,actor_id,operation,payload_hash,saved_order_id,response) VALUES (?,?,?,?,?,?,?::jsonb)", outlet, m.RequestID, actor, op, digest, o.ID, string(response)).Error
		if e != nil {
			return e
		}
		result = o
		return nil
	})
	return
}

// validateReferences deliberately permits inactive snapshots but never foreign-outlet items.
func validateReferences(tx *gorm.DB, outlet string, m Mutation) error {
	var n int64
	if e := tx.Table("order_types").Where("id = ? AND outlet_id = ?", m.OrderTypeID, outlet).Count(&n).Error; e != nil {
		return e
	}
	if n != 1 {
		return ErrValidation
	}
	for _, i := range m.Items {
		if e := tx.Table("products").Where("id = ? AND outlet_id = ?", i.ProductID, outlet).Count(&n).Error; e != nil {
			return e
		}
		if n != 1 {
			return ErrValidation
		}
		if i.ProductVariantID != nil {
			if e := tx.Table("product_variants").Where("id = ? AND product_id = ? AND outlet_id = ?", *i.ProductVariantID, i.ProductID, outlet).Count(&n).Error; e != nil {
				return e
			}
			if n != 1 {
				return ErrValidation
			}
		}
	}
	return nil
}
