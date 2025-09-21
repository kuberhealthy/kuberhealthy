# AGENTS

This file defines contributor guidelines for the entire repository.

# Maintain Documentation
Keep an up to date set of documentation in the `docs/agent/` directory with the following:
    - ARCHITECTURE.md: Detail the specific program architecture from a systems design perspective. Analyze the various components of functionality and detail their relationship to each other. Use an ASCII diagram when possible.
    - LOGIC.md: Analyze the program's logic and concisely detail the significant points of the flow.
    - INTERFACES.md: Determine what the defined points of intake are for the program and detail the inputs of those. Also detail the outputs of the program concisely.
    - STRUCTURES.md: Detail the significant data and programatic structures that are defined within the program clearly so that humans can understand their purpose and programatic shape.

# Create Simple Code
- Avoid `else` statements when possible by making additional functions and using a 'return early' strategy.
- Avoid anonymous functions except when they are very simplistic.
- Add comments to every function, including tests, that detail the purpose of that function and what its purpose is to the larger program.
- Strive to maintain a small cyclomatic complexity score through flat, simple code with frequent functions with clear names.
- Avoid turnary operators and other short hand coding practices. Strive for a high level of readability and do not seek to reduce vertical line space.
- Add comments to each stanza of code. Avoid large chunks of code without comments of any kind. Add comments even when they should be obvious.

# Software Preferences
- Use `Containerfile` instead of `Dockerfile` for compatibility with Podman.
- Use `Justfile` instead of `Makefile` for compatibility with `just`.

# Create Modular Program Architecture
- When adding a new data structure or component to the program architecture, ensure that the overall program continues to retain a low-complexity architecture with appropriate delegation of logic and functionality.
- Take care to contain clear abstractions between different program components.
- Ensure that function signatures are kept small and remain widely applicable to future code, even if that means repeating logic in some places.
- Create additive and composable functions by building up to higher levels of logic through small and stackable functions.

# Code Style

- Keep code simple and pragmatic.
- If a line does more than one thing, split it into multiple lines.
- Each function should do one thing; avoid `else` and `else if` by returning early or extracting more functions.
- Prefer early returns and guard clauses to minimize indentation and keep code paths clear.
- Avoid indenting more than five levels and aim for three or fewer.
- Avoid short variable declarations in `if` statements such as `if err := doThing(); err != nil`. Call the function on one line and check the error on the next.
- Prefer methods on structs over directly manipulating struct fields; keep methods short and focused.
- Favor composition over inheritance; keep packages small and focused with limited exported symbols.
- Wrap returned errors with context using `fmt.Errorf` or `%w` so failures are traceable.
- Add inline comments within functions frequently.
- Test names should not use underscores (use testName instead of test_name).
- Tests should focus on one small behavior. End-to-end tests may cover a large process but should assert only its initial inputs and final outputs.
- Prefer standard library packages when practical.
- Every package must include a README.md describing its scope of responsibility.

## Project decisions
- Logging is handled with `github.com/sirupsen/logrus`.
- Use `context.Context` as the first parameter for functions that perform I/O or long running work.
- Tests should be written with Go's testing package and live beside the code they test.
- Configuration is loaded from environment variables when possible.
- File names are in camelCase
- Package names must be all lower case
- Podman is used instead of Docker
- Just is used instead of Make

## Formatting and tests
- Run `gofmt -w` on changed Go files.
- Run `go test -short ./...` before committing. Use `go test ./...` for the full suite.
