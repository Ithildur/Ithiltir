module dash/web

go 1.26

// Nested module boundary for the frontend workspace.
// This keeps root `go list ./...` and `go test ./...` from traversing `web/node_modules`.
