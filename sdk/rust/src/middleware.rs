use std::sync::Arc;

use crate::errors::CapError;
use crate::pb::{BusPacket, JobRequest, JobResult};

/// Context passed through the middleware chain.
#[derive(Clone)]
pub struct Context {
    pub trace_id: String,
    pub sender_id: String,
    pub packet: Option<BusPacket>,
}

/// Synchronous job handler function type.
pub type HandlerFn = Arc<dyn Fn(&Context, &JobRequest) -> Result<JobResult, CapError> + Send + Sync>;

/// Middleware wraps a handler, returning a new handler.
pub type MiddlewareFn = Arc<dyn Fn(HandlerFn) -> HandlerFn + Send + Sync>;

/// Compose a chain of middlewares around a base handler.
/// Middlewares execute in FIFO order (first added = outermost wrapper).
pub fn apply_middleware(base: HandlerFn, chain: &[MiddlewareFn]) -> HandlerFn {
    let mut handler = base;
    for mw in chain.iter().rev() {
        handler = mw(handler);
    }
    handler
}

/// Middleware that logs job_id and duration for each request.
pub fn logging_middleware() -> MiddlewareFn {
    Arc::new(|next: HandlerFn| -> HandlerFn {
        Arc::new(move |ctx: &Context, req: &JobRequest| -> Result<JobResult, CapError> {
            let start = std::time::Instant::now();
            let result = next(ctx, req);
            let elapsed = start.elapsed().as_millis();
            tracing::info!(job_id = %req.job_id, duration_ms = %elapsed, "[cap] job completed");
            result
        })
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::Mutex;

    fn echo_handler() -> HandlerFn {
        Arc::new(|_ctx: &Context, req: &JobRequest| -> Result<JobResult, CapError> {
            Ok(JobResult {
                job_id: req.job_id.clone(),
                status: 5, // SUCCEEDED
                ..Default::default()
            })
        })
    }

    #[test]
    fn empty_middleware_returns_base() {
        let handler = apply_middleware(echo_handler(), &[]);
        let ctx = Context { trace_id: "t".into(), sender_id: "s".into(), packet: None };
        let req = JobRequest { job_id: "j-1".into(), topic: "t".into(), ..Default::default() };
        let result = handler(&ctx, &req).unwrap();
        assert_eq!(result.job_id, "j-1");
    }

    #[test]
    fn three_middlewares_fifo_order() {
        let order = Arc::new(Mutex::new(Vec::new()));

        let make_mw = |id: i32| -> MiddlewareFn {
            let order = order.clone();
            Arc::new(move |next: HandlerFn| -> HandlerFn {
                let order = order.clone();
                let id = id;
                Arc::new(move |ctx: &Context, req: &JobRequest| -> Result<JobResult, CapError> {
                    order.lock().unwrap().push(id);
                    next(ctx, req)
                })
            })
        };

        let handler = apply_middleware(echo_handler(), &[make_mw(1), make_mw(2), make_mw(3)]);
        let ctx = Context { trace_id: "t".into(), sender_id: "s".into(), packet: None };
        let req = JobRequest { job_id: "j".into(), topic: "t".into(), ..Default::default() };
        handler(&ctx, &req).unwrap();

        assert_eq!(*order.lock().unwrap(), vec![1, 2, 3]);
    }

    #[test]
    fn logging_middleware_does_not_interfere() {
        let handler = apply_middleware(echo_handler(), &[logging_middleware()]);
        let ctx = Context { trace_id: "t".into(), sender_id: "s".into(), packet: None };
        let req = JobRequest { job_id: "logged-job".into(), topic: "t".into(), ..Default::default() };
        let result = handler(&ctx, &req).unwrap();
        assert_eq!(result.job_id, "logged-job");
    }
}
