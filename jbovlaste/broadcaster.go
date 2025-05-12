// Package jbovlaste, as mentioned before, seems to handle real-time communication,
// Server-Sent Events (SSE), for a domain-specific feature ("jbovlaste").
// This file, `broadcaster.go`, defines the `Broadcaster` which manages client connections
// and broadcasts messages to them. This is a common pattern for implementing SSE servers.
// In Nest.js, this might be handled by a Gateway (for WebSockets) or a custom service
// managing SSE connections and event streams.
package jbovlaste

import (
	"fmt"
	// `sync` provides synchronization primitives like `Mutex` and `RWMutex` for safe concurrent access to shared data.
	"sync"

	// `uuid` is used to generate unique identifiers for clients.
	"github.com/google/uuid"
)

// ClientInfo holds the channels and state for a connected client.
// Imagine each person (client) listening to a radio station (our server) has their own little setup.
type ClientInfo struct {
	// sseChannel: This is like the client's personal radio receiver.
	// When the server has news (an SSEEvent), it sends it through this channel.
	// `chan SSEEvent` means it's a "channel" that carries "SSEEvent" messages.
	sseChannel chan SSEEvent

	// `cancelChannel` is used to signal a long-running task associated with this client to stop.
	// cancelChannel: If the client is doing a long task (like downloading a big file, called "import" here),
	// and they want to stop it, a message is sent through this channel.
	// `chan bool` means it carries a true/false signal (true usually means "cancel!").
	cancelChannel chan bool

	// `isCancelled` tracks the cancellation state of the client's task.
	// isCancelled: A simple flag (true or false) to remember if this client's task has been told to stop.
	isCancelled bool

	// `mu` is a mutex to protect concurrent access to `isCancelled`.
	// This is important if multiple goroutines might try to read or write `isCancelled` simultaneously.
	// mu: This is a "mutex", short for "mutual exclusion".
	// It's like a "talking stick". Only the person (part of the code) holding the stick
	// can change `isCancelled`. This prevents confusion if multiple parts try to change it at once.
	mu sync.Mutex
}

// Broadcaster manages SSE clients and message broadcasting.
// This is like the main radio station control room.
// It keeps a list of all listeners (clients) and sends out messages (broadcasts).
type Broadcaster struct {
	// clients: A list (map) of all currently connected listeners.
	// The key is a unique client ID (string), and the value is a pointer to `ClientInfo`.
	// The "key" is the client's unique ID (a string), and the "value" is their `ClientInfo` (their personal setup).
	clients map[string]*ClientInfo

	// `mu` is a Read-Write Mutex (`RWMutex`) to protect the `clients` map.
	// `RWMutex` allows multiple readers or one writer. This is efficient if reads are more frequent than writes.
	// mu: Another "talking stick" (RWMutex means Read-Write Mutex).
	// This one protects the `clients` list.
	// Many can read the list at the same time (e.g., to send a message).
	// But only one can write to it (e.g., add or remove a client) at a time.
	mu sync.RWMutex
}

// NewBroadcaster creates and returns a new Broadcaster instance.
// This is a constructor function for `Broadcaster`.
// This is like building a new radio station control room from scratch.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		// Initialize with an empty list of clients.
		clients: make(map[string]*ClientInfo),
	}
}

// NewClient registers a new client and returns its ID, SSE event channel, and cancellation channel.
// The returned sseEvents channel is for the client to receive SSE messages.
// The returned cancelSignal channel is for the service performing a task to listen for cancellation.
// This is like when a new listener tunes into our radio station.
// It returns three values: the client's ID, a receive-only SSE channel, and a receive-only cancel channel.
func (b *Broadcaster) NewClient() (string, <-chan SSEEvent, <-chan bool) {
	// Lock the mutex for exclusive access to the `clients` map, as we are modifying it.
	b.mu.Lock() // Grab the talking stick for the `clients` list (exclusive access).
	// `defer b.mu.Unlock()` ensures the mutex is unlocked when the function returns.
	defer b.mu.Unlock() // Make sure to release the stick when we're done, no matter what.

	clientID := uuid.New().String() // Give the new listener a unique ID (like a caller ID).
	// Create and initialize the `ClientInfo` for the new client.
	// Set up the new listener's personal radio receiver and cancellation button.
	clientInfo := &ClientInfo{
		// `make(chan SSEEvent, 32)` creates a channel that can hold up to 32 SSE events
		// before the sender has to wait. It's like a small buffer or waiting line for messages.
		sseChannel: make(chan SSEEvent, 32),
		// `make(chan bool, 1)` creates a channel for the cancel signal, buffered for 1 message.
		// This means sending a cancel signal won't block if the receiver isn't immediately ready.
		cancelChannel: make(chan bool, 1),
		isCancelled:   false, // They haven't cancelled anything yet.
	}

	// Add the new client to the `clients` map.
	b.clients[clientID] = clientInfo                    // Add this new listener to our list.
	fmt.Printf("New client registered: %s\n", clientID) // Log that a new listener joined.

	// Give back:
	// 1. Their unique ID.
	// 2. Their personal radio receiver (`sseChannel`) so they can get messages.
	//    The `<-chan` means it's a "receive-only" channel from the outside's perspective.
	// 3. Their cancellation signal receiver (`cancelChannel`) so the part of our server
	//    doing work for them knows if they hit "cancel".
	return clientID, clientInfo.sseChannel, clientInfo.cancelChannel
}

// Broadcast sends an SSEEvent to the specified client.
// It returns an error if the client is not found or if the send operation fails (e.g., channel closed).
// This is like the radio station sending a specific news update to one particular listener.
func (b *Broadcaster) Broadcast(clientID string, event SSEEvent) error {
	// Use RLock for read access to `clients` map, allowing concurrent reads.
	b.mu.RLock()                          // Grab the "read" part of the talking stick for `clients` (others can also read).
	clientInfo, ok := b.clients[clientID] // Find the listener by their ID.
	b.mu.RUnlock()                        // Release the read stick.

	if !ok { // If we couldn't find a listener with that ID...
		// Return an error if the client is not found.
		return fmt.Errorf("client ID %s not found", clientID) // ...report an error.
	}

	// Now, check if this specific listener has already asked to cancel their task.
	// Lock the individual client's mutex to safely access `isCancelled`.
	clientInfo.mu.Lock() // Grab the talking stick for this specific client's `isCancelled` flag.
	if clientInfo.isCancelled {
		clientInfo.mu.Unlock() // Release the client's stick.
		// If they cancelled, maybe we don't send them this update.
		// fmt.Printf("Skipping broadcast to cancelled client %s\n", clientID)
		// Decide on behavior: either return an error or succeed quietly.
		return nil // Or return an error saying "they cancelled". For now, just succeed quietly.
	}
	clientInfo.mu.Unlock() // Release the client's stick.

	// Use a `select` statement with a `default` case for a non-blocking send on the `sseChannel`.
	// Try to send the news update (event) to the listener's personal radio receiver (sseChannel).
	// The `select` statement here is a bit like trying one thing, but having a backup plan.
	select {
	case clientInfo.sseChannel <- event: // Try to send the event.
		return nil // Success!
	default:
		// If we reach `default`, it means `clientInfo.sseChannel <- event` would have blocked.
		// This usually happens if the listener's channel is full (they're not processing messages fast enough)
		// or if they've disconnected and their channel is closed.
		fmt.Printf("Failed to send SSE to client %s: channel likely full or closed\n", clientID)
		// We might decide to remove such an unresponsive client:
		// b.RemoveClient(clientID)
		return fmt.Errorf("failed to send SSE to client %s: channel full or closed", clientID)
	}
}

// CancelImport sends a cancellation signal to the task associated with the clientID.
// It returns an error if the client is not found or if the import was already cancelled.
// This is when the server itself (or another part of it) decides to tell a client's task to stop.
func (b *Broadcaster) CancelImport(clientID string) error {
	// Read-lock to find the client.
	b.mu.RLock()                          // Grab read stick for the main clients list.
	clientInfo, ok := b.clients[clientID] // Find the client.
	b.mu.RUnlock()                        // Release read stick.

	if !ok { // Client not found?
		// Return error if client doesn't exist.
		return fmt.Errorf("client ID %s not found for cancellation", clientID)
	}

	// Now, work with this specific client's cancellation status.
	// Lock the individual client's mutex to modify `isCancelled` and send on `cancelChannel`.
	clientInfo.mu.Lock()         // Grab the talking stick for this client's `isCancelled` flag.
	defer clientInfo.mu.Unlock() // Ensure this client's stick is released when done.

	if clientInfo.isCancelled { // Already cancelled?
		return fmt.Errorf("import for client ID %s already cancelled", clientID)
	}

	clientInfo.isCancelled = true // Mark it as cancelled.

	// Non-blocking send on the `cancelChannel`.
	// Try to send the "stop!" signal on their `cancelChannel`.
	select {
	case clientInfo.cancelChannel <- true: // Send `true` to signal cancellation.
		fmt.Printf("Cancellation signal sent to client %s\n", clientID)
		return nil // Signal sent successfully.
	default:
		// This `default` case means sending to `cancelChannel` would block.
		// This is unlikely if the channel is buffered (size 1) and we only send one cancel signal.
		// If it happens, it might mean we tried to cancel multiple times or something is wrong.
		return fmt.Errorf("failed to send cancellation signal to client %s: channel full or closed", clientID)
	}
}

// RemoveClient unregisters a client and closes its associated channels.
// This is when a listener disconnects or we decide to remove them.
func (b *Broadcaster) RemoveClient(clientID string) {
	// Full lock to modify the `clients` map.
	b.mu.Lock()         // Grab the main talking stick for `clients` list (exclusive access).
	defer b.mu.Unlock() // Release stick when done.

	clientInfo, ok := b.clients[clientID] // Find the client.
	if ok {                               // If they exist in our list...
		// Closing a channel is a way to tell anyone listening on it that no more messages will come.
		// It also unblocks any goroutines waiting to receive from the channel.
		close(clientInfo.sseChannel)    // Close their personal radio receiver.
		close(clientInfo.cancelChannel) // Close their cancellation signal channel.
		// Remove the client from the map.
		delete(b.clients, clientID) // Remove them from our list of active listeners.
		fmt.Printf("Client %s removed\n", clientID)
	}
}

// ListActiveImports returns a list of client IDs that are considered active (not cancelled).
// In this Go version, "active import" is synonymous with an "active client" that hasn't been explicitly cancelled.
func (b *Broadcaster) ListActiveImports() []string {
	// Read-lock to iterate over clients.
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Create a slice to hold active client IDs.
	activeIDs := make([]string, 0, len(b.clients))
	for id, clientInfo := range b.clients {
		clientInfo.mu.Lock()
		// Safely read the `isCancelled` flag.
		isCancelled := clientInfo.isCancelled
		clientInfo.mu.Unlock()
		if !isCancelled {
			activeIDs = append(activeIDs, id)
		}
	}
	return activeIDs
}

// GetClientSSEChannel returns the SSE channel for a given client ID.
// This is useful for an HTTP handler to stream SSE events to the client.
// Returns nil if client is not found.
func (b *Broadcaster) GetClientSSEChannel(clientID string) <-chan SSEEvent {
	// Read-lock to access the `clients` map.
	b.mu.RLock()
	defer b.mu.RUnlock()
	// Return the client's SSE channel (as receive-only) if the client exists.
	if client, ok := b.clients[clientID]; ok {
		return client.sseChannel
	}
	return nil
}
