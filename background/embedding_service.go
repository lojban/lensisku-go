// Package background contains services and tasks that run in the background,
// independently of direct HTTP request-response cycles. This is useful for
// long-running processes, scheduled jobs, or asynchronous operations.
// In Nest.js, this could be analogous to using `@nestjs/schedule` for cron jobs,
// or integrating with message queues (like BullMQ) for task processing.
package background

import (
	"fmt"
	"log"
	// `sync` package provides synchronization primitives like `WaitGroup` and `Mutex`.
	"sync"
	"time"

	// `pgxpool` for database interactions.
	"github.com/jackc/pgx/v5/pgxpool"
)

// DefinitionToEmbed represents a definition that needs its text embedding calculated.
// ELI5: Think of this as a work order slip for a specific piece of text (a definition).
// It says, "Here's the ID of the definition and the text itself. Please make an embedding for it!"
type DefinitionToEmbed struct {
	ID   int    // A unique number for this definition.
	Text string // The actual words of the definition.
}

// EmbeddingResult holds the computed embedding for a definition, or an error if computation failed.
// ELI5: This is the result slip you get back after the embedding work is done.
// It either has the magical number-list (Embedding) or a note saying "Oops, something went wrong" (Error).
type EmbeddingResult struct {
	DefinitionID int       // Which definition was this result for?
	Embedding    []float32 // The list of special numbers that represents the meaning of the text.
	Error        error     // If anything went wrong, the error message is here.
}

// Constants for configuring the embedding service.
const (
	// embeddingTickerDuration is how often the service checks for new definitions to process.
	// ELI5: Like setting an alarm clock that rings every 15 seconds, reminding us to look for new work orders.
	embeddingTickerDuration = 15 * time.Second

	// numProcessorWorkers is the number of concurrent workers processing definitions.
	// ELI5: We're hiring 3 helpers (processor workers) who can all work on calculating embeddings at the same time.
	// This helps get the work done faster, like having multiple cashiers at a store.
	numProcessorWorkers = 3
)

// StartEmbeddingCalculatorService initializes and starts the background service for calculating embeddings.
// It orchestrates fetching definitions, processing them, and updating them in the database.
// It can be gracefully shut down via the stopChan.
// ELI5: This function is the main manager for our "Embedding Calculation Factory".
// `dbPool` is the database connection pool.
// `stopChan <-chan struct{}` is a read-only channel used to signal the service to stop.
// This pattern allows for graceful shutdown of background goroutines.
// It sets up all the machinery and workers, gets them started, and also knows how to tell everyone
// to clean up and go home when the `stopChan` signal arrives.
func StartEmbeddingCalculatorService(dbPool *pgxpool.Pool, stopChan <-chan struct{}) {
	log.Println("Background embedding calculator service starting...")

	// Channels are used for communication between goroutines.
	// defsToProcessChan is a channel for sending definitions that need processing.
	// ELI5: This is like a conveyor belt ('defsToProcessChan') where new work order slips (DefinitionToEmbed)
	// are placed. The processor workers pick up slips from this belt.
	// `make(chan DefinitionToEmbed, 10)` creates a buffered channel with a capacity of 10.
	// It's buffered, meaning it can hold up to 10 slips even if workers are busy.
	defsToProcessChan := make(chan DefinitionToEmbed, 10)

	// resultsChan is a channel for sending back the results of embedding calculations.
	// ELI5: This is another conveyor belt ('resultsChan') where the finished result slips (EmbeddingResult)
	// are placed by the processor workers. The updater worker picks up slips from this belt.
	// Also a buffered channel.
	resultsChan := make(chan EmbeddingResult, 10)

	// `sync.WaitGroup` is used to wait for a collection of goroutines to finish.
	// The main goroutine calls `Add` to set the number of goroutines to wait for,
	// and each goroutine calls `Done` when it finishes. `Wait` blocks until all
	// goroutines have called `Done`.
	// mainWg is a WaitGroup for managing the lifecycle of goroutines that are direct children
	// of the main orchestrator logic, like the updater.
	// ELI5: `mainWg` is like a checklist for the factory manager. For every major department (like the 'updater')
	// that needs to finish its work before the factory can fully close, we add an item to this list.
	// When a department finishes, it checks itself off. The manager waits until all items are checked.
	var mainWg sync.WaitGroup

	// processorsWg is specifically for the worker goroutines that process definitions.
	// This allows the orchestrator to know when all processing is done before closing `resultsChan`.
	// processorsWg is a WaitGroup specifically for the processor worker goroutines.
	// This helps in knowing when all processors have finished their work, which is crucial
	// for safely closing the resultsChan.
	// ELI5: `processorsWg` is a special checklist just for our 3 calculation helpers.
	// We need to know when ALL of them are done before we can turn off the 'resultsChan' conveyor belt.
	var processorsWg sync.WaitGroup

	// --- Main Orchestrator Goroutine ---
	// This goroutine is the heart of the service. It manages the ticker,
	// spawns workers, and handles the shutdown signal.
	// ELI5: We're starting the main factory manager (this goroutine). This manager doesn't do the
	// calculations itself but makes sure everyone else does their job and coordinates the shutdown.
	// `go func() { ... }()` starts a new goroutine. Goroutines are lightweight, concurrently executing functions.
	go func() {
		// This defer ensures that when this goroutine exits (e.g., on shutdown),
		// it logs that it has stopped.
		defer log.Println("Embedding calculator orchestrator goroutine stopped.")

		// `time.NewTicker` creates a ticker that sends a value on its channel (`orchestratorTicker.C`)
		// at regular intervals (`embeddingTickerDuration`).
		// orchestratorTicker is like the factory's main clock.
		// ELI5: This clock (`orchestratorTicker`) chimes every `embeddingTickerDuration` (15 seconds).
		// Each chime tells the manager it's time to check for new work orders.
		orchestratorTicker := time.NewTicker(embeddingTickerDuration)
		// `defer orchestratorTicker.Stop()` ensures the ticker is stopped when the goroutine exits,
		// freeing associated resources.
		defer orchestratorTicker.Stop() // Important to stop the ticker when done to free resources.

		// --- Processor Goroutines (Worker Pool) ---
		// This loop starts multiple `processorWorker` goroutines to process definitions concurrently.
		// This is a common pattern for creating a pool of workers.
		// ELI5: The manager now hires `numProcessorWorkers` (3) specialist helpers.
		// Each helper (processorWorker goroutine) will take work orders from the `defsToProcessChan` belt,
		// do the (simulated) embedding calculation, and put the result on the `resultsChan` belt.
		for i := 0; i < numProcessorWorkers; i++ {
			// Increment the WaitGroup counter for each worker goroutine.
			processorsWg.Add(1) // Add a task to the processors' checklist.
			// Start the worker goroutine.
			go func(workerID int) { // Each worker runs in its own goroutine.
				defer processorsWg.Done() // When this worker finishes all its tasks and exits, it checks itself off the processors' list.
				log.Printf("Processor Worker %d: Starting\n", workerID)
				// This worker keeps taking definitions from the 'defsToProcessChan' conveyor belt
				// as long as there are items and the belt hasn't been turned off (channel closed).
				for def := range defsToProcessChan {
					log.Printf("Processor Worker %d: Received definition ID: %d for processing.\n", workerID, def.ID)
					// Simulate work with `time.Sleep`. In a real application, this would involve
					// CPU-bound or I/O-bound operations (e.g., calling an ML model, database queries).
					// Simulate preprocessing (e.g., cleaning text)
					time.Sleep(500 * time.Millisecond) 
					log.Printf("Processor Worker %d: Simulating embedding calculation for definition ID: %d\n", workerID, def.ID)
					// Simulate the actual work of calculating an embedding (e.g., calling an AI model)
					time.Sleep(1 * time.Second) 

					// Create a dummy embedding result
					embedding := make([]float32, 3) // A list of 3 numbers
					for j := range embedding {
						embedding[j] = float32(def.ID) + float32(j)*0.1 + float32(workerID)*0.01 // Just some fake numbers
					}
					result := EmbeddingResult{DefinitionID: def.ID, Embedding: embedding}
					log.Printf("Processor Worker %d: Processed definition ID %d. Sending to resultsChan.\n", workerID, def.ID)
					// ELI5: The worker places the finished result slip onto the `resultsChan` conveyor belt.
					// Send the result to the `resultsChan`. This might block if `resultsChan` is full.
					resultsChan <- result
				}
				// This log message appears when `defsToProcessChan` is closed and the loop finishes.
				log.Printf("Processor Worker %d: defsToProcessChan closed. Exiting.\n", workerID)
			}(i) // Pass `i` to give each worker a unique ID.
		}

		// --- Updater Goroutine ---
		// ELI5: The manager hires one more helper, the 'updater'. This helper's job is to take
		// the finished result slips from the `resultsChan` belt and (simulated) save them to the database.
		// This goroutine consumes results from `resultsChan` and updates the database.
		mainWg.Add(1) // Add the updater to the main factory checklist.
		go func() {
			// Decrement the WaitGroup counter when this goroutine finishes.
			defer mainWg.Done() // When the updater finishes and exits, it checks itself off the main list.
			log.Println("Updater: Starting")
			// The updater keeps taking results from the `resultsChan` conveyor belt
			// as long as there are items and the belt hasn't been turned off (channel closed).
			for result := range resultsChan {
				if result.Error != nil {
					log.Printf("Updater: Error processing definition ID %d: %v\n", result.DefinitionID, result.Error)
				} else {
					log.Printf("Updater: Simulating update of embedding in DB for definition ID: %d with embedding: %v\n", result.DefinitionID, result.Embedding)
					// In a real application, this is where you'd write `result.Embedding` to the database for `result.DefinitionID`.
				}
			}
			// This log message appears when `resultsChan` is closed and the loop finishes.
			log.Println("Updater: resultsChan closed. Exiting.")
		}()

		// --- Goroutine to manage closing resultsChan ---
		// This goroutine waits for all processor workers to complete their tasks.
		// Once all processors are done, it's safe to close `resultsChan`.
		// This signals the updater that no more results will be coming.
		// ELI5: The manager assigns a special supervisor whose only job is to watch the calculation helpers.
		// When all calculation helpers have finished their day's work (checked off the `processorsWg` list),
		// this supervisor turns off (closes) the `resultsChan` conveyor belt. This tells the 'updater' helper
		// that there are no more finished results coming.
		go func() {
			// `processorsWg.Wait()` blocks until all processor workers have called `Done()`.
			processorsWg.Wait() // Wait until all items on the processors' checklist are done.
			log.Println("Orchestrator: All processor workers finished. Closing resultsChan.")
			// Closing `resultsChan` signals to the `range resultsChan` loop in the updater goroutine
			// that no more values will be sent, causing the loop to terminate.
			close(resultsChan)  // Now it's safe to close the results conveyor belt.
		}()

		// --- Orchestrator's Main Loop ---
		// ELI5: The factory manager now enters their main work loop. They will keep doing this
		// until they get the signal to shut down the whole factory.
		for {
			// The `select` statement allows a goroutine to wait on multiple communication operations.
			// It blocks until one of its cases can run, then it executes that case.
			// If multiple cases are ready, one is chosen at random.
			// The `select` statement is like the manager listening to multiple phones at once.
			// They will act on the first phone that rings.
			select {
			// Phone 1: The factory clock (`orchestratorTicker.C`) chimes.
			// This case runs when the ticker sends a value.
			case <-orchestratorTicker.C:
				log.Println("Embedding calculator tick: Time to fetch new definitions.")
				// ELI5: The clock chimed! The manager tells a scout (fetchAndSendDefinitions)
				// to go look for new work order slips and put them on the `defsToProcessChan` belt.
				fetchAndSendDefinitions(dbPool, defsToProcessChan)

			// Phone 2: The main stop signal (`stopChan`) for the whole factory arrives.
			// Case for receiving a signal on `stopChan`, indicating shutdown.
			// Phone 2: The main stop signal (`stopChan`) for the whole factory arrives.
			case <-stopChan:
				log.Println("Embedding calculator orchestrator: Stop signal received. Initiating shutdown sequence...")
				// ELI5: The "time to go home" signal arrived! The manager starts the shutdown process.

				// Step 1: Close `defsToProcessChan`.
				// This tells the processor workers that no new work orders will be added to their conveyor belt.
				// They should finish what they're currently working on and then stop.
				log.Println("Orchestrator: Closing defsToProcessChan to signal processors to finish up.")
				// Closing `defsToProcessChan` causes the `range defsToProcessChan` loops in the
				// processor workers to terminate after processing any remaining items in the channel.
				close(defsToProcessChan)

				// Step 2: Wait for the processors to finish and for `resultsChan` to be closed.
				// The `processorsWg.Wait()` inside the dedicated goroutine (above) ensures processors finish.
				// That goroutine then closes `resultsChan`.

				// Step 3: Wait for the updater (and any other main tasks) to finish.
				// The updater will finish processing any remaining items on `resultsChan` once it's closed.
				log.Println("Orchestrator: Waiting for updater and other main tasks to complete...")
				// `mainWg.Wait()` blocks until the updater goroutine (and any other goroutines managed by `mainWg`) calls `Done()`.
				mainWg.Wait() // Wait for all items on the main factory checklist to be done.

				log.Println("All embedding calculator dependent services (updater) finished.")
				return // Exit the main orchestrator goroutine, effectively stopping this part of the service.
			}
		}
	}() // End of main orchestrator goroutine

	log.Println("Background embedding calculator service successfully launched its orchestrator.")
	// StartEmbeddingCalculatorService returns now, allowing the main application to continue.
	// The embedding service runs in the background. Shutdown is triggered by closing `stopChan`.
}

// fetchAndSendDefinitions simulates fetching definitions from the database that need embeddings
// and sends them to the defsToProcessChan.
// ELI5: This is our scout. It goes to the (simulated) database, finds work order slips
// (definitions that don't have embeddings yet), and puts them on the `defsToProcessChan` conveyor belt
// for the processor workers.
// `defsToProcessChan chan<- DefinitionToEmbed` indicates that this function only sends to the channel.
func fetchAndSendDefinitions(dbPool *pgxpool.Pool, defsToProcessChan chan<- DefinitionToEmbed) {
	log.Println("Fetcher logic: Fetching definitions from DB (simulation)...")
	// In a real application, this would be a database query like:
	// SELECT id, text FROM definitions WHERE embedding IS NULL LIMIT 10;

	// Simulate fetching a couple of definitions.
	// We use time to make IDs somewhat unique for demonstration.
	dummyDefinitions := []DefinitionToEmbed{
		{ID: time.Now().Second()*100 + 1, Text: fmt.Sprintf("Definition fetched at %v", time.Now().Format(time.Kitchen))},
		{ID: time.Now().Second()*100 + 2, Text: "Another sample definition text for embedding."},
	}

	for _, def := range dummyDefinitions {
		// Try to send the definition to the processing channel.
		// Use a select with a default case to prevent blocking if the channel is full.
		// This is a non-blocking send attempt.
		// This makes the fetcher non-blocking if the processors are overwhelmed.
		select {
		case defsToProcessChan <- def:
			// If the send to `defsToProcessChan` succeeds immediately (channel not full).
			log.Printf("Fetcher logic: Sent definition ID %d to process.\n", def.ID)
		default:
			// ELI5: If the `defsToProcessChan` conveyor belt is full, the scout can't place more work orders
			// right now. It logs this and will try again on the next tick.
			log.Printf("Fetcher logic: defsToProcessChan is full. Skipping definition ID %d for this tick.\n", def.ID)
		}
	}
	log.Println("Fetcher logic: Finished attempting to send definitions for this tick.")
}