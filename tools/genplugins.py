import json, os

ref = 'C:/tmp/flow-ref/Plugins'
targets = {
    'Flow.Launcher.Plugin.BrowserBookmark': 'browserbookmark',
    'Flow.Launcher.Plugin.Explorer': 'explorer',
    'Flow.Launcher.Plugin.PluginIndicator': 'pluginindicator',
    'Flow.Launcher.Plugin.PluginsManager': 'pluginsmanager',
    'Flow.Launcher.Plugin.ProcessKiller': 'processkiller',
    'Flow.Launcher.Plugin.Program': 'program',
    'Flow.Launcher.Plugin.Shell': 'shell',
    'Flow.Launcher.Plugin.Sys': 'sys',
    'Flow.Launcher.Plugin.Url': 'url',
    'Flow.Launcher.Plugin.WebSearch': 'websearch',
    'Flow.Launcher.Plugin.WindowsSettings': 'windowssettings',
}

BACKSLASH = chr(92)

for folder, pkg in targets.items():
    meta = json.load(open(os.path.join(ref, folder, 'plugin.json'), encoding='utf-8-sig'))
    keywords = meta.get('ActionKeywords') or [meta.get('ActionKeyword', '*')]
    ico = (meta.get('IcoPath') or '').replace(BACKSLASH, '/')

    lines = []
    lines.append('package ' + pkg)
    lines.append('')
    lines.append('import (')
    lines.append('\t"context"')
    lines.append('')
    lines.append('\t"launch/core"')
    lines.append(')')
    lines.append('')
    lines.append('// Plugin is the ' + meta['Name'].lower() + ' plugin.')
    lines.append('type Plugin struct {')
    lines.append('\tcore.BasePlugin')
    lines.append('')
    lines.append('\tapi *core.PublicAPI')
    lines.append('}')
    lines.append('')
    lines.append('// New makes the plugin.')
    lines.append('func New() *Plugin {')
    lines.append('\treturn &Plugin{BasePlugin: core.BasePlugin{Meta: core.PluginMetadata{')
    lines.append('\t\tID:             ' + json.dumps(meta['ID']) + ',')
    lines.append('\t\tName:           ' + json.dumps(meta['Name']) + ',')
    lines.append('\t\tDescription:    ' + json.dumps(meta['Description']) + ',')
    lines.append('\t\tAuthor:         ' + json.dumps(meta['Author']) + ',')
    lines.append('\t\tVersion:        ' + json.dumps(meta.get('Version', '1.0.0')) + ',')
    lines.append('\t\tWebsite:        ' + json.dumps(meta.get('Website', '')) + ',')
    if len(keywords) == 1:
        lines.append('\t\tActionKeywords: []string{' + json.dumps(keywords[0]) + '},')
    else:
        lines.append('\t\tActionKeywords: []string{' + ', '.join(json.dumps(k) for k in keywords) + '},')
    if meta.get('HideActionKeywordPanel'):
        lines.append('\t\tHideActionKeywordPanel: true,')
    lines.append('\t\tIcoPath:        ' + json.dumps(ico) + ',')
    lines.append('\t}}}')
    lines.append('}')
    lines.append('')
    lines.append('// Init is called once, before the first query.')
    lines.append('func (p *Plugin) Init(ctx *core.PluginInitContext) error {')
    lines.append('\tp.api = ctx.API')
    lines.append('\treturn nil')
    lines.append('}')
    lines.append('')
    lines.append('// Query answers a query.')
    lines.append('func (p *Plugin) Query(ctx context.Context, query core.Query) ([]core.Result, error) {')
    lines.append('\treturn nil, nil')
    lines.append('}')
    lines.append('')

    os.makedirs(os.path.join('plugins', pkg), exist_ok=True)
    with open(os.path.join('plugins', pkg, pkg + '.go'), 'w', encoding='utf-8') as f:
        f.write('\n'.join(lines))
    print('wrote plugins/%s/%s.go keywords=%s' % (pkg, pkg, keywords))
