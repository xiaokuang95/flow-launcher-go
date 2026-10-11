# The launcher's own languages

These are the reference's `Flow.Launcher/Languages/*.xaml`, copied
unchanged, so that the launcher speaks the same 26 languages with the same
words. `core/i18n.go` reads them: `<system:String x:Key="...">value</...>`
entries, whose values may run over several lines.

The file names are the reference's, and two of them do not match the
language code the settings hold: the code `vi-vn` has a file named
`vi.xaml`, and `nb-NO` has both `nb.xaml` and `nb-NO.xaml`. `core/i18n.go`
looks the code up, then the language without its region, so Vietnamese
loads; the reference looks the code up alone and shows English instead.
