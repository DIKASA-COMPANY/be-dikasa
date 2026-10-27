package savedorder

// CreateInput wraps a new saved cart.
type CreateInput struct{ Body Mutation }

// UpdateInput identifies an existing draft and expected cart revision.
type UpdateInput struct {
	ID   string `path:"saved_order_id" format:"uuid"`
	Body Mutation
}

// DetailInput identifies a draft within authenticated outlet scope.
type DetailInput struct {
	ID string `path:"saved_order_id" format:"uuid"`
}

// DeleteInput carries the replay key and optimistic revision in query parameters.
type DeleteInput struct {
	ID               string `path:"saved_order_id" format:"uuid"`
	RequestID        string `query:"request_id" format:"uuid" required:"true"`
	ExpectedRevision int64  `query:"expected_revision" minimum:"1" required:"true"`
}

// ListInput restricts browser pagination and label search.
type ListInput struct {
	Search   string `query:"search" maxLength:"100"`
	Page     int    `query:"page" default:"1" minimum:"1" maximum:"1000000"`
	PageSize int    `query:"page_size" default:"20" minimum:"1" maximum:"100"`
}

// Output returns the confirmed immutable command snapshot or current draft.
type Output struct {
	Body struct {
		Data *Order `json:"data"`
	}
}

// ListOutput follows the existing POS list envelope.
type ListOutput struct {
	Body struct {
		Data []Order `json:"data"`
		Meta struct {
			Page     int   `json:"page"`
			PageSize int   `json:"page_size"`
			Total    int64 `json:"total"`
		} `json:"meta"`
	}
}
