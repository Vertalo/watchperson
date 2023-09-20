import os
from mkdocs.structure.files import File
from mkdocs.structure.files import Files

def on_files(in_files, config):
    """Use files in docs-list."""

    files: list[File] = []
    with open('docs-list') as fl:
        docs = config['docs_dir']
        site = config['site_dir']
        urls = config['use_directory_urls']

        for f in fl:
            f = f.strip()
            if f.startswith('#'):
                continue

            file = File(
                os.path.relpath(f, docs),
                docs,
                site,
                urls,
            )
            files.append(file)

        env = config.theme.get_env()
        out_files = Files(files)
        out_files.add_files_from_theme(env, config)

    return out_files

