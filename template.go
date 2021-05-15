// Copyright 2021 Flamego. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package template

import (
	"bytes"
	"fmt"
	gotemplate "html/template"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/flamego/flamego"
)

type Template interface {
	HTML(status int, name string)
}

var _ Template = (*template)(nil)

// todo
type template struct {
	flamego.ResponseWriter
	*log.Logger

	*gotemplate.Template
	Data

	contentType string
	bufPool     *sync.Pool
}

func responseServerError(w http.ResponseWriter, err error) {
	if flamego.Env() == flamego.EnvTypeDev {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func (t *template) HTML(status int, name string) {
	buf := t.bufPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		t.bufPool.Put(buf)
	}()

	started := time.Now()
	err := t.ExecuteTemplate(buf, name, t.Data)
	if err != nil {
		responseServerError(t.ResponseWriter, err)
		return
	}
	t.Data["RenderDuration"] = fmt.Sprint(time.Since(started).Nanoseconds()/1e6) + "ms"

	t.ResponseWriter.Header().Set("Content-Type", t.contentType+"; charset=utf-8")
	t.ResponseWriter.WriteHeader(status)

	_, err = buf.WriteTo(t.ResponseWriter)
	if err != nil {
		t.Logger.Printf("template: failed to write out rendered HTML: %v", err)
		return
	}
}

// todo
type Data map[string]interface{}

// todo: Delims represents a set of Left and Right delimiters for HTML template rendering
type Delims struct {
	// Left delimiter, defaults to {{
	Left string
	// Right delimiter, defaults to }}
	Right string
}

type Options struct {
	// todo: Directory to load templates. Default is "templates".
	Directory string
	// todo: Additional directories to overwrite templates.
	AppendDirectories []string
	// todo
	FileSystem FileSystem
	// todo: Extensions to parse template files from. Defaults are [".tmpl", ".html"].
	Extensions []string
	// todo: FuncMaps is a slice of `template.FuncMap` to apply to the template upon compilation. This is useful for helper functions. Default is [].
	FuncMaps []gotemplate.FuncMap
	// todo: Delims sets the action delimiters to the specified strings in the Delims struct.
	Delims Delims
	// todo: Allows changing of output to XHTML instead of HTML. Default is "text/html"
	ContentType string
}

func newTemplate(opts Options) (*gotemplate.Template, error) {
	if opts.FileSystem == nil {
		var err error
		opts.FileSystem, err = newFileSystem(opts.Directory, opts.AppendDirectories, opts.Extensions)
		if err != nil {
			return nil, errors.Wrapf(err, "new file system")
		}
	}

	tpl := gotemplate.New("Flamego.Template").Delims(opts.Delims.Left, opts.Delims.Right)
	for _, f := range opts.FileSystem.Files() {
		t := tpl.New(f.Name())
		for _, funcMap := range opts.FuncMaps {
			t.Funcs(funcMap)
		}

		data, err := f.Data()
		if err != nil {
			return nil, errors.Wrapf(err, "get data of %q", f.Name())
		}
		_, err = t.Parse(string(data))
		if err != nil {
			return nil, errors.Wrapf(err, "parse %q", f.Name())
		}
	}
	return tpl, nil
}

// todo
func Templater(opts ...Options) flamego.Handler {
	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}

	parseOptions := func(opts Options) Options {
		if len(opts.Extensions) == 0 {
			opts.Extensions = []string{".tmpl", ".html"}
		}

		if opts.ContentType == "" {
			opts.ContentType = "text/html"
		}
		return opts
	}

	opt = parseOptions(opt)

	tpl, err := newTemplate(opt)
	if err != nil {
		panic("template: new template: " + err.Error())
	}

	bufPool := &sync.Pool{
		New: func() interface{} { return new(bytes.Buffer) },
	}

	return func(c flamego.Context, log *log.Logger) {
		t := &template{
			ResponseWriter: c.ResponseWriter(),
			Logger:         log,
			Template:       tpl,
			Data:           make(Data),
			contentType:    opt.ContentType,
			bufPool:        bufPool,
		}

		if flamego.Env() == flamego.EnvTypeDev {
			tpl, err := newTemplate(opt)
			if err != nil {
				http.Error(
					c.ResponseWriter(),
					fmt.Sprintf("template: new template: %v", err),
					http.StatusInternalServerError,
				)
				return
			}
			t.Template = tpl
		}

		c.MapTo(t, (*Template)(nil))
		c.Map(t.Data)
	}
}
