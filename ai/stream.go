package ai

import (
	"context"
	"fmt"
	"sync"
)

// EventStream is a push-based async event stream with a final result.
// The producer calls Push() and End() or Error(); the consumer iterates via Recv() or Events().
type EventStream struct {
	events chan StreamEvent
	done   chan struct{}
	result *AssistantMessage
	err    error
	once   sync.Once
}

// NewEventStream creates a buffered event stream.
func NewEventStream(bufSize int) *EventStream {
	if bufSize < 1 {
		bufSize = 64
	}
	return &EventStream{
		events: make(chan StreamEvent, bufSize),
		done:   make(chan struct{}),
	}
}

// Push sends an event to the stream. Must not be called after End/Error.
func (s *EventStream) Push(event StreamEvent) {
	s.events <- event
}

// End completes the stream with a final result.
func (s *EventStream) End(msg *AssistantMessage) {
	s.once.Do(func() {
		s.result = msg
		close(s.events)
		close(s.done)
	})
}

// Error completes the stream with an error.
func (s *EventStream) Error(msg *AssistantMessage) {
	s.once.Do(func() {
		s.result = msg
		close(s.events)
		close(s.done)
	})
}

// Recv returns the next event from the stream.
// Returns the event and true, or a zero event and false when the stream is done.
func (s *EventStream) Recv() (StreamEvent, bool) {
	ev, ok := <-s.events
	return ev, ok
}

// Events returns the underlying channel for use in for-range loops.
func (s *EventStream) Events() <-chan StreamEvent {
	return s.events
}

// Result blocks until the stream completes and returns the final assistant message.
func (s *EventStream) Result() *AssistantMessage {
	<-s.done
	return s.result
}

// Wait blocks until the stream completes.
func (s *EventStream) Wait() {
	<-s.done
}

// Collect consumes all events and returns the final result.
// Useful when you don't need to process individual events.
func (s *EventStream) Collect() *AssistantMessage {
	for range s.events {
		// drain
	}
	return s.result
}

// CollectWithEvents consumes all events and returns both events and the final result.
func (s *EventStream) CollectWithEvents() ([]StreamEvent, *AssistantMessage) {
	var events []StreamEvent
	for ev := range s.events {
		events = append(events, ev)
	}
	return events, s.result
}

// StreamFunc is the signature for provider streaming functions.
type StreamFunc func(ctx context.Context, model *Model, reqCtx *Context, opts StreamOptions) *EventStream

// Stream initiates a streaming LLM call using the registered provider for the model's API.
func Stream(ctx context.Context, model *Model, reqCtx *Context, opts StreamOptions) *EventStream {
	provider := GetAPIProvider(model.API)
	if provider == nil {
		stream := NewEventStream(1)
		errMsg := &AssistantMessage{
			StopReason:   StopReasonError,
			ErrorMessage: "no provider registered for API: " + string(model.API),
		}
		stream.Push(StreamEvent{Type: EventError, Message: errMsg, Reason: StopReasonError})
		stream.Error(errMsg)
		return stream
	}
	return provider.Stream(ctx, model, reqCtx, opts)
}

// Complete performs a non-streaming LLM call and returns the complete response.
func Complete(ctx context.Context, model *Model, reqCtx *Context, opts StreamOptions) (*AssistantMessage, error) {
	stream := Stream(ctx, model, reqCtx, opts)
	result := stream.Collect()
	if result != nil && result.StopReason == StopReasonError {
		return result, fmt.Errorf("LLM error: %s", result.ErrorMessage)
	}
	return result, nil
}
