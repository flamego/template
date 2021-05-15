// Copyright 2021 Flamego. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package template

import (
	"bytes"
	"embed"
	gotemplate "html/template"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/flamego/flamego"
)

//go:embed testdata/basic/*
var templates embed.FS

func TestEmbedFS(t *testing.T) {
	fs, err := EmbedFS(templates, "testdata/basic", []string{".tmpl"})
	assert.Nil(t, err)

	f := flamego.NewWithLogger(&bytes.Buffer{})
	f.Use(Templater(
		Options{
			FileSystem: fs,
			FuncMaps: []gotemplate.FuncMap{
				{"Year": func() int { return 2021 }},
			},
		},
	))
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

	want := `
<header>This is a header</header>
<p>
  Hello, Flamego!
</p>
<footer>2021</footer>
`
	if runtime.GOOS == "windows" {
		want = strings.ReplaceAll(want, "\n", "\r\n")
	}
	assert.Equal(t, want, resp.Body.String())
}
