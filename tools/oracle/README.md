# Matcher oracle

`oracle` runs the reference's own `StringMatcher` over cases given on stdin,
one JSON object per line:

```
{"query": "chr", "compare": "Chrome", "precision": 50, "ignoreAccents": false}
```

and answers with the match result, one JSON object per line:

```
{"success": true, "rawScore": 157, "score": 157, "met": true, "matchData": [0,1,2]}
```

It exists so that the Go port is checked against the implementation it
copies rather than against a reading of it. `core/oracle_test.go` runs both
over a generated corpus and fails on any difference.

## The vendored files

`reference/` holds the reference's own sources, copied unchanged from
[Flow-Launcher/Flow.Launcher](https://github.com/Flow-Launcher/Flow.Launcher)
at the commit in `reference/UPSTREAM.txt`. `src/Settings.cs` stands in for
the reference's `Settings`, which drags in WPF, with only the two fields the
matcher reads.

`StringMatcher.cs` imports `CommunityToolkit.Mvvm` for one static helper the
oracle never calls; the package reference keeps the file unchanged.

## Running it

```sh
dotnet run --project tools/oracle < cases.jsonl
```

The test skips itself when `dotnet` is not on `PATH`.
