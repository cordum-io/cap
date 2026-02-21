using NATS.Client.Core;

namespace Cordum.Cap;

/// <summary>
/// NATS connection configuration.
/// </summary>
public sealed class BusConfig
{
    public string Url { get; set; } = "nats://localhost:4222";
    public string? AuthToken { get; set; }
    public TimeSpan ConnectTimeout { get; set; } = TimeSpan.FromSeconds(5);
}

/// <summary>
/// NATS connection helper.
/// </summary>
public static class Bus
{
    /// <summary>
    /// Connect to NATS using the provided configuration.
    /// </summary>
    public static async Task<NatsConnection> ConnectNats(BusConfig config)
    {
        var opts = new NatsOpts
        {
            Url = config.Url,
            ConnectTimeout = config.ConnectTimeout,
            AuthOpts = config.AuthToken != null
                ? NatsAuthOpts.Default with { Token = config.AuthToken }
                : NatsAuthOpts.Default,
        };

        var connection = new NatsConnection(opts);
        await connection.ConnectAsync();
        return connection;
    }
}
