// Package jbovlaste appears to be related to a specific domain or feature of the application,
// possibly involving Server-Sent Events (SSE) for real-time communication.
// The name "jbovlaste" might be specific to the Lojban context of the application.
// In a Nest.js context, this could be part of a module handling real-time updates,
// perhaps using SSE, WebSockets, or integrating with a message broker.
package jbovlaste

// SSEEvent represents a Server-Sent Event.
// Think of this as the actual message or piece of news that the radio station (server)
// sends out to its listeners (clients).
// This struct defines the structure of data sent over an SSE connection.
// In a real application, this might include fields like Event (type of news), ID (message number), Retry (how often to try again if lost), etc.
type SSEEvent struct {
	// Data is the main content of the message. For example, if the server is sending
	// progress updates for a file download, Data might be "25% complete", then "50% complete".
	// In SSE, this corresponds to the "data:" field in the event stream.
	Data string // The data payload of the event
	

	// Optional fields for more structured SSE events:
	// Event string // Optional: You could add a type here, like "progressUpdate" or "newMessage".
	// ID    string // Optional: You could give each message a unique ID.
}

// NewSSEEvent creates a new SSEEvent with the given data.
// This is a simple constructor function for `SSEEvent`.
// This is a quick way to make a new news message.
func NewSSEEvent(data string) SSEEvent {
	return SSEEvent{Data: data} // Just put the data into our SSEEvent envelope.
}