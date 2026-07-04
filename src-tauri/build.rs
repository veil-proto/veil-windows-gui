fn main() {
    // TUN adapter creation and routing table changes need Administrator, and
    // there is deliberately no persistent elevated service anymore (see
    // sidecar.rs/main.go) — the whole app requests admin on launch instead,
    // via this embedded manifest, so the sidecar it spawns inherits the
    // elevation. One UAC prompt per launch is the expected tradeoff.
    let windows = tauri_build::WindowsAttributes::new().app_manifest(include_str!("app.manifest"));
    let attributes = tauri_build::Attributes::new().windows_attributes(windows);
    tauri_build::try_build(attributes).expect("failed to run tauri-build");
}
