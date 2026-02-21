pub mod subjects;
pub mod codec;
pub mod signing;
pub mod errors;
pub mod validate;
pub mod metrics;
pub mod bus;
pub mod client;
pub mod worker;
pub mod heartbeat;
pub mod progress;
pub mod middleware;
pub mod agent;
pub mod testing;

pub mod proto {
    pub mod cordum {
        pub mod agent {
            pub mod v1 {
                include!(concat!(env!("OUT_DIR"), "/cordum.agent.v1.rs"));
            }
        }
    }
}

pub use proto::cordum::agent::v1 as pb;
