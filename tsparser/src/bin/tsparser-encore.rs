use std::collections::HashMap;
use std::io::{self, BufRead, Write};
use std::path::PathBuf;

use anyhow::Result;
use prost::Message;
use serde::{Deserialize, Serialize};
use swc_common::{Globals, GLOBALS};

use encore_tsparser::builder;
use encore_tsparser::builder::Builder;
use encore_tsparser::parser::parser::ParseContext;

fn main() -> Result<()> {
    env_logger::init();
    let cwd = std::env::current_dir()?;

    let js_runtime_path = std::env::var("ENCORE_JS_RUNTIME_PATH")
        .map(PathBuf::from)
        .expect("ENCORE_JS_RUNTIME_PATH not set");

    let globals = Globals::new();
    GLOBALS.set(&globals, || -> Result<()> {
        let builder = Builder::new()?;
        let mut parse: Option<(builder::App, builder::ParseResult)> = None;

        let prepare = match parse_cmd()? {
            Some(Command::Prepare(prepare)) => prepare,
            Some(_) => anyhow::bail!("expected prepare command"),
            None => return Ok(()),
        };

        {
            let pp = builder::PrepareParams {
                js_runtime_root: &js_runtime_path,
                runtime_version: &prepare.runtime_version,
                app_root: &prepare.app_root,
                use_local_runtime: prepare.use_local_runtime,
            };

            match builder.prepare(&pp) {
                Ok(result) => {
                    let json = serde_json::to_string(&result)?;
                    write_result(Ok(json.as_bytes()))?;
                }
                Err(err) => {
                    log::error!("failed to prepare: {:?}", err);
                    write_result(Err(err))?
                }
            }
        }

        let pc = match ParseContext::new(prepare.app_root, &js_runtime_path) {
            Ok(pc) => pc,
            Err(err) => {
                log::error!("failed to construct parse context: {:?}", err);
                write_result(Err(err))?;
                return Ok(());
            }
        };

        loop {
            let cmd = match parse_cmd()? {
                Some(cmd) => cmd,
                None => return Ok(()),
            };

            match cmd {
                Command::Prepare(input) => {
                    log::debug!("got prepare input {:?}", input);
                }

                Command::Parse(input) => {
                    log::debug!("got parse input {:?}", input);
                    if parse.is_some() {
                        anyhow::bail!("already parsed!");
                    }

                    let app = builder::App {
                        root: input.app_root.clone(),
                        platform_id: input.platform_id,
                        local_id: input.local_id,
                    };
                    let pp = builder::ParseParams {
                        app: &app,
                        pc: &pc,
                        working_dir: &cwd,
                        parse_tests: input.parse_tests,
                    };

                    match builder.parse(&pp) {
                        Ok(result) => {
                            write_result(Ok(result.desc.meta.encode_to_vec().as_slice()))?;
                            parse = Some((app, result));
                        }
                        Err(err) => {
                            log::error!("failed to parse: {:?}", err);
                            write_result(Err(err))?
                        }
                    }
                }

                Command::Compile(input) => match &parse {
                    None => anyhow::bail!("no parse!"),
                    Some((app, parse)) => {
                        let cp = builder::CompileParams {
                            js_runtime_root: &js_runtime_path,
                            runtime_version: &input.runtime_version,
                            app,
                            pc: &pc,
                            working_dir: &cwd,
                            parse: &parse,
                            use_local_runtime: input.use_local_runtime,
                        };

                        match builder.compile(&cp) {
                            Ok(compile) => {
                                let json = serde_json::to_string(&compile)?;
                                write_result(Ok(json.as_bytes()))?;
                            }
                            Err(err) => {
                                log::error!("failed to compile: {:?}", err);
                                write_result(Err(err))?
                            }
                        };
                    }
                },

                Command::Test(input) => match &parse {
                    None => anyhow::bail!("no parse!"),
                    Some((app, parse)) => {
                        let p = builder::TestParams {
                            js_runtime_root: &js_runtime_path,
                            runtime_version: &input.runtime_version,
                            app,
                            pc: &pc,
                            working_dir: &cwd,
                            parse: &parse,
                            use_local_runtime: input.use_local_runtime,
                        };

                        match builder.test(&p) {
                            Ok(compile) => {
                                let json = serde_json::to_string(&compile)?;
                                write_result(Ok(json.as_bytes()))?;
                            }
                            Err(err) => {
                                log::error!("failed to run tests: {:?}", err);
                                write_result(Err(err))?
                            }
                        };
                    }
                },

                Command::GenUserFacing(input) => match &parse {
                    None => anyhow::bail!("no parse!"),
                    Some((app, parse)) => {
                        let cp = builder::CodegenParams {
                            js_runtime_root: &js_runtime_path,
                            app,
                            pc: &pc,
                            working_dir: &cwd,
                            parse: &parse,
                        };

                        match builder.generate_code(&cp) {
                            Ok(_) => write_result(Ok(&[]))?,
                            Err(err) => {
                                log::error!("failed to generate code: {:?}", err);
                                write_result(Err(err))?
                            }
                        };
                    }
                },
            }
        }
    })
}

fn write_data(is_ok: bool, data: &[u8]) -> io::Result<()> {
    let mut stdout = std::io::stdout().lock();
    let byte_len = ((data.len() + 1) as u32).to_le_bytes();
    stdout.write_all(&byte_len)?;
    stdout.write_all(&[if is_ok { 0 } else { 1 }])?;
    stdout.write_all(data)?;
    stdout.flush()?;
    Ok(())
}

fn write_result(res: Result<&[u8]>) -> io::Result<()> {
    match res {
        Ok(bytes) => write_data(true, bytes),
        Err(err) => {
            let s = format!("{:?}", err);
            let bytes = s.as_bytes();
            write_data(false, bytes)
        }
    }
}

enum Command {
    Prepare(PrepareInput),
    Parse(ParseInput),
    Compile(CompileInput),
    Test(TestInput),
    GenUserFacing(GenUserFacingInput),
}

fn parse_cmd() -> Result<Option<Command>> {
    let stdin = io::stdin();
    let mut stdin = stdin.lock();

    // Read a line and see what it says.
    let line = {
        let mut line = String::new();
        stdin.read_line(&mut line)?;
        line
    };

    match line.trim() {
        "" => Ok(None),
        "prepare" => {
            let mut de = serde_json::Deserializer::from_reader(stdin);
            let input = PrepareInput::deserialize(&mut de)?;
            Ok(Some(Command::Prepare(input)))
        }
        "parse" => {
            let mut de = serde_json::Deserializer::from_reader(stdin);
            let input = ParseInput::deserialize(&mut de)?;
            Ok(Some(Command::Parse(input)))
        }
        "gen-user-facing" => {
            let mut de = serde_json::Deserializer::from_reader(stdin);
            let input = GenUserFacingInput::deserialize(&mut de)?;
            Ok(Some(Command::GenUserFacing(input)))
        }
        "compile" => {
            let mut de = serde_json::Deserializer::from_reader(stdin);
            let input = CompileInput::deserialize(&mut de)?;
            Ok(Some(Command::Compile(input)))
        }
        "test" => {
            let mut de = serde_json::Deserializer::from_reader(stdin);
            let input = TestInput::deserialize(&mut de)?;
            Ok(Some(Command::Test(input)))
        }
        _ => anyhow::bail!("unknown command {:#?}", line),
    }
}

#[derive(Deserialize, Debug)]
struct ParseInput {
    app_root: PathBuf,
    platform_id: Option<String>,
    local_id: String,
    parse_tests: bool,
}

#[derive(Serialize, Debug)]
struct CompileResult {
    pub build_dir: PathBuf,
    pub gateways: HashMap<String, CmdSpec>,
    pub services: HashMap<String, CmdSpec>,
}

#[derive(Serialize, Debug)]
struct CmdSpec {
    pub cmd: String,
    pub args: Vec<String>,
    pub env: Vec<String>,
}

#[derive(Deserialize, Debug)]
struct PrepareInput {
    app_root: PathBuf,
    runtime_version: String,
    use_local_runtime: bool,
}

#[derive(Deserialize, Debug)]
struct CompileInput {
    runtime_version: String,
    use_local_runtime: bool,
}

#[derive(Deserialize, Debug)]
struct TestInput {
    runtime_version: String,
    use_local_runtime: bool,
}

#[derive(Deserialize, Debug)]
struct GenUserFacingInput {}
