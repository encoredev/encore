use std::sync::Arc;

use crate::{
    api::{self, paths::PathSet, schema::Method},
    EncoreName,
};

#[derive(Clone)]
pub struct Router {
    main: matchit::Router<MethodRoute>,
    fallback: matchit::Router<MethodRoute>,
}

impl Router {
    pub fn new() -> Self {
        let main = matchit::Router::new();
        let fallback = matchit::Router::new();
        Router { main, fallback }
    }

    pub fn add_routes(
        &mut self,
        routes: &PathSet<EncoreName, Arc<api::Endpoint>>,
    ) -> anyhow::Result<()> {
        for (router, routes) in [
            (&mut self.main, &routes.main),
            (&mut self.fallback, &routes.fallback),
        ] {
            fn register_methods(
                mr: &mut MethodRoute,
                path: &str,
                service: &EncoreName,
                endpoint: &api::Endpoint,
            ) {
                for method in endpoint.methods() {
                    let dst = match method {
                        Method::GET => &mut mr.get,
                        Method::HEAD => &mut mr.head,
                        Method::POST => &mut mr.post,
                        Method::PUT => &mut mr.put,
                        Method::DELETE => &mut mr.delete,
                        Method::OPTIONS => &mut mr.option,
                        Method::TRACE => &mut mr.trace,
                        Method::PATCH => &mut mr.patch,
                    };
                    log::trace!(path = path, method = method.as_str(); "registering route");
                    if dst.is_some() {
                        ::log::error!(method = method.as_str(), path = path; "tried to register same route twice, skipping");
                        continue;
                    }
                    dst.replace(Target {
                        service_name: service.clone(),
                        requires_auth: endpoint.requires_auth,
                    });
                }
            }

            for (service, routes) in routes {
                for (endpoint, paths) in routes {
                    for path in paths {
                        // Create a method route where we register each method the endpoint supports.
                        let mut mr = MethodRoute::default();
                        register_methods(&mut mr, path, service, endpoint);
                        match router.insert(path, mr) {
                            Ok(()) => {}
                            Err(matchit::InsertError::Conflict { .. }) => {
                                // If we have a conflict, we need to merge the method routes.
                                let mr = router.at_mut(path).unwrap().value;
                                register_methods(mr, path, service, endpoint);
                            }
                            Err(e) => return Err(e.into()),
                        }
                    }
                }
            }
        }

        Ok(())
    }

    pub fn route_to_service(
        &self,
        method: api::schema::Method,
        path: &str,
    ) -> Result<&Target, api::Error> {
        let mut found_path_match = false;
        for router in [&self.main, &self.fallback] {
            if let Ok(service) = router.at(path) {
                found_path_match = true;
                let service = service.value.for_method(method);
                if let Some(service) = service {
                    return Ok(service);
                }
            }
        }

        // We couldn't find a matching route.
        Err(if found_path_match {
            api::Error {
                code: api::ErrCode::NotFound,
                message: "no route for method".to_string(),
                internal_message: Some(format!("no route for method {:?}: {}", method, path)),
                stack: None,
                details: None,
            }
        } else {
            api::Error {
                code: api::ErrCode::NotFound,
                message: "endpoint not found".to_string(),
                internal_message: Some(format!("no such endpoint exists: {}", path)),
                stack: None,
                details: None,
            }
        })
    }
}

#[derive(Clone, Debug)]
pub struct Target {
    pub service_name: EncoreName,
    pub requires_auth: bool,
}

#[derive(Clone, Default)]
pub struct MethodRoute {
    get: Option<Target>,
    head: Option<Target>,
    post: Option<Target>,
    put: Option<Target>,
    delete: Option<Target>,
    option: Option<Target>,
    trace: Option<Target>,
    patch: Option<Target>,
}

impl MethodRoute {
    fn for_method(&self, method: api::schema::Method) -> Option<&Target> {
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
