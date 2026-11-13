# AGENTS.md

Read [CONTRIBUTING.md](CONTRIBUTING.md) first. It has the code conventions, the commit message rules,
how to add changelog entries in [changelog.d](changelog.d/README.md) and how releases are prepared.
In addition:

- Activate the virtual environment before committing, `source venv/bin/activate`, so that the
  pre-commit hooks find their tools.
- Only commit messages and pull request descriptions tell history: what was broken, why it changed,
  and measurements. Code comments, test comments and changelog entries describe the current behaviour.
