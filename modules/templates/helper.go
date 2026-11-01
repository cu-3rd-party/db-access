package templates

import (
	"errors"
	"html/template"
	"io/fs"
	"log/slog"
	"strings"
	"time"

	"git.cu3rd.ru/cu3rd/db-access/modules/assets"
)

var msk = time.FixedZone("MSK", 3*60*60)

func readPublic(path string) string {
	content, err := fs.ReadFile(assets.Public(), path)
	if err != nil {
		slog.Error("read asset", "path", path, "err", err)
		return ""
	}
	return string(content)
}

// favicon draws the logo: the same path data compresses to almost nothing the second
// time, and a data: URL needs no request. On a light browser tab it is dark.
func favicon(logo string) template.HTML {
	svg := strings.NewReplacer(`"`, "'", ` fill='#ddd'`, "", "#", "%23").Replace(logo)
	svg = strings.Replace(svg, "<path", "<style>path{fill:%23ddd}@media(prefers-color-scheme:light){path{fill:%23050505}}</style><path", 1)
	return template.HTML(`<link rel="icon" href="data:image/svg+xml,` + svg + `">`) //nolint:gosec // our own file
}

// NewFuncMap returns the functions available in templates.
func NewFuncMap() template.FuncMap {
	return template.FuncMap{
		// CSS, JS and the logo live in the page: every extra request would be another round trip
		"InlineCSS": func() template.CSS { return template.CSS(readPublic("assets/css/index.css")) },
		"InlineJS": func() template.JS {
			js := readPublic("assets/js/htmx.min.js") + ";" + readPublic("assets/js/index.js")
			return template.JS(strings.ReplaceAll(js, "</script", `<\/script`))
		},
		"InlineLogo":  func() template.HTML { return template.HTML(readPublic("img/logo.svg")) },
		"FaviconLink": func() template.HTML { return favicon(readPublic("img/logo.svg")) },
		// dict builds a map for passing several values to a sub-template:
		// {{template "access/database" dict "Name" .Name "Write" .Write}}
		"dict": func(kv ...any) (map[string]any, error) {
			if len(kv)%2 != 0 {
				return nil, errors.New("dict: odd number of arguments")
			}
			m := make(map[string]any, len(kv)/2)
			for i := 0; i < len(kv); i += 2 {
				k, ok := kv[i].(string)
				if !ok {
					return nil, errors.New("dict: keys must be strings")
				}
				m[k] = kv[i+1]
			}
			return m, nil
		},
		"list": func(items ...string) []string { return items },
		// all students are in Moscow time, as in the course databases
		"TimeMSK": func(unix int64) string { return time.Unix(unix, 0).In(msk).Format("02.01.2006 15:04") + " МСК" },
	}
}
