package gateway

// Gateway is the interface all payment gateways must implement.
type Gateway interface {
InitiatePayment(req PaymentRequest) (*PaymentResponse, error)
VerifyPayment(ref string) (*PaymentStatus, error)
RefundPayment(ref string, amount int64) (*RefundResponse, error)
}

type PaymentRequest struct{}
type PaymentResponse struct{}
type PaymentStatus struct{}
type RefundResponse struct{}
