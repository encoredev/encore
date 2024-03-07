use std::io::Result;

fn main() -> Result<()> {
    prost_build::compile_protos(&["../proto/encore/parser/meta/v1/meta.proto"], &["../proto/"])?;
    Ok(())
}
