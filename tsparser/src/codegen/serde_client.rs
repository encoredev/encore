use anyhow::{Context, Result};

use crate::codegen::type_decoder::{Path, PathSegment, TypeDecoder};
use crate::codegen::{Generator, Writer};
use crate::parser::resources::apis::api::Method;
use crate::parser::resources::apis::encoding::{EndpointEncoding, Param, ParamData};
use crate::parser::respath::Segment;
use crate::parser::types::Ctx;

pub struct ClientSerdeGenerator<'a> {
    types: TypeDecoder<'a>,

    /// The generated funcs.
    output_funcs: Vec<String>,
}

impl<'a> ClientSerdeGenerator<'a> {
    pub fn new(ctx: &'a Ctx<'a>) -> Self {
        Self {
            types: TypeDecoder::new(ctx, false),
            output_funcs: Vec::new(),
        }
    }

    pub fn output(self) -> String {
        let mut segments = self.output_funcs;
        segments.insert(
            0,
            r#"
import * as de from "@encore.dev/internal-runtime/http/serde/de";
import * as ser from "@encore.dev/internal-runtime/http/serde/ser";
"#
            .trim()
            .to_string(),
        );

        segments.extend(self.types.funcs());

        segments.join("\n\n")
    }

    pub fn gen_response_decoder(&mut self, ep: &EndpointEncoding, func_name: &str) -> Result<()> {
        let mut gen = Generator::new();
        let mut w = gen.writer();

        w.write("export async function ");
        w.write(func_name);
        w.writeln("(resp) {");

        {
            let mut w = w.indent();
            let enc = &ep.resp;
            // Parse the body iff we have body params.
            if enc.body().next().is_some() {
                self.write_body_parse(&mut w);
            }

            w.write("return {");
            if !enc.params.is_empty() {
                w.newline();
                let mut w = w.indent();
                for p in &enc.params {
                    let value_expr = match &p.loc {
                        ParamData::Path { .. } => {
                            anyhow::bail!("internal error: path param in response encoding")
                        }
                        ParamData::Header { header } => {
                            format!("resp.headers.get(\"{}\")", header)
                        }
                        ParamData::Body => {
                            format!("v[\"{}\"]", p.name)
                        }
                        ParamData::Query { .. } => {
                            anyhow::bail!("internal error: query param in response encoding")
                        }
                        ParamData::Cookie => {
                            anyhow::bail!("cookie params are not yet supported")
                        }
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
                        &p.typ,
                    )?;

                    w.writeln(",");
                }
            }
            w.writeln("};");
        }

        w.writeln("}");

        self.output_funcs.push(gen.buf);

        Ok(())
    }

    pub fn gen_request_encoder(&mut self, ep: &EndpointEncoding, func_name: &str) -> Result<()> {
        let mut gen = Generator::new();
        let mut w = gen.writer();

        w.write("export function ");
        w.write(func_name);
        w.writeln("(params) {");

        {
            let mut w = w.indent();

            // Get the request encoding to use.
            let enc = &ep
                .req
                .iter()
                .find(|enc| enc.methods.contains(ep.default_method))
                .or_else(|| ep.req.first())
                .context("no request encoding found")?;

            w.writeln("return {");
            {
                let locs = enc.by_loc();
                let mut w = w.indent();

                // Method
                w.write_string(format!(
                    r#"method: "{}","#,
                    enc.methods.first().unwrap_or(Method::Post).as_str()
                ));
                w.newline();

                // Path
                w.write("path: ");
                let _path_expr = self.write_path(&mut w, "params", &ep, &locs.path, &locs.query)?;
                w.writeln(",");

                // Headers
                w.writeln("headers: {");
                {
                    let mut w = w.indent();
                    w.writeln(r#""Content-Type": "application/json","#);
                    for p in &locs.header {
                        w.write_string(format!(r#""{}": params.{}"#, p.name, p.name));
                        w.writeln(",");
                    }
                }
                w.writeln("},");

                // Body
                w.write("body: ");
                if locs.header.is_empty() {
                    w.writeln("null,");
                } else {
                    w.writeln("JSON.stringify({");
                    {
                        let mut w = w.indent();
                        for p in &locs.body {
                            w.write_string(format!("\"{}\": params.{}", p.name, p.name));
                            w.writeln(",");
                        }
                    }
                    w.writeln("}),");
                }

                if !locs.cookie.is_empty() {
                    anyhow::bail!("cookies are not yet supported in client generation");
                }
            }

            w.writeln("};");
        }

        w.writeln("}");

        self.output_funcs.push(gen.buf);

        Ok(())
    }

    fn write_body_parse(&mut self, w: &mut Writer) {
        w.writeln("const v = (await resp.json());");
        w.writeln("if (v === null) {");
        w.with_indent(|mut w| w.writeln(r#"throw new Error("no data");"#));
        w.writeln("}");
        w.newline();
    }

    /// Write the path expression for the given endpoint.
    fn write_path(
        &mut self,
        w: &mut Writer,
        input_params_expr: &str,
        ep: &EndpointEncoding,
        path_params: &[&Param],
        query_params: &[&Param],
    ) -> Result<()> {
        // Path
        let mut is_dynamic = false;
        let mut path_param_idx = 0;

        let mut path = String::new();
        for seg in &ep.path.segments {
            path += "/";
            match seg {
                Segment::Literal(s) => path += s,
                _ => {
                    // Dynamic literal
                    is_dynamic = true;
                    let p = path_params
                        .get(path_param_idx)
                        .context("internal error: not enough path params")?;
                    path += "${";
                    path += input_params_expr;
                    path += ".";
                    path += &p.name;
                    path += "}";
                    path_param_idx += 1;
                }
            }
        }

        if is_dynamic {
            w.write("`");
            w.write_string(path);
            w.write("`");
        } else {
            w.write("\"");
            w.write_string(path);
            w.write("\"");
        }

        // Query strings
        if !query_params.is_empty() {
            w.writeln(" + ser.query({");
            {
                let mut w = w.indent();
                for p in query_params {
                    w.write_string(format!("\"{}\": {}.{}", p.name, input_params_expr, p.name));
                    w.writeln(",");
                }
            }
            w.write("})");
        }

        Ok(())
    }
}
