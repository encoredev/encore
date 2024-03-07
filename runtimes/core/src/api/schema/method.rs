#[derive(Debug, Copy, Clone, Hash, Eq, PartialEq, Ord, PartialOrd)]
pub enum Method {
    GET,
    HEAD,
    POST,
    PUT,
    DELETE,
    // CONNECT,
    OPTIONS,
    TRACE,
    PATCH,
}

impl Method {
    pub fn as_str(self) -> &'static str {
        match self {
            Method::GET => "GET",
            Method::HEAD => "HEAD",
            Method::POST => "POST",
            Method::PUT => "PUT",
            Method::DELETE => "DELETE",
            // Method::CONNECT => "CONNECT",
            Method::OPTIONS => "OPTIONS",
            Method::TRACE => "TRACE",
            Method::PATCH => "PATCH",
        }
    }

    /// Whether the method supports a request body.
    pub fn supports_body(&self) -> bool {
        match self {
            Self::POST | Self::PUT | Self::PATCH /* | Self::CONNECT */ => true,
            Self::GET | Self::DELETE | Self::HEAD | Self::OPTIONS | Self::TRACE => false,
        }
    }
}

impl TryFrom<&str> for Method {
    type Error = anyhow::Error;

    fn try_from(s: &str) -> Result<Self, Self::Error> {
        match s {
            "GET" => Ok(Method::GET),
            "HEAD" => Ok(Method::HEAD),
            "POST" => Ok(Method::POST),
            "PUT" => Ok(Method::PUT),
            "DELETE" => Ok(Method::DELETE),
            // "CONNECT" => Ok(Method::CONNECT),
            "OPTIONS" => Ok(Method::OPTIONS),
            "TRACE" => Ok(Method::TRACE),
            "PATCH" => Ok(Method::PATCH),
            _ => Err(anyhow::anyhow!("invalid method: {}", s)),
        }
    }
}

impl Into<axum::http::Method> for Method {
    fn into(self) -> axum::http::Method {
        match self {
            Method::GET => axum::http::Method::GET,
            Method::HEAD => axum::http::Method::HEAD,
            Method::POST => axum::http::Method::POST,
            Method::PUT => axum::http::Method::PUT,
            Method::DELETE => axum::http::Method::DELETE,
            // Method::CONNECT => http::Method::CONNECT,
            Method::OPTIONS => axum::http::Method::OPTIONS,
            Method::TRACE => axum::http::Method::TRACE,
            Method::PATCH => axum::http::Method::PATCH,
        }
    }
}

impl Into<reqwest::Method> for Method {
    fn into(self) -> reqwest::Method {
        match self {
            Method::GET => reqwest::Method::GET,
            Method::HEAD => reqwest::Method::HEAD,
            Method::POST => reqwest::Method::POST,
            Method::PUT => reqwest::Method::PUT,
            Method::DELETE => reqwest::Method::DELETE,
            // Method::CONNECT => reqwest::Method::CONNECT,
            Method::OPTIONS => reqwest::Method::OPTIONS,
            Method::TRACE => reqwest::Method::TRACE,
            Method::PATCH => reqwest::Method::PATCH,
        }
    }
}

impl TryFrom<axum::http::Method> for Method {
    type Error = anyhow::Error;
    fn try_from(m: axum::http::Method) -> anyhow::Result<Self> {
        Ok(match m.as_str() {
            "GET" => Method::GET,
            "HEAD" => Method::HEAD,
            "POST" => Method::POST,
            "PUT" => Method::PUT,
            "DELETE" => Method::DELETE,
            // "CONNECT" => Method::CONNECT,
            "OPTIONS" => Method::OPTIONS,
            "TRACE" => Method::TRACE,
            "PATCH" => Method::PATCH,
            x => anyhow::bail!("invalid method: {}", x),
        })
    }
}

pub fn method_filter<M>(methods: M) -> Option<axum::routing::MethodFilter>
where
    M: Iterator<Item = Method>,
{
    use axum::routing::MethodFilter;
    let mut filter = None;
    for method in methods {
        let method_filter = match method {
            Method::GET => MethodFilter::GET,
            Method::HEAD => MethodFilter::HEAD,
            Method::POST => MethodFilter::POST,
            Method::PUT => MethodFilter::PUT,
            Method::DELETE => MethodFilter::DELETE,
            // Method::CONNECT => MethodFilter::CONNECT,
            Method::OPTIONS => MethodFilter::OPTIONS,
            Method::TRACE => MethodFilter::TRACE,
            Method::PATCH => MethodFilter::PATCH,
        };
        filter = Some(filter.unwrap_or(method_filter).or(method_filter));
    }
    filter
}
