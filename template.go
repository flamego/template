// Copyright 2021 Flamego. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package template

import (
	"bytes"
	"fmt"
	gotemplate "html/template"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/charmbracelet/log"
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

func (t *template) responseServerError(w http.ResponseWriter, err error) {
	t.logger.Error("rendering", "error", err)
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
		t.responseServerError(t.responseWriter, err)
		return
	}

	t.responseWriter.Header().Set("Content-Type", t.contentType+"; charset=utf-8")
	t.responseWriter.WriteHeader(status)

	_, err = buf.WriteTo(t.responseWriter)
	if err != nil {
		t.logger.Error("[template] Failed to write out rendered HTML", "error", err)
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
	// FileSystem is the interface for supporting any implementation of the
	// FileSystem.
	FileSystem FileSystem
	// Directory is the primary directory to load templates. This value is ignored
	// when FileSystem is set. Default is "templates".
	Directory string
	// AppendDirectories is a list of additional directories to load templates for
	// overwriting templates that are loaded from FileSystem or Directory.
	AppendDirectories []string
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

func newTemplate(allowedExtensions []string, funcMaps []gotemplate.FuncMap, delmis Delims, fs FileSystem, dir string, others ...string) (*gotemplate.Template, error) {
	if fs == nil {
		var err error
		fs, err = newFileSystem(dir, allowedExtensions)
		if err != nil {
			return nil, errors.Wrapf(err, "new file system")
		}
	}

	// Directories are composed in the reverse order because later ones overwrites
	// previous ones. Therefore, we can simply break of the loop once found an
	// overwritten when looping in the reverse order.
	dirs := make([]string, 0, len(others))
	for i := len(others) - 1; i >= 0; i-- {
		dirs = append(dirs, others[i])
	}

	for i := range dirs {
		if !isDir(dirs[i]) {
			continue
		}

		var err error
		dirs[i], err = filepath.EvalSymlinks(dirs[i])
		if err != nil {
			return nil, errors.Wrapf(err, "eval symlinks for %q", dirs[i])
		}
	}

	tpl := gotemplate.New("Flamego.Template").Delims(delmis.Left, delmis.Right)
	for _, f := range fs.Files() {
		t := tpl.New(f.Name())
		for _, funcMap := range funcMaps {
			t.Funcs(funcMap)
		}

		var err error
		var data []byte

		// Loop over append directories and break out once found.
		for _, dir := range dirs {
			fpath := filepath.Join(dir, f.Name()+f.Ext())
			if !isFile(fpath) {
				continue
			}

			data, err = os.ReadFile(fpath)
			if err != nil {
				return nil, errors.Wrap(err, "read")
			}
			break
		}

		if len(data) == 0 {
			data, err = f.Data()
			if err != nil {
				return nil, errors.Wrapf(err, "get data of %q", f.Name())
			}
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
// When running with flamego.EnvTypeDev, if either Directory or
// AppendDirectories is specified, templates will be recompiled upon every
// request.
func Templater(opts ...Options) flamego.Handler {
	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}

	parseOptions := func(opts Options) Options {
		if opts.Directory == "" {
			opts.Directory = "templates"
		}

		if len(opts.Extensions) == 0 {
			opts.Extensions = []string{".tmpl", ".html"}
		}

		if opts.ContentType == "" {
			opts.ContentType = "text/html"
		}
		return opts
	}

	opt = parseOptions(opt)

	tpl, err := newTemplate(opt.Extensions, opt.FuncMaps, opt.Delims, opt.FileSystem, opt.Directory, opt.AppendDirectories...)
	if err != nil {
		panic("template: new template: " + err.Error())
	}

	bufPool := &sync.Pool{
		New: func() interface{} { return new(bytes.Buffer) },
	}

	return flamego.LoggerInvoker(func(c flamego.Context, logger *log.Logger) {
		t := &template{
			responseWriter: c.ResponseWriter(),
			logger:         logger.WithPrefix("template"),
			Template:       tpl,
			Data:           make(Data),
			contentType:    opt.ContentType,
			bufPool:        bufPool,
		}

		if flamego.Env() == flamego.EnvTypeDev &&
			(opt.Directory != "" || len(opt.AppendDirectories) > 0) {
			tpl, err := newTemplate(opt.Extensions, opt.FuncMaps, opt.Delims, opt.FileSystem, opt.Directory, opt.AppendDirectories...)
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
