package model

// PublicationCopy is new-form input only. It never carries resources,
// confirmation, request identity, execution authorization or old results.
type PublicationCopy struct {
	Platform   string      `json:"platform"`
	CategoryID int64       `json:"category_id,string"`
	Content    CopyContent `json:"content"`
}
type CopyContent struct {
	Description string `json:"description"`
	Topics      string `json:"topics"`
	Declaration string `json:"declaration"`
}
