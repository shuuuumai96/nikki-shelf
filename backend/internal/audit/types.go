package audit

const (
	OutcomeSucceeded = "succeeded"
	OutcomeFailed    = "failed"
)

type Event struct {
	ID            int64             `json:"id"`
	EventType     string            `json:"eventType"`
	Outcome       string            `json:"outcome"`
	ActorUserID   *int64            `json:"actorUserId,omitempty"`
	ActorUsername string            `json:"actorUsername,omitempty"`
	ActorRole     string            `json:"actorRole,omitempty"`
	TargetType    string            `json:"targetType,omitempty"`
	TargetID      string            `json:"targetId,omitempty"`
	ReasonKind    string            `json:"reasonKind,omitempty"`
	RequestID     string            `json:"requestId,omitempty"`
	RemoteIP      string            `json:"remoteIp,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	CreatedAt     string            `json:"createdAt"`
}

type ListResponse struct {
	Items []Event `json:"items"`
}
