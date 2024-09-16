use std::io::Result;

fn main() -> Result<()> {
    prost_build::compile_protos(
        &[
            "../proto/encore/runtime/v1/runtime.proto",
            "../proto/encore/parser/meta/v1/meta.proto",
        ],
        &["../proto/"],
    )?;
    Ok(())
}
