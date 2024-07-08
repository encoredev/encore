use std::sync::Arc;

use crate::{
    api::{self, schema::Method},
    EncoreName,
};

#[derive(Clone)]
pub struct Router {
    inner: matchit::Router<MethodRoute>,
}

impl Router {
    pub fn new() -> Self {
        let inner = matchit::Router::new();
        Router { inner }
    }

    pub fn add_routes(
        &mut self,
        service: &EncoreName,
        routes: &Vec<(Arc<api::Endpoint>, Vec<String>)>,
    ) -> anyhow::Result<()> {
        for (endpoint, paths) in routes {
            for path in paths {
                let method_route = match self.inner.at_mut(path) {
                    Ok(m) => m.value,
                    Err(_) => {
                        self.inner.insert(path, MethodRoute::default())?;
                        self.inner.at_mut(path).unwrap().value
                    }
                };

                for method in endpoint.methods() {
                    let dst = match method {
                        Method::GET => &mut method_route.get,
                        Method::HEAD => &mut method_route.head,
                        Method::POST => &mut method_route.post,
                        Method::PUT => &mut method_route.put,
                        Method::DELETE => &mut method_route.delete,
                        Method::OPTIONS => &mut method_route.option,
                        Method::TRACE => &mut method_route.trace,
                        Method::PATCH => &mut method_route.patch,
                    };
                    if dst.is_some() {
                        ::log::error!(method = method.as_str(), path = path; "tried to register same route twice, skipping");
                        continue;
                    }
                    dst.replace(service.clone());
                }
            }
        }

        Ok(())
    }

    pub fn route_to_service(
        &self,
        method: api::schema::Method,
        path: &str,
    ) -> Result<&EncoreName, api::Error> {
        let matched_route = self.inner.at(path).map_err(|_| api::Error {
            code: api::ErrCode::NotFound,
            message: "endpoint not found".to_string(),
            internal_message: Some(format!("no such endpoint exists: {}", path)),
            stack: None,
        })?;

        // returna pi error for method not found
        let service = matched_route
            .value
            .for_method(method)
            .ok_or_else(|| api::Error {
                code: api::ErrCode::NotFound,
                message: "no route for method".to_string(),
                internal_message: Some(format!("no route for method {:?}: {}", method, path)),
                stack: None,
            })?;

        Ok(service)
    }
}

#[derive(Clone, Default)]
pub struct MethodRoute {
    get: Option<EncoreName>,
    head: Option<EncoreName>,
    post: Option<EncoreName>,
    put: Option<EncoreName>,
    delete: Option<EncoreName>,
    option: Option<EncoreName>,
    trace: Option<EncoreName>,
    patch: Option<EncoreName>,
}

impl MethodRoute {
    fn for_method(&self, method: api::schema::Method) -> Option<&EncoreName> {
        match method {
            Method::GET => self.get.as_ref(),
            Method::HEAD => self.head.as_ref(),
            Method::POST => self.post.as_ref(),
            Method::PUT => self.put.as_ref(),
            Method::DELETE => self.delete.as_ref(),
            Method::OPTIONS => self.option.as_ref(),
            Method::TRACE => self.trace.as_ref(),
            Method::PATCH => self.patch.as_ref(),
        }
    }
}
