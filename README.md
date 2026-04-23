# lazypprof

A terminal UI for exploring Go pprof profiles. Inspired by lazygit, lazydocker, and lazytrivy.

## Status

v0.1 in progress.

- [x] Load CPU + heap profiles from disk
- [x] Top view (functions sorted by cumulative samples)
- [x] Switch sample type with `s`
- [ ] Tree view
- [ ] Flame graph
- [ ] Live mode (`/debug/pprof` polling)
- [ ] Diff mode

## Usage

```bash
go mod tidy
go run . path/to/profile.pb.gz
```

Try it against a running Go service exposing pprof:

```bash
curl -o cpu.pb.gz "http://localhost:6060/debug/pprof/profile?seconds=10"
go run . cpu.pb.gz
```

## Keys

| Key   | Action            |
|-------|-------------------|
| `tab` | Cycle views       |
| `s`   | Cycle sample type |
| `↑/↓` | Navigate table    |
| `q`   | Quit              |
