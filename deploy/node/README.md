# Local Node Binaries

## Accepted Layouts

- `linux/node_linux_amd64`
- `linux/node_linux_arm64`
- `macos/node_macos_arm64`
- `windows/node_windows_amd64.exe`
- `windows/node_windows_arm64.exe`
- `windows/runner_windows_amd64.exe`
- `windows/runner_windows_arm64.exe`

## Accepted Flat Asset Names

- `Ithiltir-node-linux-amd64`
- `Ithiltir-node-linux-arm64`
- `Ithiltir-node-macos-arm64`
- `Ithiltir-node-windows-amd64.exe`
- `Ithiltir-node-windows-arm64.exe`
- `Ithiltir-runner-windows-amd64.exe`
- `Ithiltir-runner-windows-arm64.exe`

## Package Command

```bash
bash scripts/package.sh --version 0.0.0-dev.0 --node-version 0.0.0-dev.0 --node-local -o release -t linux/amd64 --tar-gz
```
