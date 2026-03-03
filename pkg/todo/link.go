package todo

type Link struct {
	ID          string   `json:"id"`
	URL         string   `json:"url"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	CreatedAtMS int64    `json:"createdAtMs"`
	UpdatedAtMS int64    `json:"updatedAtMs"`
}

type LinkEvent struct {
	Action string `json:"action"` // "created", "updated", "deleted"
	Link   Link   `json:"link"`
}
