use std::str::FromStr;

#[derive(Debug, Copy, Clone, Hash, Eq, PartialEq)]
pub enum MetaKey {
    TraceParent,
    TraceState,
    XCorrelationId,
    Version,
    UserId,
    UserData,
    Caller,
    Callee,
    SvcAuthMethod,
    SvcAuthEncoreAuthHash,
    SvcAuthEncoreAuthDate,
}

impl MetaKey {
    pub fn header_key(&self) -> &'static str {
        use MetaKey::*;
        match self {
            TraceParent => "traceparent",
            TraceState => "tracestate",
            XCorrelationId => "x-correlation-id",
            Version => "x-encore-meta-version",
            UserId => "x-encore-meta-userid",
            UserData => "x-encore-meta-authdata",
            Caller => "x-encore-meta-caller",
            Callee => "x-encore-meta-callee",
            SvcAuthMethod => "x-encore-meta-svc-auth-method",
            SvcAuthEncoreAuthHash => "x-encore-meta-svc-auth",
            SvcAuthEncoreAuthDate => "x-encore-meta-date",
        }
    }
}

pub struct NotMetaKey;

impl FromStr for MetaKey {
    type Err = NotMetaKey;

    fn from_str(value: &str) -> Result<Self, Self::Err> {
        use MetaKey::*;
        Ok(match value {
            "traceparent" => TraceParent,
            "tracestate" => TraceState,
            "x-correlation-id" => XCorrelationId,
            "x-encore-meta-version" => Version,
            "x-encore-meta-userid" => UserId,
            "x-encore-meta-authdata" => UserData,
            "x-encore-meta-caller" => Caller,
            "x-encore-meta-callee" => Callee,
            "x-encore-meta-svc-auth-method" => SvcAuthMethod,
            "x-encore-meta-svc-auth" => SvcAuthEncoreAuthHash,
            "x-encore-meta-date" => SvcAuthEncoreAuthDate,
            _ => return Err(NotMetaKey),
        })
    }
}

pub trait MetaMapMut: MetaMap {
    fn set(&mut self, key: MetaKey, value: String) -> anyhow::Result<()>;
}

pub trait MetaMap {
    fn get_meta(&self, key: MetaKey) -> Option<&str>;
    fn meta_values<'a>(&'a self, key: MetaKey) -> Box<dyn Iterator<Item = &'a str> + 'a>;

    /// Returns all meta keys, sorted alphabetically based on MetaKey::header_key.
    fn sorted_meta_keys(&self) -> Vec<MetaKey>;
}

impl MetaMap for reqwest::header::HeaderMap {
    fn get_meta(&self, key: MetaKey) -> Option<&str> {
        self.get(key.header_key()).and_then(|v| v.to_str().ok())
    }

    fn meta_values<'a>(&'a self, key: MetaKey) -> Box<dyn Iterator<Item = &'a str> + 'a> {
        Box::new(
            self.get_all(key.header_key())
                .iter()
                .filter_map(|v| v.to_str().ok()),
        )
    }

    fn sorted_meta_keys(&self) -> Vec<MetaKey> {
        let mut keys: Vec<_> = self
            .keys()
            .filter_map(|k| MetaKey::from_str(k.as_str()).ok())
            .collect();
        keys.sort_by_key(|k| k.header_key());
        keys
    }
}

impl MetaMapMut for reqwest::header::HeaderMap {
    fn set(&mut self, key: MetaKey, value: String) -> anyhow::Result<()> {
        self.insert(key.header_key(), value.parse()?);
        Ok(())
    }
}

impl MetaMap for axum::http::HeaderMap {
    fn get_meta(&self, key: MetaKey) -> Option<&str> {
        self.get(key.header_key()).and_then(|v| v.to_str().ok())
    }

    fn meta_values<'a>(&'a self, key: MetaKey) -> Box<dyn Iterator<Item = &'a str> + 'a> {
        Box::new(
            self.get_all(key.header_key())
                .iter()
                .filter_map(|v| v.to_str().ok()),
        )
    }

    fn sorted_meta_keys(&self) -> Vec<MetaKey> {
        let mut keys: Vec<_> = self
            .keys()
            .filter_map(|k| MetaKey::from_str(k.as_str()).ok())
            .collect();
        keys.sort_by_key(|k| k.header_key());
        keys
    }
}
