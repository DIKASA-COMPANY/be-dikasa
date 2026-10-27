package registration

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// RegisterRoutes mendaftarkan endpoint registrasi publik ke Huma.
func RegisterRoutes(api huma.API, handler *Handler) {
	huma.Register(api, huma.Operation{
		OperationID: "register", Method: http.MethodPost, Path: "/register", Summary: "Mendaftarkan usaha dan akun pemilik",
		Description: "Membuat outlet dan akun nonaktif, lalu mengirim OTP lima digit. MAIL_MODE=disabled mengembalikan 503 sebelum menyimpan akun.",
		Tags:        []string{"Registrasi"}, Security: []map[string][]string{}, MaxBodyBytes: 1 << 20,
		Errors: []int{400, 409, 415, 422, 500, 502, 503},
	}, handler.Register)
	huma.Register(api, huma.Operation{
		OperationID: "confirm-registration-otp", Method: http.MethodPost, Path: "/register/otp/confirm", Summary: "Mengonfirmasi OTP registrasi",
		Description: "Memverifikasi OTP sekali pakai lalu mengaktifkan akun pemilik dan outlet.",
		Tags:        []string{"Registrasi"}, Security: []map[string][]string{}, MaxBodyBytes: 1 << 20,
		Errors: []int{400, 409, 410, 415, 422, 429, 500, 503},
	}, handler.ConfirmOTP)
	huma.Register(api, huma.Operation{
		OperationID: "resend-registration-otp", Method: http.MethodPost, Path: "/register/otp/resend", Summary: "Mengirim ulang OTP registrasi",
		Description: "Mengganti OTP sebelumnya dan mengirim kode baru setelah cooldown selesai.",
		Tags:        []string{"Registrasi"}, Security: []map[string][]string{}, MaxBodyBytes: 1 << 20,
		Errors: []int{400, 404, 409, 415, 422, 429, 500, 502, 503},
	}, handler.ResendOTP)
}
