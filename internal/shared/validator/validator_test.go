package validator

import "testing"

type loginPayload struct {
	Identifier string `json:"identifier" validate:"required,max=100"`
	Password   string `json:"password" validate:"required,min=8,max=128"`
}

func TestValidateReturnsIndonesianDetailsUsingJSONNames(t *testing.T) {
	validate := New()

	details := validate.Validate(loginPayload{Identifier: "", Password: "pendek"})

	if details["identifier"] != "Identifier wajib diisi." {
		t.Fatalf("unexpected identifier message: %q", details["identifier"])
	}
	if details["password"] != "Password minimal 8 karakter." {
		t.Fatalf("unexpected password message: %q", details["password"])
	}
}

func TestValidateAcceptsValidStruct(t *testing.T) {
	validate := New()
	if details := validate.Validate(loginPayload{Identifier: "kasir", Password: "rahasia123"}); len(details) != 0 {
		t.Fatalf("expected no validation errors, got %+v", details)
	}
}
