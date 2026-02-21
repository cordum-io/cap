use std::time::Duration;

use crate::errors::CapError;

pub struct BusConfig {
    pub url: String,
    pub token: Option<String>,
    pub connect_timeout: Duration,
}

impl Default for BusConfig {
    fn default() -> Self {
        BusConfig {
            url: "nats://localhost:4222".into(),
            token: None,
            connect_timeout: Duration::from_secs(5),
        }
    }
}

pub async fn connect_nats(config: &BusConfig) -> Result<async_nats::Client, CapError> {
    let mut opts = async_nats::ConnectOptions::new();

    if let Some(ref token) = config.token {
        opts = opts.token(token.clone());
    }

    let client = opts
        .connect(&config.url)
        .await
        .map_err(|e| CapError::connection_lost(&format!("NATS connect failed: {}", e)))?;

    Ok(client)
}
