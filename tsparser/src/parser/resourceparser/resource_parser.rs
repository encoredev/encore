use std::collections::{HashMap, HashSet};

use swc_ecma_ast as ast;

use crate::parser::module_loader::Module;
use crate::parser::resourceparser::paths::PkgPath;
use crate::parser::resourceparser::ResourceParseContext;
use crate::parser::resources::DEFAULT_RESOURCE_PARSERS;

/// A parser for a specific resource type.
#[derive(Debug)]
pub struct ResourceParser {
    pub name: &'static str,

    pub interesting_pkgs: &'static [PkgPath<'static>],

    pub run: fn(&mut ResourceParseContext) -> anyhow::Result<()>,
}

impl PartialEq for ResourceParser {
    fn eq(&self, other: &Self) -> bool {
        self.name == other.name
    }
}

#[derive(Debug)]
pub struct ResourceParserRegistry<'a> {
    /// List of registered parsers.
    parsers: Vec<&'a ResourceParser>,
    interested_for_paths: HashMap<PkgPath<'a>, Vec<&'a ResourceParser>>,
}

impl<'a> Default for ResourceParserRegistry<'a> {
    fn default() -> Self {
        Self::new(DEFAULT_RESOURCE_PARSERS)
    }
}

impl<'a> ResourceParserRegistry<'a> {
    pub fn new(parsers: &[&'a ResourceParser]) -> Self {
        let mut registry = Self {
            parsers: Vec::new(),
            interested_for_paths: HashMap::new(),
        };
        for p in parsers {
            registry.register(p);
        }
        registry
    }

    /// Register a new parser.
    pub fn register(&mut self, parser: &'a ResourceParser) {
        self.parsers.push(parser);
        for path in parser.interesting_pkgs {
            self.interested_for_paths
                .entry(*path)
                .or_insert_with(|| Vec::new())
                .push(parser);
        }
    }

    /// Compute the parsers interested in processing the given module.
    pub fn interested_parsers(&self, module: &Module) -> Vec<&'a ResourceParser> {
        let mut parsers = Vec::new();
        let mut seen_parsers = HashSet::new();

        // Iterate over the imports in this module and collect all the interested parsers.
        for it in &module.ast.body {
            if let ast::ModuleItem::ModuleDecl(ast::ModuleDecl::Import(import)) = it {
                let path = PkgPath(&import.src.value);
                self.interested_for_paths.get(&path).map(|found| {
                    for p in found {
                        // If we haven't already seen the parser with that name,
                        // add it to the list of parsers to run.
                        if !seen_parsers.contains(p.name) {
                            seen_parsers.insert(p.name);
                            parsers.push(*p);
                        }
                    }
                });
            }
        }

        parsers
    }
}

#[cfg(test)]
mod tests {
    

    
    use crate::parser::resources::apis::api::ENDPOINT_PARSER;
    
    use crate::testutil::testparse::test_parse;

    use super::*;

    #[test]
    fn test_parser_registry() {
        let registry = ResourceParserRegistry::new(&[&ENDPOINT_PARSER]);

        // Should return the parser when the import path matches.
        {
            let res = test_parse("import { APIEndpoint } from 'encore.dev/api';");
            let got = registry.interested_parsers(&res);
            let want = vec![&ENDPOINT_PARSER];
            assert_eq!(got, want);
        }

        // Should also work for wildcard imports.
        {
            let res = test_parse("import * as foo from 'encore.dev/api';");
            let got = registry.interested_parsers(&res);
            let want = vec![&ENDPOINT_PARSER];
            assert_eq!(got, want);
        }

        // Should be empty otherwise.
        {
            let res = test_parse("import { APIEndpoint } from 'encore.dev/api2';");
            let got = registry.interested_parsers(&res);
            let want: Vec<&ResourceParser> = vec![];
            assert_eq!(got, want);
        }
    }
}
