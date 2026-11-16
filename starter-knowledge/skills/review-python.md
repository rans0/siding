# review-python

Loaded at review when a repo in this track has `pyproject.toml`, `requirements.txt` or `setup.py`. Run the tools first; review what they cannot see.

Tools: the repo's test command, `ruff check`, `mypy` or `pyright` if configured, `bandit -r <pkg>`, `pip-audit`.

- **Errors.** No bare `except:` or `except Exception: pass`. Exceptions re-raised with `raise ... from err`. Specific exceptions caught.
- **Defaults.** No mutable default arguments (`def f(x=[])`). No module-level state mutated per request.
- **Resources.** Files, connections and locks opened with `with`. Timeouts on `requests`/`httpx` calls (the default is none).
- **Async.** No blocking calls (`requests`, `time.sleep`, sync DB drivers) inside `async def`. Tasks awaited or tracked.
- **Types.** Public functions annotated. `Optional` values checked. `Any` and `# type: ignore` justified.
- **Unsafe calls.** No `eval`, `exec`, `pickle.loads`, `yaml.load` (without `SafeLoader`) or `subprocess(..., shell=True)` on untrusted input.
- **Framework.** Django/FastAPI: querysets evaluated once (N+1, see review-database), request models validate input, auth dependencies on every route.

Sources: [PEP 8](https://peps.python.org/pep-0008/), [PEP 484](https://peps.python.org/pep-0484/), [Google Python Style Guide](https://google.github.io/styleguide/pyguide.html), [Ruff rules](https://docs.astral.sh/ruff/rules/), [Bandit](https://bandit.readthedocs.io/).
