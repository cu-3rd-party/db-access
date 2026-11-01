package templates

import (
	"errors"
	"html/template"
	"time"
)

// AssetVersion busts browser caches of /assets between builds.
var AssetVersion = time.Now().Format("20060102150405")

var msk = time.FixedZone("MSK", 3*60*60)

// NewFuncMap returns the functions available in templates.
func NewFuncMap() template.FuncMap {
	return template.FuncMap{
		"AssetURL": func(path string) string { return "/assets/" + path + "?v=" + AssetVersion },
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
