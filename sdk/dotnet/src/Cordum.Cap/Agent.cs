using System.Collections.Concurrent;
using System.Security.Cryptography;
using Google.Protobuf.WellKnownTypes;
using NATS.Client.Core;
using Cordum.Agent.V1;

namespace Cordum.Cap;

/// <summary>
/// Builder for configuring and constructing an Agent.
/// </summary>
public sealed class AgentBuilder
{
    private string _natsUrl = "nats://localhost:4222";
    private NatsConnection? _nc;
    private string _senderId;
    private string _pool = "";
    private int _maxParallel = 1;
    private ECDsa? _privateKey;
    private readonly Dictionary<string, ECDsa> _publicKeys = new();
    private IMetricsHook _metrics = new NoopMetrics();
    private readonly List<MiddlewareDelegate> _middlewares = new();
    private TimeSpan _heartbeatInterval = Subjects.DefaultHeartbeatInterval;

    public AgentBuilder(string senderId)
    {
        _senderId = senderId;
    }

    public AgentBuilder NatsUrl(string url) { _natsUrl = url; return this; }
    public AgentBuilder Connection(NatsConnection nc) { _nc = nc; return this; }
    public AgentBuilder Pool(string pool) { _pool = pool; return this; }
    public AgentBuilder MaxParallel(int n) { _maxParallel = n; return this; }
    public AgentBuilder PrivateKey(ECDsa key) { _privateKey = key; return this; }
    public AgentBuilder PublicKeys(Dictionary<string, ECDsa> keys) { foreach (var kv in keys) _publicKeys[kv.Key] = kv.Value; return this; }
    public AgentBuilder Metrics(IMetricsHook metrics) { _metrics = metrics; return this; }
    public AgentBuilder Middleware(MiddlewareDelegate mw) { _middlewares.Add(mw); return this; }
    public AgentBuilder HeartbeatInterval(TimeSpan interval) { _heartbeatInterval = interval; return this; }

    /// <summary>
    /// Build and connect the agent.
    /// </summary>
    public async Task<Agent> BuildAsync()
    {
        var nc = _nc ?? await Bus.ConnectNats(new BusConfig { Url = _natsUrl });

        return new Agent(
            nc, _senderId, _pool, _maxParallel,
            _privateKey, _publicKeys, _metrics,
            _middlewares, _heartbeatInterval);
    }
}

/// <summary>
/// High-level CAP agent runtime. Manages handler registration, job dispatch,
/// and heartbeat emission. Implements IAsyncDisposable for clean shutdown.
/// </summary>
public sealed class Agent : IAsyncDisposable
{
    private readonly NatsConnection _nc;
    private readonly string _senderId;
    private readonly string _pool;
    private readonly int _maxParallel;
    private readonly ECDsa? _privateKey;
    private readonly Dictionary<string, ECDsa> _publicKeys;
    private readonly IMetricsHook _metrics;
    private readonly List<MiddlewareDelegate> _middlewares;
    private readonly TimeSpan _heartbeatInterval;
    private readonly Dictionary<string, HandlerDelegate> _handlers = new();
    private HeartbeatLoop? _heartbeatLoop;
    private int _activeJobs;

    internal Agent(
        NatsConnection nc,
        string senderId,
        string pool,
        int maxParallel,
        ECDsa? privateKey,
        Dictionary<string, ECDsa> publicKeys,
        IMetricsHook metrics,
        List<MiddlewareDelegate> middlewares,
        TimeSpan heartbeatInterval)
    {
        _nc = nc;
        _senderId = senderId;
        _pool = pool;
        _maxParallel = maxParallel;
        _privateKey = privateKey;
        _publicKeys = publicKeys;
        _metrics = metrics;
        _middlewares = middlewares;
        _heartbeatInterval = heartbeatInterval;
    }

    /// <summary>
    /// Register a handler for a subject.
    /// </summary>
    public void Register(string subject, HandlerDelegate handler)
    {
        _handlers[subject] = handler;
    }

    /// <summary>
    /// Start all workers and the heartbeat loop.
    /// </summary>
    public async Task StartAsync(CancellationToken ct = default)
    {
        // Apply middleware to all handlers first
        var wrappedHandlers = new Dictionary<string, HandlerDelegate>();
        foreach (var (subject, handler) in _handlers)
        {
            wrappedHandlers[subject] = MiddlewareChain.ApplyMiddleware(handler, _middlewares);
        }

        // Start subscription tasks for each handler
        foreach (var (subject, wrapped) in wrappedHandlers)
        {
            var worker = new Worker(
                _nc, subject, _senderId, wrapped,
                _privateKey, _publicKeys, _metrics);
            _ = Task.Run(() => worker.StartAsync(ct), ct);
        }

        // Start heartbeat loop
        _heartbeatLoop = new HeartbeatLoop(
            _nc,
            () => HeartbeatHelper.HeartbeatPayload(
                _senderId, _pool,
                Interlocked.CompareExchange(ref _activeJobs, 0, 0),
                _maxParallel, 0f),
            _senderId,
            _heartbeatInterval,
            _metrics);
        _heartbeatLoop.Start();
    }

    /// <summary>
    /// Stop heartbeat and clean up resources.
    /// </summary>
    public async ValueTask DisposeAsync()
    {
        if (_heartbeatLoop != null)
        {
            await _heartbeatLoop.DisposeAsync();
        }
    }
}
