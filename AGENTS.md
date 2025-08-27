# AGENTS

This file defines contributor guidelines for the entire repository.

## Development principles
- Keep code simple and pragmatic.
- If a line does more than one thing, split it into multiple lines.
- Add inline comments within functions frequently.
- Try hard not to indent past 4 indents anywhere in the codebase.
- Prefer standard library packages when practical.
- Test names should not use underscores (use testName instead of test_name)
- Never use `else` statements unless absolutely necessary. Instead use an `if` and a `return` with more functions.

## Project decisions
- Logging is handled with `github.com/sirupsen/logrus`.
- Use `context.Context` as the first parameter for functions that perform I/O or long running work.
- Tests should be written with Go's testing package and live beside the code they test.
- Configuration is loaded from environment variables when possible.
- File names are in camelCase
- Package names must be all lower case

## Formatting and tests
- Run `gofmt -w` on changed Go files.
- Run `go test ./...` before committing.
