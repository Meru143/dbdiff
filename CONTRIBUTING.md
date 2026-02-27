# Contributing to DBDiff

## Development Setup

```bash
git clone github.com/Meru143/dbdiff
cd dbdiff
go mod download
make test
make build
```

## Code Standards

- Follow Go standard conventions
- Run `make lint` before committing
- Add tests for new features
- Keep coverage above 20%

## Commit Messages

- `feat: add new command`
- `fix: resolve bug`
- `docs: update README`
- `test: add tests`
- `chore: update dependencies`

## Pull Request Process

1. Fork the repo
2. Create a feature branch
3. Make changes and add tests
4. Run `make test` and `make lint`
5. Push and create PR
6. Wait for CI to pass
