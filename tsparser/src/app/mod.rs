use anyhow::Result;

use crate::encore::parser::meta::v1;
use crate::legacymeta::compute_meta;
use crate::parser::parser::{ParseContext, ParseResult};

#[derive(Debug)]
pub struct AppDesc {
    pub parse: ParseResult,
    pub meta: v1::Data,
}

pub fn validate_and_describe(pc: &ParseContext, parse: ParseResult) -> Result<AppDesc> {
    let meta = compute_meta(pc, &parse)?;
    Ok(AppDesc { parse, meta })
}
