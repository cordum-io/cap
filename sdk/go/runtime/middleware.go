package runtime

import (
	"log"
	"time"
)

// HandlerFunc is the type-erased handler signature used internally by the Agent.
type HandlerFunc func(Context, any) (any, error)

// Middleware wraps a HandlerFunc, returning a new HandlerFunc.
// Middleware is applied in FIFO order: the first registered middleware is the outermost.
type Middleware func(next HandlerFunc) HandlerFunc

// Use appends middleware to the Agent. Middleware executes in registration order
// before the handler. Each middleware can inspect/modify the context, measure
// timing, or short-circuit by returning without calling next.
func (a *Agent) Use(mw ...Middleware) {
	a.middlewares = append(a.middlewares, mw...)
}

// applyMiddleware wraps handler through the middleware chain in FIFO order.
func applyMiddleware(handler HandlerFunc, mws []Middleware) HandlerFunc {
	h := handler
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// LoggingMiddleware returns a Middleware that logs the job ID, topic, and
// execution duration for every request.
func LoggingMiddleware(logger *log.Logger) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx Context, data any) (any, error) {
			start := time.Now()
			out, err := next(ctx, data)
			elapsed := time.Since(start)
			if err != nil {
				logger.Printf("middleware: job=%s topic=%s duration=%s error=%v",
					ctx.Job.GetJobId(), ctx.Job.GetTopic(), elapsed, err)
			} else {
				logger.Printf("middleware: job=%s topic=%s duration=%s ok",
					ctx.Job.GetJobId(), ctx.Job.GetTopic(), elapsed)
			}
			return out, err
		}
	}
}
