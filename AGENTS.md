# AGENTS

This file defines contributor guidelines for the entire repository.

## Development principles
- Keep code simple and pragmatic.
- If a line does more than one thing, split it into multiple lines.
- Each function should do one thing; avoid `else` and `else if` by returning early or extracting more functions.
- Prefer early returns and guard clauses to minimize indentation and keep code paths clear.
- Avoid indenting more than five levels and aim for three or fewer.
- Avoid short variable declarations in `if` statements such as `if err := doThing(); err != nil`. Call the function on one line and check the error on the next.
- Prefer methods on structs over directly manipulating struct fields; keep methods short and focused.
- Favor composition over inheritance; keep packages small and focused with limited exported symbols.
- Wrap returned errors with context using `fmt.Errorf` or `%w` so failures are traceable.
- Document all exported functions, types, and methods with brief, descriptive comments.
- Add inline comments within functions frequently.
- Test names should not use underscores (use testName instead of test_name).
- Tests should focus on one small behavior. End-to-end tests may cover a large process but should assert only its initial inputs and final outputs.
- For normal validation, you only need to run tests with short mode (-short). Continue to target specific tests as needed using `-run`, though.
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
