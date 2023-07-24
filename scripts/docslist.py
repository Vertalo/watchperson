from pathlib import Path

import pathspec

DOCS_DIR = 'mkdocs-docs'
SPEC_FILE = '.docinclude'

def schema():
    # Create PathSpec object from .docinclude
    # (see https://pypi.org/project/pathspec/)
    with open(SPEC_FILE, "r") as fp:
        spec = pathspec.PathSpec.from_lines("gitwildmatch", fp)

    # Use the PathSpec object to match our desired documentation assets
    matches = spec.match_tree('.')
    paths = [print(match) for match in matches]

if __name__ == "__main__":
    schema()
