package jsondiscrim_test

import (
	"fmt"

	"github.com/go-json-experiment/json"

	"github.com/cue-exp/jsondiscrim"
)

// Message is the interface type that all message types implement.
type Message interface {
	Format() string
}

// TextMessage represents a text message.
type TextMessage struct {
	Type jsondiscrim.Const[string, struct{ string `const:"text"` }] `json:"type"`
	Text string                                                       `json:"text"`
}

func (m *TextMessage) Format() string {
	return fmt.Sprintf("Text: %s", m.Text)
}

// ImageMessage represents an image message.
type ImageMessage struct {
	Type jsondiscrim.Const[string, struct{ string `const:"image"` }] `json:"type"`
	URL  string                                                        `json:"url"`
	Alt  string                                                        `json:"alt"`
}

func (m *ImageMessage) Format() string {
	return fmt.Sprintf("Image: %s (%s)", m.URL, m.Alt)
}

// LinkMessage represents a link message.
type LinkMessage struct {
	Type jsondiscrim.Const[string, struct{ string `const:"link"` }] `json:"type"`
	URL  string                                                       `json:"url"`
	Text string                                                       `json:"text"`
}

func (m *LinkMessage) Format() string {
	return fmt.Sprintf("Link: %s -> %s", m.Text, m.URL)
}

// Conversation represents a collection of messages.
type Conversation struct {
	Messages []Message `json:"messages"`
}

// This example demonstrates how to use Structs to unmarshal JSON
// with a discriminator field into different concrete types based
// on the discriminator value.
func Example_structs() {
	// Create an unmarshaler for the Message interface.
	unmarshalers := jsondiscrim.Structs[Message](
		(*TextMessage)(nil),
		(*ImageMessage)(nil),
		(*LinkMessage)(nil),
	)

	// Example JSON conversation with mixed message types.
	conversationJSON := `{
		"messages": [
			{"type":"text","text":"Hello, World!"},
			{"type":"image","url":"https://example.com/pic.jpg","alt":"A picture"},
			{"type":"link","url":"https://example.com","text":"Visit our site"},
			{"type":"text","text":"Goodbye!"}
		]
	}`

	// Unmarshal the entire conversation.
	var conv Conversation
	if err := json.Unmarshal([]byte(conversationJSON), &conv, json.WithUnmarshalers(unmarshalers)); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Print each message using its Format method.
	for _, msg := range conv.Messages {
		fmt.Println(msg.Format())
	}

	// Output:
	// Text: Hello, World!
	// Image: https://example.com/pic.jpg (A picture)
	// Link: Visit our site -> https://example.com
	// Text: Goodbye!
}
