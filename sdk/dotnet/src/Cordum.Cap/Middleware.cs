using System.Diagnostics;
using Cordum.Agent.V1;

namespace Cordum.Cap;

/// <summary>
/// Context passed through the middleware chain.
/// </summary>
public sealed class Context
{
    public required string TraceId { get; init; }
    public required string SenderId { get; init; }
    public BusPacket? Packet { get; init; }
}

/// <summary>
/// Synchronous job handler delegate.
/// </summary>
public delegate JobResult HandlerDelegate(Context ctx, JobRequest request);

/// <summary>
/// Middleware wraps a handler, returning a new handler.
/// </summary>
public delegate HandlerDelegate MiddlewareDelegate(HandlerDelegate next);

/// <summary>
/// Middleware composition utilities.
/// </summary>
public static class MiddlewareChain
{
    /// <summary>
    /// Compose a chain of middlewares around a base handler.
    /// Middlewares execute in FIFO order (first added = outermost wrapper).
    /// </summary>
    public static HandlerDelegate ApplyMiddleware(HandlerDelegate handler, IReadOnlyList<MiddlewareDelegate> chain)
    {
        var result = handler;
        for (var i = chain.Count - 1; i >= 0; i--)
        {
            result = chain[i](result);
        }
        return result;
    }

    /// <summary>
    /// Middleware that logs job_id and duration for each request.
    /// </summary>
    public static MiddlewareDelegate LoggingMiddleware()
    {
        return next => (ctx, req) =>
        {
            var sw = Stopwatch.StartNew();
            var result = next(ctx, req);
            sw.Stop();
            Console.WriteLine($"[cap] job={req.JobId} duration={sw.ElapsedMilliseconds}ms");
            return result;
        };
    }
}
