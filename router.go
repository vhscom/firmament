package firmament

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"sync"
)

// SignalHandler is implemented by anything that can receive a Signal from the Router.
type SignalHandler interface {
	// HandleSignal processes a signal. Errors are logged but do not interrupt routing.
	HandleSignal(Signal) error
}

// Router distributes Signals from a Monitor to registered SignalHandlers.
// It reads from the signal channel until it is closed or the context is done.
// Routing is one-directional: monitor → handlers.
type Router struct {
	mu       sync.RWMutex
	handlers []SignalHandler
}

// NewRouter creates an empty Router.
func NewRouter() *Router { return &Router{} }

// Add registers a SignalHandler. Safe to call concurrently with Route.
func (r *Router) Add(h SignalHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers = append(r.handlers, h)
}

// Route reads signals from the provided channel and dispatches each to all
// registered handlers. Blocks until the channel is closed or ctx is done.
func (r *Router) Route(ctx context.Context, signals <-chan Signal) {
	for {
		select {
		case <-ctx.Done():
			return
		case sig, ok := <-signals:
			if !ok {
				return
			}
			r.dispatch(sig)
		}
	}
}

// dispatch delivers sig to every registered handler, logging any errors.
func (r *Router) dispatch(sig Signal) {
	r.mu.RLock()
	handlers := make([]SignalHandler, len(r.handlers))
	copy(handlers, r.handlers)
	r.mu.RUnlock()

	for _, h := range handlers {
		if err := h.HandleSignal(sig); err != nil {
			slog.Error("signal handler error", "err", err)
		}
	}
}

// LogHandler writes each Signal as a JSON line to the provided writer.
// It is safe for concurrent use.
type LogHandler struct {
	mu  sync.Mutex
	enc *json.Encoder
}

// NewLogHandler creates a LogHandler that writes JSON-encoded signals to w.
func NewLogHandler(w io.Writer) *LogHandler {
	return &LogHandler{enc: json.NewEncoder(w)}
}

// HandleSignal implements SignalHandler.
func (h *LogHandler) HandleSignal(sig Signal) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.enc.Encode(sig)
}

// CallbackHandler invokes a function for each received Signal.
type CallbackHandler struct {
	fn func(Signal) error
}

// NewCallbackHandler creates a SignalHandler that calls fn for each signal.
func NewCallbackHandler(fn func(Signal) error) *CallbackHandler {
	return &CallbackHandler{fn: fn}
}

// HandleSignal implements SignalHandler.
func (h *CallbackHandler) HandleSignal(sig Signal) error {
	return h.fn(sig)
}
