use std::error::Error;

fn main() -> Result<(), Box<dyn Error>> {
    tonic_build::configure()
        .build_server(true)
        .compile(&["../proto/cloudguardian.proto"], &["../proto/"])?;
    Ok(())
}
