// Copyright 2021 Flamego. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package template

import (
	"bytes"
	gotemplate "html/template"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/flamego/flamego"
)

func TestTemplate_HTML(t *testing.T) {
	tests := []struct {
		name string
		opts Options
		want string
	}{
		{
			name: "basic",
			opts: Options{
				Directory: "testdata/basic",
				FuncMaps: []gotemplate.FuncMap{
					{"Year": func() int { return 2021 }},
				},
			},
			want: `
<header>This is a header</header>
<p>
  Hello, Flamego!
</p>
<footer>2021</footer>
`,
		},
		{
			name: "overwrite",
			opts: Options{
				Directory:         "testdata/overwrite/primary",
				AppendDirectories: []string{"testdata/overwrite/append"},
			},
			want: `
<header>The header is overwritten</header>
<p>
  Hello, Flamego!
</p>
`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := flamego.NewWithLogger(&bytes.Buffer{})
			f.Use(Templater(test.opts))
			f.Get("/", func(t Template, data Data) {
				data["Name"] = "Flamego"
				t.HTML(http.StatusOK, "home")
			})

			resp := httptest.NewRecorder()
			req, err := http.NewRequest(http.MethodGet, "/", nil)
			assert.Nil(t, err)

			f.ServeHTTP(resp, req)

			assert.Equal(t, http.StatusOK, resp.Code)
			assert.Equal(t, "text/html; charset=utf-8", resp.Header().Get("Content-Type"))

			want := test.want
			if runtime.GOOS == "windows" {
				want = strings.ReplaceAll(want, "\n", "\r\n")
			}
			assert.Equal(t, want, resp.Body.String())
		})
	}
}
