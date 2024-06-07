use std::{collections::HashMap, str::FromStr, sync::Arc, time::Duration};

use anyhow::Context;
use jsonwebtoken::jwk::{Jwk, JwkSet, KeyAlgorithm};
use serde::Deserialize;

/// Implements a client that fetches and caches JWK sets from a URL.
#[derive(Debug)]
pub struct CachingClient {
    /// The cached JWK set, keyed by the URL.
    cached: tokio::sync::RwLock<HashMap<&'static str, CachedJwkSet>>,
}

impl CachingClient {
    pub fn new() -> Self {
        Self {
            cached: tokio::sync::RwLock::default(),
        }
    }

    /// Returns the cached JWK set, fetching it if necessary.
    pub async fn get(&self, url: &'static str) -> anyhow::Result<Arc<JwkSet>> {
        // Do we already have a cached JWK set that's still valid?
        if let Some(cached) = self.get_if_cached(url).await {
            return Ok(cached);
        }

        // Fetch the JWK set from the URL.
        let response = fetch(url).await?;
        let set = response.set.clone();

        // Update the cache.
        {
            let mut write_guard = self.cached.write().await;
            write_guard.insert(url, response);
        }

        Ok(set)
    }

    /// Reports the cached JWK set if it's still valid.
    async fn get_if_cached(&self, url: &str) -> Option<Arc<JwkSet>> {
        let read_guard = self.cached.read().await;
        if let Some(cached) = read_guard.get(url) {
            if cached.is_valid() {
                return Some(cached.set.clone());
            }
        }
        None
    }
}

#[derive(Debug)]
struct CachedJwkSet {
    /// The JWK set.
    set: Arc<JwkSet>,

    /// The time the JWK set cache expires.
    /// None if the response should not be cached.
    exp: Option<std::time::Instant>,
}

impl CachedJwkSet {
    pub fn is_valid(&self) -> bool {
        self.exp
            .map(|exp| exp > std::time::Instant::now())
            .unwrap_or(false)
    }
}

/// Fetches a JWK set from a URL and returns it.
async fn fetch(url: &str) -> anyhow::Result<CachedJwkSet> {
    // Fetch the JWK set from the URL.
    let response = reqwest::get(url).await?;

    // If the status is not 200, return an error.
    if !response.status().is_success() {
        return Err(anyhow::anyhow!(
            "failed to fetch JWK set: {}",
            response.status()
        ));
    }

    // Determine the expiration time from the cache headers.
    let exp = response_cache_exp_time(response.headers());
    let exp = exp.map(|dur| std::time::Instant::now() + dur);

    #[derive(Deserialize)]
    struct RawKeyList {
        keys: Vec<serde_json::Value>,
    }
    let key_list: RawKeyList = response.json().await.context("unable to parse key list")?;
    let keys = key_list
        .keys
        .into_iter()
        .filter_map(parse_key)
        .collect::<Vec<Jwk>>();

    Ok(CachedJwkSet {
        set: Arc::new(JwkSet { keys }),
        exp,
    })
}

fn parse_key(val: serde_json::Value) -> Option<Jwk> {
    if let serde_json::Value::Object(map) = &val {
        if let Some(serde_json::Value::String(alg)) = map.get("alg") {
            if KeyAlgorithm::from_str(alg).is_err() {
                return None;
            }
            // We have an algorithm that can be parsed, so parse the whole JWT.
            return serde_json::from_value(val).ok();
        }
    }

    None
}

fn response_cache_exp_time(resp: &reqwest::header::HeaderMap) -> Option<Duration> {
    let cache_control = resp.get("cache-control").and_then(|v| v.to_str().ok());
    let age = resp.get("age").and_then(|v| v.to_str().ok());

    cache_exp_time(cache_control, age)
}

fn cache_exp_time(
    cache_control_header: Option<&str>,
    age_header: Option<&str>,
) -> Option<Duration> {
    let mut max_age = None;
    if let Some(cache_control) = cache_control_header {
        let parts = cache_control.split(',');
        for part in parts {
            let directive = part.trim();
            if directive.starts_with("max-age=") {
                if let Some(eq_idx) = directive.find('=') {
                    let age_value = directive[eq_idx + 1..].trim();
                    if let Ok(seconds) = age_value.parse::<u64>() {
                        max_age = Some(Duration::from_secs(seconds));
                    }
                }
            }
        }
    }

    let mut age = Duration::from_secs(0);
    if let Some(age_header) = age_header {
        if let Ok(age_secs) = age_header.parse::<u64>() {
            age = Duration::from_secs(age_secs);
        }
    }

    max_age.and_then(|ma| if ma >= age { Some(ma - age) } else { None })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_cache_exp_time() {
        assert_eq!(cache_exp_time(None, None), None);
        assert_eq!(
            cache_exp_time(Some("max-age=60"), None),
            Some(Duration::from_secs(60))
        );
        assert_eq!(
            cache_exp_time(Some("max-age=60"), Some("30")),
            Some(Duration::from_secs(30))
        );
        assert_eq!(
            cache_exp_time(Some("max-age=60, stale-while-revalidate=30"), Some("30")),
            Some(Duration::from_secs(30))
        );

        // Test when max-age is below age.
        assert_eq!(cache_exp_time(Some("max-age=30"), Some("60")), None);
    }
}
