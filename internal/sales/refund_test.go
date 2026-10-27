package sales

import "testing"

func TestPartialRefundStatusAndAmounts(t *testing.T) {
	if !validOrderStatus("partially_refunded") {
		t.Fatal("partial filter rejected")
	}
	amount := int64(100)
	net := int64(200)
	d := orderData(Order{TotalAmount: 300, RefundedAmount: &amount, NetAfterRefund: &net, RefundsSupported: true})
	if d.TotalAmount != 300 || *d.RefundedAmount != 100 || *d.NetAfterRefund != 200 {
		t.Fatal(d)
	}
}
