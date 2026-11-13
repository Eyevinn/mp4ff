# AGENTS.md

Read [CONTRIBUTING.md](CONTRIBUTING.md) first. It has the code conventions, the commit message rules
and how to add changelog entries in [changelog.d](changelog.d/README.md). In addition:

- Activate the virtual environment before committing, `source venv/bin/activate`, so that the
  pre-commit hooks find their tools.
- Only commit messages and pull request descriptions tell history: what was broken, why it changed,
  and measurements. Code comments, test comments and changelog entries describe the current behaviour.

## Releases

A release is prepared in a pull request `chore: prepare vX.Y.Z release` from a branch
`release-vX.Y.Z`. It moves the changelog entries into CHANGELOG.md as described in
[changelog.d/README.md](changelog.d/README.md), and renames a BENCHMARKS.md section titled
"(unreleased)" to the version. Before 1.0, a new API or a breaking change gives a minor version and
fixes only a patch version. A tag is permanent for Go modules, since the module proxy caches it, so
never move or reuse one.
