use std::env;
use std::fs;
use std::io;
use std::path::{Path, PathBuf};

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let manifest_dir = PathBuf::from(env::var("CARGO_MANIFEST_DIR")?);
    let repo_dir = manifest_dir
        .parent()
        .and_then(Path::parent)
        .ok_or_else(|| io::Error::new(io::ErrorKind::NotFound, "CAP repository root"))?;
    let proto_dir = repo_dir.join("proto");
    let source_dir = proto_dir.join("cordum/agent/v1");
    let proto_files = sorted_proto_files(&source_dir)?;
    let protoc = protoc_bin_vendored::protoc_bin_path()?;
    let protoc_include = protoc_bin_vendored::include_path()?;

    env::set_var("PROTOC", protoc);
    println!("cargo:rerun-if-changed={}", source_dir.display());
    for path in &proto_files {
        println!("cargo:rerun-if-changed={}", path.display());
    }

    let mut config = prost_build::Config::new();
    config.btree_map(["."]);
    config.compile_protos(&proto_files, &[proto_dir, protoc_include])?;

    Ok(())
}

fn sorted_proto_files(source_dir: &Path) -> io::Result<Vec<PathBuf>> {
    let mut proto_files = Vec::new();
    for entry in fs::read_dir(source_dir)? {
        let path = entry?.path();
        if path.extension().map_or(false, |ext| ext == "proto") {
            proto_files.push(path);
        }
    }
    proto_files.sort();
    if proto_files.is_empty() {
        return Err(io::Error::new(
            io::ErrorKind::NotFound,
            "no CAP proto files",
        ));
    }
    Ok(proto_files)
}
