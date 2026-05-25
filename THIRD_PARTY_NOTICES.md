# Third-Party Notices

Nikki uses third-party open source dependencies in its frontend and backend.
Dependency licenses should be reviewed before formal releases and after dependency changes.

This notice is informational and does not replace the license files, copyright notices, or notice files distributed by upstream packages.

## Frontend

The current frontend dependency set includes:

- Vue ecosystem dependencies, including Vue, Vue language tooling, and Vue integration packages.
- Vite, TypeScript, and Prettier tooling.
- Tiptap packages for editor behavior.
- `markdown-it`.
- `turndown`.
- `lucide-vue-next`.
- `pinia`.
- `vue-i18n`.

The current bounded review found permissive licenses such as MIT, Apache-2.0, BSD-style, ISC, and Python-2.0 for a transitive frontend dependency.

## Backend

The current backend Go module set includes:

- Echo.
- `pgx` and PostgreSQL-related modules.
- `bcrypt` / `golang.org/x/crypto`.
- `golang.org/x/*` modules.

The current bounded review found permissive licenses such as MIT, BSD-style, and ISC in the backend module set.

Repeat dependency license review before formal public releases and after dependency changes.
