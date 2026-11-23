package service

import "unicode/utf8"

// Baidu combines description and topics in the observed editor's single field.
// Keep the exact confirmed text; validation never trims or truncates it.
func publicationTextLength(description, topics string) int {
	length := utf8.RuneCountInString(description)
	if topics != "" {
		length += 1 + utf8.RuneCountInString(topics)
	}
	return length
}
