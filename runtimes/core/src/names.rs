use anyhow::Context;
use std::borrow::Borrow;
use std::fmt::Display;
use std::hash::Hash;
use std::ops::Deref;

#[derive(Debug, Clone, Eq, Hash, PartialEq)]
pub struct EncoreName(String);

impl Deref for EncoreName {
    type Target = str;
    fn deref(&self) -> &str {
        &self.0
    }
}

impl AsRef<str> for EncoreName {
    fn as_ref(&self) -> &str {
        &self.0
    }
}

impl From<String> for EncoreName {
    fn from(value: String) -> Self {
        Self(value)
    }
}

impl From<&str> for EncoreName {
    fn from(value: &str) -> Self {
        Self(value.to_string())
    }
}

impl From<&String> for EncoreName {
    fn from(value: &String) -> Self {
        Self(value.clone())
    }
}

impl Borrow<str> for EncoreName {
    fn borrow(&self) -> &str {
        &self.0
    }
}
impl Borrow<str> for &EncoreName {
    fn borrow(&self) -> &str {
        &self.0
    }
}

impl Borrow<String> for EncoreName {
    fn borrow(&self) -> &String {
        &self.0
    }
}
impl Borrow<String> for &EncoreName {
    fn borrow(&self) -> &String {
        &self.0
    }
}

impl Display for EncoreName {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.0)
    }
}

#[derive(Debug, Clone, Eq, Hash, PartialEq)]
pub struct CloudName(String);

impl Deref for CloudName {
    type Target = str;
    fn deref(&self) -> &str {
        &self.0
    }
}

impl Display for CloudName {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.0)
    }
}

impl From<String> for CloudName {
    fn from(value: String) -> Self {
        Self(value)
    }
}

#[derive(Debug, Clone)]
pub struct EndpointName {
    /// The full name ("service.endpoint")
    name: String,

    /// Cached length of the service name.
    service_len: usize,
}

impl Hash for EndpointName {
    fn hash<H: std::hash::Hasher>(&self, state: &mut H) {
        self.name.hash(state)
    }
}

impl PartialEq for EndpointName {
    fn eq(&self, other: &Self) -> bool {
        self.name == other.name
    }
}

impl Eq for EndpointName {}

impl Deref for EndpointName {
    type Target = str;

    fn deref(&self) -> &str {
        &self.name
    }
}

impl Display for EndpointName {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.name)
    }
}

impl EndpointName {
    pub fn new<S: Into<String>>(service: S, endpoint: S) -> Self {
        let mut name = service.into();
        let service_len = name.len();
        name.push('.');
        name.push_str(&endpoint.into());

        Self { name, service_len }
    }

    pub fn service(&self) -> &str {
        &self.name[..self.service_len]
    }

    pub fn endpoint(&self) -> &str {
        &self.name[self.service_len + 1..]
    }
}

impl TryFrom<String> for EndpointName {
    type Error = anyhow::Error;

    fn try_from(value: String) -> Result<Self, Self::Error> {
        // Find the '.'.
        let idx = value.find('.').context("missing '.'")?;
        if idx == 0 {
            anyhow::bail!("missing service name");
        } else if idx == value.len() - 1 {
            anyhow::bail!("missing endpoint name");
        }

        Ok(Self {
            name: value,
            service_len: idx,
        })
    }
}
