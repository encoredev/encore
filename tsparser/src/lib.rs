pub mod app;
pub mod builder;
mod legacymeta;
pub mod parser;

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

mod runtimeresolve;
#[cfg(test)]
pub mod testutil;
