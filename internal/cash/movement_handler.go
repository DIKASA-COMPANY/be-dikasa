package cash

import (
	shared "be-dikasa/internal/shared/errors"
	"be-dikasa/internal/shared/imagefile"
	"context"
	"errors"
	"github.com/danielgtaylor/huma/v2"
)

// MovementHandler serves authenticated private cash activity and commands.
type MovementHandler struct{ s *MovementService }

// NewMovementHandler uses the existing authenticated identity context.
func NewMovementHandler(s *MovementService) *MovementHandler { return &MovementHandler{s} }

// MovementActivityInput bounds activity pages in one explicit session.
type MovementActivityInput struct {
	ID       string `path:"session_id" format:"uuid"`
	Page     int    `query:"page" default:"1" minimum:"1" maximum:"1000000"`
	PageSize int    `query:"page_size" default:"20" minimum:"1" maximum:"100"`
}

// MovementIDInput identifies one scoped manual entry.
type MovementIDInput struct {
	ID         string `path:"session_id" format:"uuid"`
	MovementID string `path:"movement_id" format:"uuid"`
}

// MovementRequestInput resolves an original command's actor-bound request ID.
type MovementRequestInput struct {
	ID        string `path:"session_id" format:"uuid"`
	RequestID string `path:"request_id" format:"uuid"`
}

// MovementForm includes no client-authoritative actor, outlet or time.
type MovementForm struct {
	RequestID string        `form:"request_id" required:"true" format:"uuid"`
	Kind      string        `form:"kind" required:"true" enum:"in,out"`
	Name      string        `form:"name" required:"true" minLength:"1" maxLength:"120"`
	Amount    int64         `form:"amount" required:"true" minimum:"1" maximum:"9007199254740991"`
	Reason    string        `form:"reason" required:"true" minLength:"1" maxLength:"500"`
	Proof     huma.FormFile `form:"proof" required:"false" contentType:"image/png,image/jpeg"`
}

// MovementCreateInput is a bounded multipart request with optional evidence.
type MovementCreateInput struct {
	ID      string `path:"session_id" format:"uuid"`
	RawBody huma.MultipartFormFiles[MovementForm]
}

// MovementReverseInput allows a reason and a new request UUID, never a changed original amount.
type MovementReverseInput struct {
	ID         string `path:"session_id" format:"uuid"`
	MovementID string `path:"movement_id" format:"uuid"`
	Body       ReversalCommand
}

// MovementOutput contains only safe immutable event data.
type MovementOutput struct {
	Body struct {
		Data MovementRecord `json:"data"`
	}
}

// MovementDetailOutput adds read-time eligibility and reversal linkage.
type MovementDetailOutput struct {
	Body struct {
		Data MovementDetail `json:"data"`
	}
}

// MovementActivityOutput preserves full-session totals outside pagination.
type MovementActivityOutput struct{ Body ActivityPage }

// MovementProofOutput returns the authorized Supabase Storage public URL.
type MovementProofOutput struct {
	Body struct {
		Data ProofLink `json:"data"`
	}
}

// movementError exposes business decisions without internal storage or SQL detail.
func movementError(e error) error {
	switch {
	case errors.Is(e, ErrMovementInvalid):
		return shared.New(422, "CASH_MOVEMENT_INVALID", "Periksa nama, nominal Rupiah bulat positif, jenis, dan alasan transaksi kas.", nil)
	case errors.Is(e, ErrMovementInsufficient):
		return shared.New(409, "CASH_MOVEMENT_INSUFFICIENT_CASH", "Nominal keluar melebihi kas yang diharapkan. Muat ulang rincian kas.", nil)
	case errors.Is(e, ErrMovementReversed):
		return shared.New(409, "CASH_MOVEMENT_ALREADY_REVERSED", "Entri sudah dikoreksi atau merupakan entri koreksi.", nil)
	case errors.Is(e, ErrMovementRequestUsed):
		return shared.New(409, "CASH_MOVEMENT_REQUEST_USED", "ID permintaan sudah dipakai untuk transaksi kas lain.", nil)
	case errors.Is(e, ErrCloseForbidden):
		return shared.New(403, "CASH_MOVEMENT_FORBIDDEN", "Hanya kasir pembuka atau manager yang berwenang.", nil)
	case errors.Is(e, ErrProofDisabled):
		return shared.New(503, "CASH_PROOF_DISABLED", "Penyimpanan bukti kas belum tersedia.", nil)
	case errors.Is(e, ErrProofInvalid):
		return shared.New(422, "CASH_PROOF_INVALID", "Bukti harus PNG/JPEG valid maksimal 4096 px per sisi dan16 MP.", nil)
	case errors.Is(e, ErrProofTooLarge):
		return shared.New(413, "CASH_PROOF_TOO_LARGE", "Ukuran bukti maksimal3 MiB.", nil)
	case errors.Is(e, ErrProofStorage):
		return shared.New(502, "CASH_PROOF_STORAGE", "Bukti kas belum dapat diakses.", nil)
	}
	return closingError(e)
}

// Activity returns a session's actual cash sources and full totals.
func (h *MovementHandler) Activity(ctx context.Context, in *MovementActivityInput) (*MovementActivityOutput, error) {
	a, e := closingIdentity(ctx)
	if e != nil {
		return nil, e
	}
	v, e := h.s.Activity(ctx, a, in.ID, in.Page, in.PageSize)
	if e != nil {
		return nil, movementError(e)
	}
	return &MovementActivityOutput{v}, nil
}

// Create saves or replays one immutable command.
func (h *MovementHandler) Create(ctx context.Context, in *MovementCreateInput) (*MovementOutput, error) {
	a, e := closingIdentity(ctx)
	if e != nil {
		return nil, e
	}
	f := in.RawBody.Data()
	c := MovementCommand{RequestID: f.RequestID, Kind: f.Kind, Name: f.Name, Amount: f.Amount, Reason: f.Reason}
	if f.Proof.IsSet {
		v, e := imagefile.Validate(imagefile.Upload{Reader: f.Proof.File, Size: f.Proof.Size, Filename: f.Proof.Filename, ContentType: f.Proof.ContentType}, imagefile.StandardPolicy)
		if errors.Is(e, imagefile.ErrTooLarge) {
			return nil, movementError(ErrProofTooLarge)
		}
		if e != nil {
			return nil, movementError(ErrProofInvalid)
		}
		c.Proof = &v
	}
	m, e := h.s.Create(ctx, a, in.ID, c)
	if e != nil {
		return nil, movementError(e)
	}
	o := &MovementOutput{}
	o.Body.Data = m
	return o, nil
}

// Reverse records a linked opposite entry with its own reason and identity.
func (h *MovementHandler) Reverse(ctx context.Context, in *MovementReverseInput) (*MovementOutput, error) {
	a, e := closingIdentity(ctx)
	if e != nil {
		return nil, e
	}
	m, e := h.s.Reverse(ctx, a, in.ID, in.MovementID, in.Body)
	if e != nil {
		return nil, movementError(e)
	}
	o := &MovementOutput{}
	o.Body.Data = m
	return o, nil
}

// Detail checks current outlet and correction eligibility.
func (h *MovementHandler) Detail(ctx context.Context, in *MovementIDInput) (*MovementDetailOutput, error) {
	a, e := closingIdentity(ctx)
	if e != nil {
		return nil, e
	}
	v, e := h.s.Detail(ctx, a, in.ID, in.MovementID)
	if e != nil {
		return nil, movementError(e)
	}
	o := &MovementDetailOutput{}
	o.Body.Data = v
	return o, nil
}

// ByRequest recovers a successful command without targeting the current/new till.
func (h *MovementHandler) ByRequest(ctx context.Context, in *MovementRequestInput) (*MovementOutput, error) {
	a, e := closingIdentity(ctx)
	if e != nil {
		return nil, e
	}
	m, e := h.s.ByRequest(ctx, a, in.ID, in.RequestID)
	if e != nil {
		return nil, movementError(e)
	}
	o := &MovementOutput{}
	o.Body.Data = m
	return o, nil
}

// Proof authorizes the event before returning its Supabase Storage public URL.
func (h *MovementHandler) Proof(ctx context.Context, in *MovementIDInput) (*MovementProofOutput, error) {
	a, e := closingIdentity(ctx)
	if e != nil {
		return nil, e
	}
	v, e := h.s.Proof(ctx, a, in.ID, in.MovementID)
	if e != nil {
		return nil, movementError(e)
	}
	output := &MovementProofOutput{}
	output.Body.Data = v
	return output, nil
}
