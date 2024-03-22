use anyhow::Result;

use crate::codegen::type_decoder::{Path, PathSegment, TypeDecoder};
use crate::codegen::{Generator, Writer};
use crate::parser::resources::apis::encoding::{EndpointEncoding, ParamData, RequestEncoding};
use crate::parser::types::{Ctx, Type};

pub struct RequestDecoderGenerator<'a> {
    types: TypeDecoder<'a>,

    /// The generated funcs.
    output_funcs: Vec<String>,
}

impl<'a> RequestDecoderGenerator<'a> {
    pub fn new(ctx: &'a Ctx<'a>) -> Self {
        Self {
            types: TypeDecoder::new(ctx, true),
            output_funcs: Vec::new(),
        }
    }

    pub fn output(self) -> String {
        let mut segments = self.output_funcs;
        segments.insert(
            0,
            r#"
import { Request } from "@encore.dev/internal-runtime/http/types";
import * as de from "@encore.dev/internal-runtime/http/serde/de";
"#
            .trim()
            .to_string(),
        );

        segments.extend(self.types.funcs());

        segments.join("\n\n")
    }

    pub fn gen_request_decoder(&mut self, ep: &EndpointEncoding, func_name: &str) -> Result<()> {
        let mut gen = Generator::new();
        let mut w = gen.writer();

        w.write("export async function ");
        w.write(func_name);
        w.writeln("(req: Request, pathParams: string[]) {");

        {
            let mut w = w.indent();
            if ep.req.len() == 1 {
                // If we have a single encoding, use that.
                self.gen_request_decode_expression(&mut w, &ep.req[0], false)?;
            } else {
                // If all the encodings need the request body,
                // parse it once at the top.
                let all_need_body = ep.req.iter().all(|enc| enc.body().next().is_some());
                if all_need_body {
                    self.write_body_parse(&mut w);
                }

                // Otherwise, generate a switch statement.
                w.writeln("switch (req.method) {");
                {
                    for enc in &ep.req {
                        for m in enc.methods.to_vec() {
                            w.write("case \"");
                            w.write(&m);
                            w.writeln("\":");
                        }
                        {
                            let mut w = w.indent();
                            self.gen_request_decode_expression(&mut w, enc, all_need_body)?;
                        }
                    }
                }
                w.writeln("}");
            }
        }

        w.writeln("}");

        self.output_funcs.push(gen.buf);

        Ok(())
    }

    fn gen_request_decode_expression(
        &mut self,
        w: &mut Writer,
        enc: &RequestEncoding,
        parsed_body_already: bool,
    ) -> Result<()> {
        // Parse the body iff we have body params.
        if !parsed_body_already && enc.body().next().is_some() {
            self.write_body_parse(w);
        }

        w.write("return {");
        if !enc.params.is_empty() {
            w.newline();
            let mut w = w.indent();
            for p in &enc.params {
                let value_expr = match &p.loc {
                    ParamData::Path { index } => {
                        format!("pathParams[{}]", index)
                    }
                    ParamData::Header { header } => {
                        format!("req.headers.get(\"{}\")", header)
                    }
                    ParamData::Body => {
                        format!("v[\"{}\"]", p.name)
                    }
                    ParamData::Query { query } => {
                        format!("req.query.get(\"{}\")", query)
                    }
                    ParamData::Cookie => {
                        anyhow::bail!("cookie params are not yet supported")
                    }
                };

                // If the field is optional, wrap in Type::Optional so we generate
                // the correct output code.
                let typ = match p.optional {
                    true => Type::Optional(Box::new(p.typ.clone())),
                    false => p.typ.clone(),
                };

                w.write(&p.name);
                w.write(": ");
                self.types.typ(
                    &mut w,
                    &value_expr,
                    Path::new(&mut vec![
                        PathSegment::Root,
                        PathSegment::Lit(p.name.to_string()),
                    ]),
                    &typ,
                )?;

                w.writeln(",");
            }
        }
        w.writeln("};");

        Ok(())
    }

    fn write_body_parse(&mut self, w: &mut Writer) {
        w.writeln("const v = (await req.json()) as any;");
        w.writeln("if (v === null) {");
        w.with_indent(|mut w| w.writeln(r#"throw new Error("no data");"#));
        w.writeln("}");
        w.newline();
    }
}
