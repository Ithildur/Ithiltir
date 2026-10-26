package notify

import (
	"strings"

	"dash/internal/model"
)

// JoinMessages preserves each alert's title and body in a human-readable digest.
// Metadata describes the shared transition and language, not an individual event.
func JoinMessages(kind model.NotifyType, messages []Message) Message {
	first := messages[0]
	parts := make([]string, 0, len(messages))
	for _, message := range messages {
		if kind == model.NotifyTypeTelegram {
			parts = append(parts, telegramBotText(message))
		} else {
			parts = append(parts, message.Text())
		}
	}
	joined := Message{
		Body: strings.Join(parts, "\n\n---\n\n"),
		Metadata: map[string]string{
			"transition": first.Metadata["transition"],
			"language":   first.Metadata["language"],
		},
	}
	if kind == model.NotifyTypeEmail {
		joined.Title = emailTransitionLabel(first.Metadata["transition"], first.Metadata)
	}
	return joined
}
