# CLAUDE.md - Guidelines for Raindrop.io MCP Server (Go)

## Build & Run Commands
- Build project: `go build -o raindrop-mcp-server`
- Run server: `./raindrop-mcp-server`
- Get dependencies: `go mod tidy`
- Format code: `go fmt ./...`
- Lint code: `golangci-lint run`
- Test: `go test ./...`

## Code Style Guidelines
- **Imports**: Group standard library imports first, then third-party, then local packages
- **Types**: Use strong typing with interfaces when appropriate
- **Error Handling**: Check errors explicitly, use descriptive error messages
- **Naming**: Use camelCase for unexported functions/variables, PascalCase for exported ones
- **Formatting**: Use `gofmt` standard formatting, 4-space indentation by convention
- **Comments**: Include godoc-style comments for all exported functions
- **Environment**: Use godotenv for loading environment variables
- **Logging**: Use the standard log package or a structured logger like zap/zerolog

## MCP Server Standards
- Validate all inputs with proper type assertions or JSON unmarshaling
- Use explicit error returns for handling failures
- Use jsonschema package for schema validation
- Return structured responses following the MCP protocol
- Group related functionality into separate packages for larger applications