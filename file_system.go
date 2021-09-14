// Copyright 2021 Flamego. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package template

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

// File is a template file that contains name, data and its extension.
type File interface {
	// Name returns the name of the file, stripping its extension. It should return
	// "home" not "home.tmpl".
	Name() string
	// Data returns the file content.
	Data() ([]byte, error)
	// Ext returns the file extension, carrying the dot ("."). It should return
	// ".tmpl" not "tmpl".
	Ext() string
}

// FileSystem is a template file system consists a list of template files.
type FileSystem interface {
	// Files returns the the list of template files.
	Files() []File
}

type file struct {
	name string
	data []byte
	ext  string
}

func (f *file) Name() string          { return f.name }
func (f *file) Data() ([]byte, error) { return f.data, nil }
func (f *file) Ext() string           { return f.ext }

type fileSystem struct {
	files []File
}

func (fs *fileSystem) Files() []File { return fs.files }

// isDir returns true if given path is a directory, and returns false when it's
// a file or does not exist.
func isDir(dir string) bool {
	f, e := os.Stat(dir)
	if e != nil {
		return false
	}
	return f.IsDir()
}

// isFile returns true if given path exists as a file (i.e. not a directory).
func isFile(path string) bool {
	f, e := os.Stat(path)
	if e != nil {
		return false
	}
	return !f.IsDir()
}

// getExt returns the extension of given name, prefixed with the dot (".").
func getExt(name string) string {
	i := strings.Index(name, ".")
	if i == -1 {
		return ""
	}
	return name[i:]
}

// newFileSystem constructs and returns a FileSystem from local disk.
func newFileSystem(dir string, allowedExtensions []string) (FileSystem, error) {
	var files []File
	err := filepath.WalkDir(dir, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		ext := getExt(path)
		for _, allowed := range allowedExtensions {
			if ext != allowed {
				continue
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return errors.Wrap(err, "read")
			}

			relpath, err := filepath.Rel(dir, path)
			if err != nil {
				return errors.Wrap(err, "get relative path")
			}

			name := filepath.ToSlash(relpath[:len(relpath)-len(ext)])
			files = append(files,
				&file{
					name: name,
					data: data,
					ext:  ext,
				},
			)
			break
		}
		return nil
	})
	if err != nil {
		return nil, errors.Wrapf(err, "walk %q", dir)
	}
	return &fileSystem{
		files: files,
	}, nil
}

// EmbedFS wraps the given embed.FS into a FileSystem.
func EmbedFS(efs embed.FS, dir string, allowedExtensions []string) (FileSystem, error) {
	var files []File
	err := fs.WalkDir(efs, dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		relpath, err := filepath.Rel(dir, path)
		if err != nil {
			return errors.Wrap(err, "get relative path")
		}

		ext := getExt(relpath)
		for _, allowed := range allowedExtensions {
			if ext != allowed {
				continue
			}

			data, err := efs.ReadFile(path)
			if err != nil {
				return errors.Wrap(err, "read")
			}

			name := filepath.ToSlash(relpath[:len(relpath)-len(ext)])
			files = append(files,
				&file{
					name: name,
					data: data,
					ext:  ext,
				},
			)
		}

		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "walk")
	}

	return &fileSystem{
		files: files,
	}, nil
}
