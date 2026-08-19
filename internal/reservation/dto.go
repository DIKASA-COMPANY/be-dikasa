package reservation

import "time"

// TypeData adalah item dropdown jenis reservasi.
type TypeData struct {
	ID        string `json:"id" format:"uuid"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

// TableData adalah representasi meja restoran.
type TableData struct {
	ID        string `json:"id" format:"uuid"`
	Name      string `json:"name"`
	Capacity  int    `json:"capacity" minimum:"1"`
	SortOrder int    `json:"sort_order"`
	IsActive  bool   `json:"is_active"`
}

// ReservationData adalah representasi reservasi untuk client.
type ReservationData struct {
	ID               string    `json:"id" format:"uuid"`
	CustomerName     string    `json:"customer_name"`
	CustomerContact  string    `json:"customer_contact"`
	ReservedAt       time.Time `json:"reserved_at" format:"date-time"`
	Notes            string    `json:"notes"`
	PrintReservation bool      `json:"print_reservation"`
	SendReservation  bool      `json:"send_reservation"`
	Status           string    `json:"status" enum:"booked,confirmed,seated,completed,cancelled"`
	ReservationType  TypeData  `json:"reservation_type"`
	RestaurantTable  TableData `json:"restaurant_table"`
	CreatedBy        string    `json:"created_by" format:"uuid"`
	CreatedAt        time.Time `json:"created_at" format:"date-time"`
	UpdatedAt        time.Time `json:"updated_at" format:"date-time"`
}

// ListTablesInput berisi filter waktu ketersediaan meja.
type ListTablesInput struct {
	ReservationAt string `query:"reservation_at"`
}

// TableRequest adalah payload konfigurasi meja.
type TableRequest struct {
	Name      string `json:"name" validate:"required,max=100"`
	Capacity  int    `json:"capacity" validate:"required,min=1"`
	SortOrder int    `json:"sort_order"`
	IsActive  bool   `json:"is_active"`
}

// TableCreateInput adalah request pembuatan meja.
type TableCreateInput struct{ Body TableRequest }

// TableMutationInput adalah request perubahan meja.
type TableMutationInput struct {
	TableID string `path:"table_id" format:"uuid"`
	Body    TableRequest
}

// TableDeleteInput adalah path soft delete meja.
type TableDeleteInput struct {
	TableID string `path:"table_id" format:"uuid"`
}

// ReservationRequest adalah payload pembuatan reservasi.
type ReservationRequest struct {
	CustomerName      string    `json:"customer_name" validate:"required,min=2,max=100"`
	CustomerContact   string    `json:"customer_contact" validate:"required,min=8,max=30"`
	ReservationTypeID string    `json:"reservation_type_id" validate:"required,uuid" format:"uuid"`
	RestaurantTableID string    `json:"restaurant_table_id" validate:"required,uuid" format:"uuid"`
	ReservedAt        time.Time `json:"reserved_at" validate:"required" format:"date-time"`
	Notes             string    `json:"notes" maxLength:"1000"`
	PrintReservation  bool      `json:"print_reservation"`
	SendReservation   bool      `json:"send_reservation"`
}

// ReservationPatchRequest adalah payload parsial perubahan reservasi.
type ReservationPatchRequest struct {
	CustomerName      *string    `json:"customer_name,omitempty"`
	CustomerContact   *string    `json:"customer_contact,omitempty"`
	ReservationTypeID *string    `json:"reservation_type_id,omitempty" format:"uuid"`
	RestaurantTableID *string    `json:"restaurant_table_id,omitempty" format:"uuid"`
	ReservedAt        *time.Time `json:"reserved_at,omitempty" format:"date-time"`
	Notes             *string    `json:"notes,omitempty" maxLength:"1000"`
	PrintReservation  *bool      `json:"print_reservation,omitempty"`
	SendReservation   *bool      `json:"send_reservation,omitempty"`
	Status            *string    `json:"status,omitempty" enum:"booked,confirmed,seated,completed,cancelled"`
}

// ReservationCreateInput adalah request pembuatan reservasi.
type ReservationCreateInput struct{ Body ReservationRequest }

// ReservationMutationInput adalah request perubahan reservasi.
type ReservationMutationInput struct {
	ReservationID string `path:"reservation_id" format:"uuid"`
	Body          ReservationPatchRequest
}

// ReservationIDInput adalah path detail atau pembatalan reservasi.
type ReservationIDInput struct {
	ReservationID string `path:"reservation_id" format:"uuid"`
}

// ListReservationsInput berisi filter dan pagination reservasi.
type ListReservationsInput struct {
	Date     string `query:"date" pattern:"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"`
	Status   string `query:"status" enum:"booked,confirmed,seated,completed,cancelled"`
	Page     int    `query:"page" default:"1" minimum:"1"`
	PageSize int    `query:"page_size" default:"30" minimum:"1" maximum:"100"`
}

// TypesOutput membungkus daftar jenis reservasi.
type TypesOutput struct {
	Body struct {
		Data []TypeData `json:"data"`
	}
}

// TablesOutput membungkus daftar meja.
type TablesOutput struct {
	Body struct {
		Data []TableData `json:"data"`
	}
}

// TableOutput membungkus satu meja.
type TableOutput struct {
	Status int
	Body   struct {
		Data TableData `json:"data"`
	}
}

// ReservationOutput membungkus satu reservasi.
type ReservationOutput struct {
	Status int
	Body   struct {
		Data ReservationData `json:"data"`
	}
}

// ReservationsOutput membungkus halaman reservasi.
type ReservationsOutput struct {
	Body struct {
		Data []ReservationData `json:"data"`
		Meta struct {
			Page     int   `json:"page"`
			PageSize int   `json:"page_size"`
			Total    int64 `json:"total"`
		} `json:"meta"`
	}
}

// NoContentOutput menghasilkan response tanpa body.
type NoContentOutput struct{ Status int }
