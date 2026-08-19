package cash

import "time"

// OpenRequest adalah payload kas awal dalam satuan Rupiah utuh.
type OpenRequest struct {
	OpeningCash int64 `json:"opening_cash" validate:"gte=0,lte=9007199254740991" minimum:"0" maximum:"9007199254740991" example:"500000"`
}

// OpenInput membungkus body endpoint pembukaan sesi kas.
type OpenInput struct{ Body OpenRequest }

// SessionData adalah data sesi kas yang aman dikirim ke client.
type SessionData struct {
	ID          string     `json:"id" format:"uuid"`
	OutletID    string     `json:"outlet_id" format:"uuid"`
	UserID      string     `json:"user_id" format:"uuid"`
	OpeningCash int64      `json:"opening_cash" example:"500000"`
	ClosingCash *int64     `json:"closing_cash,omitempty" example:"750000"`
	Status      string     `json:"status" enum:"open,closed" example:"open"`
	OpenedAt    time.Time  `json:"opened_at" format:"date-time"`
	ClosedAt    *time.Time `json:"closed_at,omitempty" format:"date-time"`
}

// SessionResponse adalah envelope response sesi kas.
type SessionResponse struct {
	Data SessionData `json:"data"`
}

// OpenOutput membungkus response 201 pembukaan sesi kas.
type OpenOutput struct {
	Status int
	Body   SessionResponse
}

// CurrentOutput membungkus response sesi kas aktif.
type CurrentOutput struct{ Body SessionResponse }
