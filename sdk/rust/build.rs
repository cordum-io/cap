fn main() -> Result<(), Box<dyn std::error::Error>> {
    let proto_dir = std::path::Path::new("../../proto");
    let proto_files: Vec<_> = std::fs::read_dir(proto_dir.join("cordum/agent/v1"))?
        .filter_map(|e| e.ok())
        .map(|e| e.path())
        .filter(|p| p.extension().map_or(false, |ext| ext == "proto"))
        .collect();

    prost_build::Config::new()
        .compile_protos(
            &proto_files.iter().map(|p| p.as_path()).collect::<Vec<_>>(),
            &[proto_dir],
        )?;

    Ok(())
}
