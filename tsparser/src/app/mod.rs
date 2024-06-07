use std::path::{Path, PathBuf};

use anyhow::Result;
use swc_common::sync::Lrc;

use crate::app::service_discovery::{discover_services, DiscoveredService};
use crate::encore::parser::meta::v1;
use crate::legacymeta::compute_meta;
use crate::parser::parser::{ParseContext, ParseResult};
use crate::parser::resourceparser::bind::Bind;
use crate::parser::{FilePath, FileSet};

pub mod service_discovery;

#[derive(Debug)]
pub struct AppDesc {
    pub services: Vec<Service>,
    pub meta: v1::Data,
}

/// Describes an Encore service.
#[derive(Debug)]
pub struct Service {
    pub name: String,

    /// The root directory of the service.
    // TODO change
    pub root: PathBuf,

    /// The binds defined within the service.
    pub binds: Vec<Lrc<Bind>>,
}

pub fn validate_and_describe(pc: &ParseContext, parse: &ParseResult) -> Result<AppDesc> {
    let disco = discover_services(&pc.file_set, &parse.binds)?;
    let services = collect_services(&pc.file_set, parse, disco)?;
    let meta = compute_meta(pc, parse, &services)?;
    Ok(AppDesc { services, meta })
}

pub fn collect_services<'a>(
    file_set: &'a FileSet,
    parse: &'a ParseResult,
    discovered: Vec<DiscoveredService>,
) -> Result<Vec<Service>> {
    let mut services = Vec::with_capacity(discovered.len());
    for svc in discovered.into_iter() {
        services.push(Service {
            name: svc.name,
            root: svc.root,
            binds: vec![],
        });
    }

    // Sort the services by path so we can do a binary search below.
    services.sort_by(|a, b| a.root.cmp(&b.root));

    for b in &parse.binds {
        let Some(range) = b.range else { continue };
        let file_path = range.file(file_set);
        let path: &Path = match file_path {
            FilePath::Real(ref buf) => buf.as_path(),
            FilePath::Custom(_) => continue,
        };

        // found is where the bind would be inserted:
        // - Ok(idx) means an exact path match
        // - Err(idx) means where the path would be inserted.
        //   For this case we need to check if the path is a subdirectory of the service root.
        let found = services.binary_search_by_key(&path, |s| s.root.as_path());
        match found {
            Ok(idx) => services[idx].binds.push(b.clone()),
            Err(idx) => {
                // Is this path a subdirectory of the preceding service root?
                if idx > 0 {
                    let svc = &mut services[idx - 1];
                    if path.starts_with(&svc.root) {
                        svc.binds.push(b.clone());
                        continue;
                    }
                }
            }
        }
    }

    Ok(services)
}
