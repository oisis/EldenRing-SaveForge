Bump the project version to $ARGUMENTS and prepare a release commit.

Follow these steps exactly in order:

1. Update `Makefile` — change the `VERSION=` line to `VERSION=$ARGUMENTS`
2. Show the diff of the Makefile change and STOP — wait for user "OK" before proceeding
3. Stage only the Makefile: `git add Makefile`
4. Commit with message: `[build] bump version to $ARGUMENTS`
5. Create an annotated git tag: `git tag -a v$ARGUMENTS -m "v$ARGUMENTS"`
6. Show the commit hash and tag — STOP — wait for user "OK" before pushing
7. Push commit and tag: `git push && git push origin v$ARGUMENTS`
