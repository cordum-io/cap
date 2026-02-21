package io.cordum.cap;

import io.nats.client.Connection;
import io.nats.client.Nats;
import io.nats.client.Options;

import java.io.IOException;
import java.time.Duration;

public final class Bus {

    private Bus() {}

    public static final class Config {
        private String url = "nats://localhost:4222";
        private String token;
        private String username;
        private String password;
        private Duration connectTimeout = Duration.ofSeconds(5);

        public Config url(String url) { this.url = url; return this; }
        public Config token(String token) { this.token = token; return this; }
        public Config credentials(String username, String password) {
            this.username = username;
            this.password = password;
            return this;
        }
        public Config connectTimeout(Duration timeout) { this.connectTimeout = timeout; return this; }
    }

    public static Connection connect(Config config) throws IOException, InterruptedException {
        Options.Builder builder = new Options.Builder()
                .server(config.url)
                .connectionTimeout(config.connectTimeout);

        if (config.token != null && !config.token.isEmpty()) {
            builder.token(config.token.toCharArray());
        }
        if (config.username != null && config.password != null) {
            builder.userInfo(config.username, config.password);
        }

        return Nats.connect(builder.build());
    }
}
