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

type File interface {
	Name() string
	Data() ([]byte, error)
	Ext() string
}

// todo
type FileSystem interface {
	Files() []File
}

type file struct {
	name string
	data []byte
	ext  string
}

func (f *file) Name() string {
	return f.name
}

func (f *file) Data() ([]byte, error) {
	return f.data, nil
}

func (f *file) Ext() string {
	return f.ext
}

type fileSystem struct {
	files []File
}

func (fs *fileSystem) Files() []File {
	return fs.files
}

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

func getExt(name string) string {
	i := strings.Index(name, ".")
	if i == -1 {
		return ""
	}
	return name[i:]
}

func newFileSystem(primaryDir string, appendDirs, allowedExtensions []string) (FileSystem, error) {
	// Directories are composed in the reverse order because later ones overwrites
	// previous ones. Therefore, we can simply break of the loop once found an
	// overwrite when looping in the reverse order.
	dirs := make([]string, 0, len(appendDirs)+1)
	for i := len(appendDirs) - 1; i >= 0; i-- {
		dirs = append(dirs, appendDirs[i])
	}
	dirs = append(dirs, primaryDir)

	var err error
	for i := range dirs {
		if !isDir(dirs[i]) {
			continue
		}

		dirs[i], err = filepath.EvalSymlinks(dirs[i])
		if err != nil {
			return nil, errors.Wrapf(err, "eval symlinks for %q", dirs[i])
		}
	}

	// Walk the primary directory because it is non-sense to load templates not even
	// exist in the primary directory.
	var files []File
	err = filepath.WalkDir(primaryDir, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relpath, err := filepath.Rel(primaryDir, path)
		if err != nil {
			return errors.Wrap(err, "get relative path")
		}

		ext := getExt(relpath)
		for _, allowed := range allowedExtensions {
			if ext != allowed {
				continue
			}

			// Loop over append directories and break out once found. The file is guaranteed
			// to exist because otherwise the code won't be executed, and read file from the
			// primary directory is the ultimate fallback.
			var data []byte
			for _, dir := range dirs {
				fpath := filepath.Join(dir, relpath)
				if !isFile(fpath) {
					continue
				}

				data, err = os.ReadFile(fpath)
				if err != nil {
					return errors.Wrap(err, "read")
				}
				break
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
		return nil, errors.Wrapf(err, "walk %q", primaryDir)
	}
	return &fileSystem{
		files: files,
	}, nil
}

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
