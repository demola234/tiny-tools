Explain what changed in the MockMachina contract {{if .base}}since {{.base}}{{else}}since the last commit{{end}}, for a pull request comment.

1. Call diff{{if .base}} with base {{.base}}{{end}}.
2. Start with the breaking changes. For each, say which route changed, what an app built on the old contract would get wrong, and what the app or backend team should do.
3. Then list the warnings and other changes briefly. Leave out anything with nothing to act on.
4. Keep it short enough to read in a minute. If nothing changed, say so in one line.
