# Contributing to mp4ff

Contributions are welcome, as issues and as pull requests against the `master` branch.
Coding agents follow these conventions too, with the additions in [AGENTS.md](AGENTS.md).

## Setting up

The module needs Go 1.24 or later. Build, test and lint with

    make build
    make test
    make check

Pre-commit hooks run `go fmt`, `golangci-lint`, the unit tests and `codespell`, and check
the commit message. Install them once per clone with

    make pre-commit-install

which creates a Python virtual environment in `venv` with `pre-commit` and installs both the
pre-commit and the commit-msg hook.

## Commit messages

Commit messages must follow the [Conventional Commits](https://www.conventionalcommits.org/)
specification, which the commit-msg hook checks. Examples:

- `feat: add support for VVC video codec`
- `fix: resolve memory leak in fragment processing`
- `docs: update API documentation for mp4.File`
- `chore: update dependencies to latest versions`

## Code conventions

### Both decode and encode paths

Every box can be decoded from an `io.Reader` and from a `bits.SliceReader`, and encoded to an
`io.Writer` and to a `bits.SliceWriter`. The SliceReader and SliceWriter paths are much faster
and allocate far less, but both paths must be kept working.

### How to implement a new box

To implement a new box `fooo`, the following is needed:

1. Create a file `mp4/fooo.go` with a struct type `FoooBox`.
2. `FoooBox` must implement the `Box` interface methods

    ```go
    Type() string
    Size() uint64
    Encode(w io.Writer) error
    EncodeSW(sw bits.SliceWriter) error
    Info(w io.Writer, specificBoxLevels, indent, indentStep string) error
    ```

3. Add the decode functions `DecodeFooo` and `DecodeFoooSR` to the `decoders` map in `mp4/box.go`
   and the `decodersSR` map in `mp4/boxsr.go`, respectively.
4. Add a test in `mp4/fooo_test.go` that uses `boxDiffAfterEncodeAndDecode` to check that
   the box is the same after encoding and decoding. Test both decode paths where possible.

For a simple example, look at the `PrftBox` in `mp4/prft.go`.

### Sample numbers

Following the ISOBMFF standard, sample numbers in the APIs start at 1.
The storage in slices is zero-based, so sample number 1 has index 0.

### Specifications

The main specification is ISO/IEC 14496-12:2026 (ISOBMFF, 8th edition). Boxes from other
specifications should say in their doc comment where they are defined, with the clause number.

### Comments and changelog

Code comments describe the current behaviour. What was broken before and why it changed belongs
in the commit message and the pull request description.

For each change that users will notice, add a file to [changelog.d](changelog.d/) instead of editing
[CHANGELOG.md](CHANGELOG.md), as described in [changelog.d/README.md](changelog.d/README.md). Keep
each entry to one or two lines that say what changed and name the affected API or tool. The entries
move into CHANGELOG.md when a version is released.

### Documentation

The guide to the library is the package documentation in [mp4/doc.go](mp4/doc.go), shown on
[pkg.go.dev](https://pkg.go.dev/github.com/Eyevinn/mp4ff/mp4). Preview documentation changes
locally with `make open-docs`.

## Releases

Maintainers prepare a release in a pull request `chore: prepare vX.Y.Z release` from a branch
`release-vX.Y.Z`. It moves the changelog entries into CHANGELOG.md as described in
[changelog.d/README.md](changelog.d/README.md), and renames a BENCHMARKS.md section titled
"(unreleased)" to the version. Before 1.0, a new API or a breaking change gives a minor version and
fixes only a patch version. A tag is permanent for Go modules, since the module proxy caches it, so
never move or reuse one.
