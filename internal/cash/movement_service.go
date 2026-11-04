package cash

import (
	"be-dikasa/internal/shared/supabasestorage"
	"be-dikasa/internal/user"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/google/uuid"
	"strings"
	"time"
)

// MovementCommand is one immutable drawer intent plus optional validated original proof.
type MovementCommand struct {
	RequestID, Kind, Name, Reason string
	Amount                        int64
	ProofObjectName               string
}

// ReversalCommand links one correction intent to an original movement.
type ReversalCommand struct {
	RequestID string `json:"request_id" format:"uuid"`
	Reason    string `json:"reason" minLength:"1" maxLength:"500"`
}

// MovementService coordinates authenticated intents with durable session locks.
type MovementService struct {
	repo     *MovementRepository
	accounts ReconciliationAccounts
	proof    MovementProofStore
	now      func() time.Time
}

// NewMovementService configures private proof storage without requiring it for text entries.
func NewMovementService(r *MovementRepository, a ReconciliationAccounts, p MovementProofStore, clock func() time.Time) *MovementService {
	if clock == nil {
		clock = time.Now
	}
	return &MovementService{r, a, p, clock}
}
func (s *MovementService) account(ctx context.Context, id string) (*user.User, error) {
	a, e := s.accounts.FindByIDWithRole(ctx, id)
	if e != nil || a == nil || !a.IsActive || a.OutletID == nil || (a.Role.Name != "cashier" && a.Role.Name != "manager") {
		return nil, ErrCloseForbidden
	}
	return a, nil
}
func movementUUID(v string) (string, error) {
	u, e := uuid.Parse(v)
	if e != nil || u == uuid.Nil {
		return "", ErrMovementInvalid
	}
	return u.String(), nil
}

// Create records one cash addition or withdrawal, or replays its committed result.
func (s *MovementService) Create(ctx context.Context, actor, sid string, c MovementCommand) (MovementRecord, error) {
	return s.command(ctx, actor, sid, "", c)
}

// PrepareProofUpload authorizes one session request before issuing a direct Supabase upload URL.
func (s *MovementService) PrepareProofUpload(ctx context.Context, actor, sid, requestID, contentType string, size int64) (supabasestorage.SignedUpload, error) {
	if s.proof == nil {
		return supabasestorage.SignedUpload{}, ErrProofDisabled
	}
	a, err := s.account(ctx, actor)
	if err != nil {
		return supabasestorage.SignedUpload{}, err
	}
	sid, err = movementUUID(sid)
	if err != nil {
		return supabasestorage.SignedUpload{}, err
	}
	requestID, err = movementUUID(requestID)
	if err != nil {
		return supabasestorage.SignedUpload{}, err
	}
	activity, err := s.repo.activity(ctx, a, sid, 1, 1, true)
	if err != nil {
		return supabasestorage.SignedUpload{}, err
	}
	if !activity.Capabilities.CanUploadProof {
		return supabasestorage.SignedUpload{}, ErrCloseForbidden
	}
	return s.proof.Prepare(ctx, *a.OutletID, sid, a.ID, requestID, contentType, size)
}

// Reverse records exactly one opposite entry; the original remains unchanged.
func (s *MovementService) Reverse(ctx context.Context, actor, sid, id string, c ReversalCommand) (MovementRecord, error) {
	v, e := movementUUID(id)
	if e != nil {
		return MovementRecord{}, e
	}
	return s.command(ctx, actor, sid, v, MovementCommand{RequestID: c.RequestID, Reason: c.Reason})
}

// command validates canonical intent before entering the lock/replay transaction.
func (s *MovementService) command(ctx context.Context, actor, sid, reverse string, c MovementCommand) (MovementRecord, error) {
	var empty MovementRecord
	a, e := s.account(ctx, actor)
	if e != nil {
		return empty, e
	}
	sid, e = movementUUID(sid)
	if e != nil {
		return empty, e
	}
	c.RequestID, e = movementUUID(c.RequestID)
	if e != nil {
		return empty, e
	}
	c.Name = strings.TrimSpace(c.Name)
	c.Reason = strings.TrimSpace(c.Reason)
	if len([]rune(c.Reason)) < 1 || len([]rune(c.Reason)) > 500 {
		return empty, ErrMovementInvalid
	}
	if reverse == "" && ((c.Kind != "in" && c.Kind != "out") || c.Amount < 1 || c.Amount > maxOpeningCash || len([]rune(c.Name)) < 1 || len([]rune(c.Name)) > 120) {
		return empty, ErrMovementInvalid
	}
	proofHash := strings.TrimSpace(c.ProofObjectName)
	if proofHash != "" {
		if s.proof == nil {
			return empty, ErrProofDisabled
		}
		c.ProofObjectName = proofHash
	}
	raw, _ := json.Marshal(struct {
		Actor, Outlet, Session, Reverse, Request, Kind, Name, Reason, Proof string
		Amount                                                              int64
	}{actor, *a.OutletID, sid, reverse, c.RequestID, c.Kind, c.Name, c.Reason, proofHash, c.Amount})
	h := sha256.Sum256(raw)
	return s.repo.execute(ctx, a.ID, *a.OutletID, sid, reverse, c, hex.EncodeToString(h[:]), s.proof, s.now)
}

// Detail returns outlet-scoped history with current correction eligibility.
func (s *MovementService) Detail(ctx context.Context, actor, sid, id string) (MovementDetail, error) {
	a, e := s.account(ctx, actor)
	if e != nil {
		return MovementDetail{}, e
	}
	if sid, e = movementUUID(sid); e != nil {
		return MovementDetail{}, e
	}
	if id, e = movementUUID(id); e != nil {
		return MovementDetail{}, e
	}
	return s.repo.detail(ctx, a, sid, id)
}

// ByRequest resolves only the authenticated actor's original command in its original session.
func (s *MovementService) ByRequest(ctx context.Context, actor, sid, id string) (MovementRecord, error) {
	a, e := s.account(ctx, actor)
	if e != nil {
		return MovementRecord{}, e
	}
	if sid, e = movementUUID(sid); e != nil {
		return MovementRecord{}, e
	}
	if id, e = movementUUID(id); e != nil {
		return MovementRecord{}, e
	}
	return s.repo.byRequest(ctx, *a.OutletID, a.ID, sid, id)
}

// Activity reads a consistent session projection; pagination never changes card totals.
func (s *MovementService) Activity(ctx context.Context, actor, sid string, page, size int) (ActivityPage, error) {
	a, e := s.account(ctx, actor)
	if e != nil {
		return ActivityPage{}, e
	}
	if sid, e = movementUUID(sid); e != nil || page < 1 || page > 1000000 || size < 1 || size > 100 {
		return ActivityPage{}, ErrMovementInvalid
	}
	return s.repo.activity(ctx, a, sid, page, size, s.proof != nil)
}

// ProofLink contains an authorized Supabase Storage public URL.
type ProofLink struct {
	URL string `json:"url" format:"uri" doc:"Supabase Storage public URL for direct client rendering"`
}

// Proof checks outlet scope before returning the persisted Supabase URL.
func (s *MovementService) Proof(ctx context.Context, actor, sid, id string) (ProofLink, error) {
	d, e := s.Detail(ctx, actor, sid, id)
	if e != nil {
		return ProofLink{}, e
	}
	if !d.Movement.HasProof {
		return ProofLink{}, ErrNotFound
	}
	if d.Movement.ProofURL == nil || *d.Movement.ProofURL == "" {
		return ProofLink{}, ErrProofStorage
	}
	return ProofLink{URL: *d.Movement.ProofURL}, nil
}
