#[derive(Debug, Copy, Clone, Hash, Eq, PartialEq)]
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
            // Method::CONNECT => axum::http::Method::CONNECT,
            Method::OPTIONS => axum::http::Method::OPTIONS,
            Method::TRACE => axum::http::Method::TRACE,
            Method::PATCH => axum::http::Method::PATCH,
        }
    }
}

pub(super) fn method_filter<M>(methods: M) -> Option<axum::routing::MethodFilter>
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
