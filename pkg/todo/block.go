package todo

type Block struct {
	ID          string `json:"id"`
	TaskID      string `json:"taskId"`
	StartAtMS   int64  `json:"startAtMs"`
	EndAtMS     int64  `json:"endAtMs"`
	Note        string `json:"note,omitempty"`
	CreatedAtMS int64  `json:"createdAtMs"`
}

type BlockEvent struct {
	Action string `json:"action"` // "created", "updated", "deleted"
	Block  Block  `json:"block"`
}
