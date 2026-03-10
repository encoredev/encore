pub mod app;
#[cfg(feature = "native")]
pub mod builder;
pub mod exports;
mod legacymeta;
pub mod parser;
pub mod resolve_utils;
mod span_err;
pub mod tsconfig;

pub mod encore {
    pub mod parser {
        pub mod meta {
            pub mod v1 {
                include!(concat!(env!("OUT_DIR"), "/encore.parser.meta.v1.rs"));
            }
        }

        pub mod schema {
            pub mod v1 {
                include!(concat!(env!("OUT_DIR"), "/encore.parser.schema.v1.rs"));
            }
        }
    }
}

#[cfg(feature = "native")]
mod runtimeresolve;
#[cfg(test)]
pub mod testutil;
