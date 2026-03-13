use std::io::Result;

fn main() -> Result<()> {
    let mut config = prost_build::Config::new();
    if std::env::var("CARGO_FEATURE_SERDE_META").is_ok() {
        config.type_attribute(".", "#[derive(serde::Serialize)]");
    }
    config.compile_protos(
        &["../proto/encore/parser/meta/v1/meta.proto"],
        &["../proto/"],
    )?;
    Ok(())
}
