#![deny(clippy::all)]

pub mod api;
mod gateway;
mod log;
mod meta;
mod napi_util;
pub mod pubsub;
mod raw_api;
mod request_meta;
pub mod runtime;
mod secret;
mod sqldb;
mod stream;
mod threadsafe_function;

#[cfg(not(target_env = "msvc"))]
use tikv_jemallocator::Jemalloc;

#[cfg(not(target_env = "msvc"))]
#[global_allocator]
static GLOBAL: Jemalloc = Jemalloc;
