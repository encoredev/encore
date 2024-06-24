use std::path::Path;

use anyhow::Result;

use crate::app::{validate_and_describe, AppDesc};
use crate::parser::parser::{ParseContext, Parser};
use crate::parser::resourceparser::PassOneParser;

use super::{App, Builder};

#[derive(Debug, Clone)]
pub struct ParseParams<'a> {
    pub app: &'a App,
    pub pc: &'a ParseContext,
    pub working_dir: &'a Path,
    pub parse_tests: bool,
}

impl Builder<'_> {
    pub fn parse(&self, params: &ParseParams) -> Result<AppDesc> {
        let pc = params.pc;
        let pass1 = PassOneParser::new(
            pc.file_set.clone(),
            pc.type_checker.clone(),
            Default::default(),
        );
        let parser = Parser::new(pc, pass1);

        let result = parser.parse()?;
        let desc = validate_and_describe(pc, result)?;

        if pc.errs.has_errors() {
            anyhow::bail!("parse failed")
        }
        Ok(desc)
    }
}
