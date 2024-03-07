use std::path::Path;

use anyhow::Result;
use swc_common::sync::Lrc;

use crate::app::{validate_and_describe, AppDesc};
use crate::parser::parser::{ParseContext, Parser};
use crate::parser::resourceparser::PassOneParser;
use crate::parser::FileSet;

use super::{App, Builder};

#[derive(Debug, Clone)]
pub struct ParseParams<'a> {
    pub app: &'a App,
    pub pc: &'a ParseContext<'a>,
    pub working_dir: &'a Path,
    pub parse_tests: bool,
}

#[derive(Debug)]
pub struct ParseResult {
    pub file_set: Lrc<FileSet>,
    pub desc: AppDesc,
}

impl Builder<'_> {
    pub fn parse(&self, params: &ParseParams) -> Result<ParseResult> {
        let pc = params.pc;
        let pass1 = PassOneParser::new(
            pc.file_set.clone(),
            pc.type_checker.clone(),
            Default::default(),
        );
        let parser = Parser::new(&pc, pass1);

        let result = parser.parse()?;
        let desc = validate_and_describe(&pc, &result)?;

        let file_set = pc.file_set.clone();
        Ok(ParseResult { file_set, desc })
    }
}
