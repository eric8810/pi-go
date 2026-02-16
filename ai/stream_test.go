package ai

import (
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// NewEventStream
// ---------------------------------------------------------------------------

func TestNewEventStream(t *testing.T) {
	s := NewEventStream(10)
	if s == nil {
		t.Fatal("NewEventStream returned nil")
	}
	if cap(s.events) != 10 {
		t.Errorf("channel capacity = %d, want 10", cap(s.events))
	}
}

func TestNewEventStream_DefaultBufSize(t *testing.T) {
	s := NewEventStream(0)
	if cap(s.events) != 64 {
		t.Errorf("channel capacity = %d, want 64 (default)", cap(s.events))
	}

	s2 := NewEventStream(-1)
	if cap(s2.events) != 64 {
		t.Errorf("channel capacity = %d, want 64 (default for negative)", cap(s2.events))
	}
}

// ---------------------------------------------------------------------------
// Push / Recv
// ---------------------------------------------------------------------------

func TestPushRecv(t *testing.T) {
	s := NewEventStream(8)
	s.Push(StreamEvent{Type: EventTextDelta, Delta: "hello"})
	s.Push(StreamEvent{Type: EventTextDelta, Delta: " world"})

	ev, ok := s.Recv()
	if !ok {
		t.Fatal("expected ok=true on first Recv")
	}
	if ev.Delta != "hello" {
		t.Errorf("Delta = %q, want %q", ev.Delta, "hello")
	}

	ev, ok = s.Recv()
	if !ok {
		t.Fatal("expected ok=true on second Recv")
	}
	if ev.Delta != " world" {
		t.Errorf("Delta = %q, want %q", ev.Delta, " world")
	}
}

func TestRecv_ReturnsFalseAfterEnd(t *testing.T) {
	s := NewEventStream(8)
	s.Push(StreamEvent{Type: EventTextDelta, Delta: "data"})
	msg := &AssistantMessage{Content: []ContentBlock{TextBlock("done")}}
	s.End(msg)

	// Drain buffered event
	ev, ok := s.Recv()
	if !ok || ev.Delta != "data" {
		t.Fatalf("expected buffered event, got ok=%v, delta=%q", ok, ev.Delta)
	}

	// Next recv should signal closed
	_, ok = s.Recv()
	if ok {
		t.Error("expected ok=false after stream ended")
	}
}

// ---------------------------------------------------------------------------
// Events channel
// ---------------------------------------------------------------------------

func TestEvents_ForRange(t *testing.T) {
	s := NewEventStream(8)
	go func() {
		s.Push(StreamEvent{Type: EventTextDelta, Delta: "a"})
		s.Push(StreamEvent{Type: EventTextDelta, Delta: "b"})
		s.Push(StreamEvent{Type: EventTextDelta, Delta: "c"})
		s.End(&AssistantMessage{Content: []ContentBlock{TextBlock("abc")}})
	}()

	var deltas []string
	for ev := range s.Events() {
		deltas = append(deltas, ev.Delta)
	}
	if len(deltas) != 3 {
		t.Fatalf("expected 3 events, got %d", len(deltas))
	}
	if deltas[0] != "a" || deltas[1] != "b" || deltas[2] != "c" {
		t.Errorf("deltas = %v, want [a, b, c]", deltas)
	}
}

// ---------------------------------------------------------------------------
// End and Result
// ---------------------------------------------------------------------------

func TestEnd_And_Result(t *testing.T) {
	s := NewEventStream(4)
	msg := &AssistantMessage{
		Content:    []ContentBlock{TextBlock("final answer")},
		StopReason: StopReasonStop,
	}
	s.End(msg)

	result := s.Result()
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.GetText() != "final answer" {
		t.Errorf("result text = %q, want %q", result.GetText(), "final answer")
	}
	if result.StopReason != StopReasonStop {
		t.Errorf("StopReason = %q, want %q", result.StopReason, StopReasonStop)
	}
}

// ---------------------------------------------------------------------------
// Error completes the stream
// ---------------------------------------------------------------------------

func TestError_CompletesStream(t *testing.T) {
	s := NewEventStream(4)
	errMsg := &AssistantMessage{
		StopReason:   StopReasonError,
		ErrorMessage: "something broke",
	}
	s.Error(errMsg)

	result := s.Result()
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.StopReason != StopReasonError {
		t.Errorf("StopReason = %q, want %q", result.StopReason, StopReasonError)
	}
	if result.ErrorMessage != "something broke" {
		t.Errorf("ErrorMessage = %q, want %q", result.ErrorMessage, "something broke")
	}
}

// ---------------------------------------------------------------------------
// Collect
// ---------------------------------------------------------------------------

func TestCollect(t *testing.T) {
	s := NewEventStream(8)
	go func() {
		s.Push(StreamEvent{Type: EventTextDelta, Delta: "x"})
		s.Push(StreamEvent{Type: EventTextDelta, Delta: "y"})
		s.End(&AssistantMessage{Content: []ContentBlock{TextBlock("xy")}})
	}()

	result := s.Collect()
	if result == nil {
		t.Fatal("expected non-nil result from Collect")
	}
	if result.GetText() != "xy" {
		t.Errorf("result text = %q, want %q", result.GetText(), "xy")
	}
}

// ---------------------------------------------------------------------------
// CollectWithEvents
// ---------------------------------------------------------------------------

func TestCollectWithEvents(t *testing.T) {
	s := NewEventStream(8)
	go func() {
		s.Push(StreamEvent{Type: EventStart})
		s.Push(StreamEvent{Type: EventTextDelta, Delta: "hello"})
		s.Push(StreamEvent{Type: EventDone})
		s.End(&AssistantMessage{Content: []ContentBlock{TextBlock("hello")}})
	}()

	events, result := s.CollectWithEvents()
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	if events[0].Type != EventStart {
		t.Errorf("events[0].Type = %q, want %q", events[0].Type, EventStart)
	}
	if events[1].Type != EventTextDelta {
		t.Errorf("events[1].Type = %q, want %q", events[1].Type, EventTextDelta)
	}
	if events[2].Type != EventDone {
		t.Errorf("events[2].Type = %q, want %q", events[2].Type, EventDone)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.GetText() != "hello" {
		t.Errorf("result text = %q, want %q", result.GetText(), "hello")
	}
}

// ---------------------------------------------------------------------------
// Wait
// ---------------------------------------------------------------------------

func TestWait(t *testing.T) {
	s := NewEventStream(4)
	done := make(chan struct{})

	go func() {
		s.Wait()
		close(done)
	}()

	// Wait should block until End is called
	select {
	case <-done:
		t.Fatal("Wait() returned before End()")
	case <-time.After(50 * time.Millisecond):
		// Expected: Wait is still blocking
	}

	s.End(&AssistantMessage{Content: []ContentBlock{TextBlock("ok")}})

	select {
	case <-done:
		// Expected: Wait returned after End
	case <-time.After(1 * time.Second):
		t.Fatal("Wait() did not return after End()")
	}
}

// ---------------------------------------------------------------------------
// Multiple End calls don't panic (once.Do)
// ---------------------------------------------------------------------------

func TestMultipleEnd_NoPanic(t *testing.T) {
	s := NewEventStream(4)
	msg1 := &AssistantMessage{Content: []ContentBlock{TextBlock("first")}}
	msg2 := &AssistantMessage{Content: []ContentBlock{TextBlock("second")}}

	// First End should succeed
	s.End(msg1)
	// Second End should be a no-op (no panic)
	s.End(msg2)

	result := s.Result()
	// Result should be from the first End call
	if result.GetText() != "first" {
		t.Errorf("result text = %q, want %q (from first End)", result.GetText(), "first")
	}
}

func TestMultipleError_NoPanic(t *testing.T) {
	s := NewEventStream(4)
	msg1 := &AssistantMessage{ErrorMessage: "err1"}
	msg2 := &AssistantMessage{ErrorMessage: "err2"}

	s.Error(msg1)
	s.Error(msg2)

	result := s.Result()
	if result.ErrorMessage != "err1" {
		t.Errorf("ErrorMessage = %q, want %q", result.ErrorMessage, "err1")
	}
}

func TestEndThenError_NoPanic(t *testing.T) {
	s := NewEventStream(4)
	s.End(&AssistantMessage{Content: []ContentBlock{TextBlock("ok")}})
	s.Error(&AssistantMessage{ErrorMessage: "nope"})

	result := s.Result()
	if result.GetText() != "ok" {
		t.Errorf("result text = %q, want %q", result.GetText(), "ok")
	}
}

// ---------------------------------------------------------------------------
// Concurrent Push and Recv
// ---------------------------------------------------------------------------

func TestConcurrentPushRecv(t *testing.T) {
	s := NewEventStream(256)
	const numEvents = 100

	var wg sync.WaitGroup

	// Producer goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numEvents; i++ {
			s.Push(StreamEvent{Type: EventTextDelta, Delta: "x"})
		}
		s.End(&AssistantMessage{Content: []ContentBlock{TextBlock("done")}})
	}()

	// Consumer goroutine
	var received int
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range s.Events() {
			received++
		}
	}()

	wg.Wait()

	if received != numEvents {
		t.Errorf("received %d events, want %d", received, numEvents)
	}

	result := s.Result()
	if result == nil || result.GetText() != "done" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestConcurrentMultipleProducers(t *testing.T) {
	s := NewEventStream(512)
	const producerCount = 5
	const eventsPerProducer = 20

	var wgProd sync.WaitGroup
	for i := 0; i < producerCount; i++ {
		wgProd.Add(1)
		go func() {
			defer wgProd.Done()
			for j := 0; j < eventsPerProducer; j++ {
				s.Push(StreamEvent{Type: EventTextDelta, Delta: "d"})
			}
		}()
	}

	// Close the stream after all producers finish
	go func() {
		wgProd.Wait()
		s.End(&AssistantMessage{Content: []ContentBlock{TextBlock("all done")}})
	}()

	events, result := s.CollectWithEvents()
	expected := producerCount * eventsPerProducer
	if len(events) != expected {
		t.Errorf("collected %d events, want %d", len(events), expected)
	}
	if result == nil || result.GetText() != "all done" {
		t.Errorf("unexpected result: %+v", result)
	}
}

// ---------------------------------------------------------------------------
// CollectWithEvents on empty stream
// ---------------------------------------------------------------------------

func TestCollectWithEvents_Empty(t *testing.T) {
	s := NewEventStream(4)
	s.End(&AssistantMessage{Content: []ContentBlock{TextBlock("empty")}})

	events, result := s.CollectWithEvents()
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
	if result == nil || result.GetText() != "empty" {
		t.Errorf("unexpected result: %+v", result)
	}
}
