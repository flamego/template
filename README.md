# template

[![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/flamego/template/go.yml?branch=main&logo=github&style=for-the-badge)](https://github.com/flamego/template/actions?query=workflow%3AGo)
[![Codecov](https://img.shields.io/codecov/c/gh/flamego/template?logo=codecov&style=for-the-badge)](https://app.codecov.io/gh/flamego/template)
[![GoDoc](https://img.shields.io/badge/GoDoc-Reference-blue?style=for-the-badge&logo=go)](https://pkg.go.dev/github.com/flamego/template?tab=doc)
[![Sourcegraph](https://img.shields.io/badge/view%20on-Sourcegraph-brightgreen.svg?style=for-the-badge&logo=sourcegraph)](https://sourcegraph.com/github.com/flamego/template)

Package template is a middleware that provides Go template rendering for [Flamego](https://github.com/flamego/flamego).

## Installation

The minimum requirement of Go is **1.18**.

	go get github.com/flamego/template

## Getting started

```html
<!-- templates/home.tmpl -->
<p>
  Hello, <b>{{.Name}}</b>!
</p>
```

```go
package main

import (
	"net/http"

	"github.com/flamego/flamego"
	"github.com/flamego/template"
)

func main() {
	f := flamego.Classic()
	f.Use(template.Templater())
	f.Get("/", func(t template.Template, data template.Data) {
		data["Name"] = "Joe"
		t.HTML(http.StatusOK, "home")
	})
	f.Run()
}
```

## Getting help

- Read [documentation and examples](https://flamego.dev/middleware/template.html).
- Please [file an issue](https://github.com/flamego/flamego/issues) or [start a discussion](https://github.com/flamego/flamego/discussions) on the [flamego/flamego](https://github.com/flamego/flamego) repository.

## License

This project is under the MIT License. See the [LICENSE](LICENSE) file for the full license text.
