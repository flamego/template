// Copyright 2021 Flamego. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package template

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/flamego/flamego"
)

func TestTemplate_HTML(t *testing.T) {
	f := flamego.NewWithLogger(&bytes.Buffer{})
	f.Use(Templater(
		Options{
			Directory: "testdata/basic",
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
`
	assert.Equal(t, want, resp.Body.String())
}
