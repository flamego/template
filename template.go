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

// Template is a Go template rendering engine.
type Template interface {
	// HTML renders the named template with the given status.
	HTML(status int, name string)
}

var _ Template = (*template)(nil)

type template struct {
	responseWriter flamego.ResponseWriter
	logger         *log.Logger

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
	t.Data["RenderDuration"] = func() string {
		return fmt.Sprint(time.Since(started).Nanoseconds()/1e6) + "ms"
	}

	err := t.ExecuteTemplate(buf, name, t.Data)
	if err != nil {
		responseServerError(t.responseWriter, err)
		return
	}

	t.responseWriter.Header().Set("Content-Type", t.contentType+"; charset=utf-8")
	t.responseWriter.WriteHeader(status)

	_, err = buf.WriteTo(t.responseWriter)
	if err != nil {
		t.logger.Printf("template: failed to write out rendered HTML: %v", err)
		return
	}
}

// Data is used as the root object for rendering a template.
type Data map[string]interface{}

// Delims is a pair of Left and Right delimiters for rendering HTML templates.
type Delims struct {
	// Left is the left delimiter. Default is "{{".
	Left string
	// Right is the right delimiter. Default is "}}".
	Right string
}

// Options contains options for the template.Templater middleware.
type Options struct {
	// Directory is the primary directory to load templates. This value is ignored
	// when FileSystem is set. Default is "templates".
	Directory string
	// AppendDirectories is a list of additional directories to load templates for
	// overwriting templates that are loaded from Directory. This value is ignored
	// when FileSystem is set.
	AppendDirectories []string
	// FileSystem is the interface for supporting any implementation of the
	// FileSystem.
	FileSystem FileSystem
	// Extensions is a list of extensions to be used for template files. Default is
	// `[".tmpl", ".html"]`.
	Extensions []string
	// FuncMaps is a list of `template.FuncMap` to be applied for rendering
	// templates.
	FuncMaps []gotemplate.FuncMap
	// Delims is the pair of left and right delimiters for rendering templates.
	Delims Delims
	// ContentType specifies the value of "Content-Type". Default is "text/html".
	ContentType string
}

func newTemplate(opts Options) (*gotemplate.Template, error) {
	if opts.Directory == "" {
		opts.Directory = "templates"
	}

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

// Templater returns a middleware handler that injects template.Templater and
// template.Data into the request context, which are used for rendering
// templates to the ResponseWriter.
//
// When running with flamego.EnvTypeDev and no FileSystem is specified,
// templates will be recompiled upon every request.
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
		panic("template: " + err.Error())
	}

	bufPool := &sync.Pool{
		New: func() interface{} { return new(bytes.Buffer) },
	}

	return flamego.LoggerInvoker(func(c flamego.Context, log *log.Logger) {
		t := &template{
			responseWriter: c.ResponseWriter(),
			logger:         log,
			Template:       tpl,
			Data:           make(Data),
			contentType:    opt.ContentType,
			bufPool:        bufPool,
		}

		if flamego.Env() == flamego.EnvTypeDev && opt.FileSystem == nil {
			tpl, err := newTemplate(opt)
			if err != nil {
				http.Error(
					c.ResponseWriter(),
					fmt.Sprintf("template: %v", err),
					http.StatusInternalServerError,
				)
				return
			}
			t.Template = tpl
		}

		c.MapTo(t, (*Template)(nil))
		c.Map(t.Data)
	})
}
