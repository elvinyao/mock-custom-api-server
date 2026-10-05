# Vendored browser assets

Swagger UI is pinned to `swagger-ui-dist` version **5.17.14**. The files
`swagger-ui-bundle.js`, `swagger-ui.css`, and `swagger-ui-LICENSE` come from:

https://registry.npmjs.org/swagger-ui-dist/-/swagger-ui-dist-5.17.14.tgz

These assets are embedded in the Go binary for offline use. Preserve the Apache
2.0 license when updating them. Run any download or extraction through
`bash .agent/run.sh`, as required by the repository execution policy.

The existing `alpine.min.js` and `tailwind.cdn.js` are also served locally by the
dashboard. Their existing embedded license notices remain unchanged.
