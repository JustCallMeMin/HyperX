package event

type PaymentPayload interface {
	Payload
	GetAmountCents() int64
	GetCurrency() string
	GetProvider() string
	GetProviderEventID() string
}
