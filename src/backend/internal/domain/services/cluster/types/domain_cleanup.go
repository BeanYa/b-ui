package clustertypes

type DomainCleanupPayload struct {
	RequestID string `json:"request_id"`
	DomainID  string `json:"domain_id"`
}
