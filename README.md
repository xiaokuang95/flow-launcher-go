# flow-launcher-go

A port of [Flow Launcher](https://github.com/Flow-Launcher/Flow.Launcher)
to Go, built on the [mygo](https://github.com/egoist/mygo) desktop
framework.

Flow Launcher is a Windows launcher written in C# on WPF. This is the same
launcher, rewritten in Go: the same search pipeline, the same plugins, the
same settings, the same themes and the same languages, drawn by mygo's own
native UI rather than by WPF.

## What it does

Type to search. The launcher asks every plugin that answers to the query at
once and shows the results as they arrive, best first.

- **Programs** — the Start Menu, the registry's App Paths, the PATH and the
  folders you add, run from the launcher
- **Files** — search your folders, and walk into a path as you type it
- **Bookmarks** — what you saved in Chrome, Edge and the other Chromium
  browsers
- **The web** — twenty searches, from Google to Wikipedia, with what a site
  suggests as you type
- **System** — shut down, restart, sleep, lock, empty the recycle bin, and
  the launcher's own settings
- **Calculator** — arithmetic, with the functions and the exact rounding of
  the reference's engine
- **Windows settings** — every page of the settings app, by name
- **Running programs** — find one and end it
- **Plugins** — install, remove and update them from the store, and write
  your own in Python, JavaScript or any language that can be run

It also has a preview panel, a menu on every result, a tray icon, a global
hotkey, a settings window, nineteen themes and twenty-six languages.

## Building

Go 1.27 or later:

```sh
go build -o launch.exe .
./launch.exe
```

Cross-compiling for the other platforms only checks that it compiles: the
interface is mygo's, which draws natively on each of them.

```sh
CGO_ENABLED=0 GOOS=linux go build ./...
CGO_ENABLED=0 GOOS=darwin go build ./...
```

## Testing

```sh
go test ./...
```

191 tests. The platform's own actions are kept behind injectable fields
(`sys.Machine`, `shell.Runner`, `processkiller.Processes`, `url.Opener`,
`program.Finder`, `explorer.FileIndex`, `windowssettings.Opener`), so a
test watches what was asked for rather than locking the screen or ending a
process.

### Checking against the reference

`tools/oracle` is the reference's own C# sources, copied in unchanged and
built with the .NET SDK, so that this port can be compared against what the
reference really does rather than against a reading of it:

```sh
dotnet build tools/oracle
go test ./core/ -run 'TestMatchesTheReference|TestHashesMatchTheReference'
go test ./plugins/calculator/ ./plugins/url/
```

The StringMatcher (1058 cases), the result hashes (900), the calculator's
expressions and .NET decimal formatting, and the URL validation rules are
all checked this way, case for case.

## Layout

| | |
|---|---|
| `core/` | the search pipeline: queries, matching, results, plugins, settings, storage, i18n, themes |
| `plugins/` | the twelve built-in plugins |
| `tools/oracle/` | the reference's sources, for comparing against |
| `tools/extractthemes/` | reads the reference's themes into `core/themes_data.go` |
| `tools/i18nkeys/` | prints the launcher's own translations, to write the interface against |
| `docs/state.md` | what is done, what is not, and why |
| `docs/plugin-authoring.md` | how to write a plugin |

## Writing a plugin

A plugin is a Go package that answers a query with results, or a program of
its own that the launcher runs over JSON-RPC. `docs/plugin-authoring.md`
describes both, and `plugins/calculator/` is the worked example.

## What is not done

- **DialogJump (Alt+G)** — the shell's object model would not answer a
  late-bound call, whatever the locale or the class context. See
  `docs/state.md`.
- **Checking for a new version** — this launcher has no release channel.

## License

MIT, as Flow Launcher is. The reference's language files, theme colours and
the sources under `tools/oracle/` are its authors' work, taken from
[Flow-Launcher/Flow.Launcher](https://github.com/Flow-Launcher/Flow.Launcher)
at commit `c68afbc`.
