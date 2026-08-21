// Generates the `encore.ecl.v1` proto types (the ECL evaluation-output wire
// schema) into OUT_DIR, included via `pub mod pb` in lib.rs.
fn main() -> std::io::Result<()> {
    prost_build::compile_protos(&["../../proto/encore/ecl/v1/ecl.proto"], &["../../proto/"])?;
    Ok(())
}
