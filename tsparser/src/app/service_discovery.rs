use std::collections::{HashMap, HashSet};
use std::path::PathBuf;

use anyhow::Result;
use swc_common::sync::Lrc;

use crate::parser::resourceparser::bind::Bind;
use crate::parser::resources::Resource;
use crate::parser::{FilePath, FileSet};

/// Discover the services in an Encore application, based on the parsed resources.
pub fn discover_services<'a>(
    file_set: &'a FileSet,
    binds: &'a Vec<Lrc<Bind>>,
) -> Result<Vec<DiscoveredService>> {
    let sd = ServiceDiscoverer {
        file_set,
        binds,
        services: HashMap::new(),
        strong_root: HashSet::new(),
    };
    sd.discover()
}

#[derive(Debug, PartialEq, Eq)]
pub struct DiscoveredService {
    pub name: String,
    pub root: PathBuf,
}

struct ServiceDiscoverer<'a> {
    // Inputs
    file_set: &'a FileSet,
    binds: &'a Vec<Lrc<Bind>>,

    // Outputs
    services: HashMap<PathBuf, DiscoveredService>,
    strong_root: HashSet<PathBuf>,
}

impl<'a> ServiceDiscoverer<'a> {
    fn discover(mut self) -> Result<Vec<DiscoveredService>> {
        for b in self.binds {
            match b.resource {
                Resource::APIEndpoint(_) => self.possible_service_root(b.as_ref(), false),
                Resource::PubSubSubscription(_) => self.possible_service_root(b.as_ref(), false),
                Resource::Gateway(_) => self.possible_service_root(b.as_ref(), true),
                Resource::AuthHandler(_) => self.possible_service_root(b.as_ref(), false),

                // TODO: Include service struct, auth handler
                _ => {}
            }
        }

        let mut svcs = self.services.into_values().collect::<Vec<_>>();

        // Validate the services.
        for (i, svc) in svcs.iter().enumerate() {
            for other in &svcs[i + 1..] {
                if svc.root.starts_with(&other.root) {
                    anyhow::bail!(
                        "service {} cannot be contained within service {}",
                        svc.name,
                        other.name,
                    )
                } else if other.root.starts_with(&svc.root) {
                    anyhow::bail!(
                        "service {} cannot be contained within service {}",
                        other.name,
                        svc.name,
                    )
                } else if svc.name == other.name {
                    anyhow::bail!("duplicate service name {}", svc.name)
                }
            }
        }

        // Sort the services by name for deterministic output.
        svcs.sort_by(|a, b| a.name.cmp(&b.name));

        Ok(svcs)
    }

    fn possible_service_root(&mut self, bind: &Bind, strong: bool) {
        let Some(range) = bind.range else {
            return;
        };
        let file = range.file(self.file_set);
        let file_path = match file {
            FilePath::Real(ref buf) => buf,
            FilePath::Custom(_) => return,
        };
        let Some(root) = file_path.parent().map(|p| p.to_path_buf()) else {
            return;
        };

        // Ensure we have a valid service name.
        let dir_name = root
            .file_name()
            .and_then(|x| x.to_str())
            .map(|x| x.to_string());
        let Some(service_name) = dir_name else { return };

        if strong {
            // Always mark the root as a strong root, even if it is already marked as a service.
            self.strong_root.insert(root.clone());
        }

        // If the service is already marked, we don't need to do anything.
        if self.services.contains_key(&root) {
            return;
        }

        // Loop over the existing services and remove any that are subdirectories of this one.
        // Also look for any existing services which are parents of this root.
        let mut to_delete = Vec::new();
        for existing_root in self.services.keys() {
            // If the existing service is a subdirectory of this one, we can remove it.
            if existing_root.starts_with(&root) {
                // If the existing service is a strong root, we can't merge it with this one.
                if self.strong_root.contains(existing_root) {
                    continue;
                } else {
                    // The existing service is a descendant of this one,
                    // so remove it in favor of this one.
                    to_delete.push(existing_root);
                }
            } else if root.starts_with(existing_root) && !strong {
                // The new service is a descendant of an existing service, and this is not
                // a strong root so we consider this to be part of the existing service,
                // so we're done.
                return;
            }
        }

        // The new service is not a descendant of any existing service, so add it.
        self.services.insert(
            root.clone(),
            DiscoveredService {
                name: service_name.to_string(),
                root,
            },
        );
    }
}

#[cfg(test)]
mod tests {
    use std::path::Path;
    use swc_common::{Globals, GLOBALS};
    use tempdir::TempDir;

    use crate::parser::parser::{ParseContext, Parser};
    use crate::parser::resourceparser::PassOneParser;
    use crate::testutil::testresolve::TestResolver;

    use super::*;

    fn parse(tmp_dir: &Path, src: &str) -> Result<Vec<DiscoveredService>> {
        let globals = Globals::new();
        GLOBALS.set(&globals, || {
            let ar = txtar::from_str(src);
            ar.materialize(tmp_dir)?;

            let base = PathBuf::from("/dummy");
            let resolver = Box::new(TestResolver::new(&base, &ar));
            let mut pc = ParseContext::with_resolver(resolver);
            pc.dir_roots.push(tmp_dir.to_path_buf());
            pc.loader.load_archive(&base, &ar)?;

            let pass1 = PassOneParser::new(
                pc.file_set.clone(),
                pc.type_checker.clone(),
                Default::default(),
            );
            let parser = Parser::new(&pc, pass1);
            let result = parser.parse()?;
            discover_services(&pc.file_set, &result.binds)
        })
    }

    #[test]
    fn test_api_endpoints() -> Result<()> {
        let tmp_dir = TempDir::new("tsparser-test")?;
        let svcs = parse(
            tmp_dir.path(),
            r#"
-- systemA/svc1/foo.ts --
import { api } from "encore.dev/api";

export const foo = api(
  { method: "POST" },
  async (): Promise<void> => {}
);

-- systemA/svc2/bar.ts --
import { api } from "encore.dev/api";

export const bar = api(
  { method: "POST" },
  async (): Promise<void> => {}
);

-- svc3/bar.ts --
import { api } from "encore.dev/api";

export const bar = api(
  { method: "POST" },
  async (): Promise<void> => {}
);
"#,
        )?;

        let tmp_root = tmp_dir.path();
        assert_eq!(svcs.len(), 3);
        assert_eq!(svcs[0].name, "svc1");
        assert_eq!(svcs[1].name, "svc2");
        assert_eq!(svcs[2].name, "svc3");
        assert_eq!(svcs[0].root, tmp_root.join("systemA/svc1"));
        assert_eq!(svcs[1].root, tmp_root.join("systemA/svc2"));
        assert_eq!(svcs[2].root, tmp_root.join("svc3"));
        Ok(())
    }
}
