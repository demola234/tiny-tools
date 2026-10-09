package docs

import (
	"bytes"
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
)

//go:embed swaggerui/swagger-ui-bundle.js swaggerui/swagger-ui.css swaggerui/LICENSE swaggerui/NOTICE
var assets embed.FS

var page = template.Must(template.New("index").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.}}</title>
<link rel="stylesheet" href="swagger-ui.css">
</head>
<body>
<div id="ui"></div>
<script src="swagger-ui-bundle.js"></script>
<script>SwaggerUIBundle({ url: "openapi.yaml", dom_id: "#ui", deepLinking: true });</script>
</body>
</html>
`))

var types = map[string]string{
	"/swagger-ui-bundle.js": "text/javascript; charset=utf-8",
	"/swagger-ui.css":       "text/css; charset=utf-8",
}

func index(title string) []byte {
	var b bytes.Buffer
	_ = page.Execute(&b, title)
	return b.Bytes()
}

func Handler(title string, spec func() ([]byte, error)) http.Handler {
	html := index(title)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/", "/index.html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(html)
		case "/openapi.yaml":
			data, err := spec()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/yaml")
			_, _ = w.Write(data)
		default:
			ctype, ok := types[r.URL.Path]
			if !ok {
				http.NotFound(w, r)
				return
			}
			data, _ := assets.ReadFile("swaggerui" + r.URL.Path)
			w.Header().Set("Content-Type", ctype)
			_, _ = w.Write(data) //nolint:gosec // one of two bundled files, picked from a fixed list
		}
	})
}

func Write(dir, title string, spec []byte) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	files := map[string][]byte{"index.html": index(title), "openapi.yaml": spec}
	entries, err := fs.ReadDir(assets, "swaggerui")
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		data, err := assets.ReadFile("swaggerui/" + e.Name())
		if err != nil {
			return nil, err
		}
		files[e.Name()] = data
	}
	written := make([]string, 0, len(files))
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			return written, err
		}
		written = append(written, name)
	}
	return written, nil
}
