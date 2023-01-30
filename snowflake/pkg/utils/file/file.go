// Copyright 2022 Antrea Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package file

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
)

func Download(ctx context.Context, logger logr.Logger, url string, dir string, filename string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if filename == "" {
		filename = path.Base(req.URL.Path)
	}
	dest := filepath.Join(dir, filename)
	logger.V(4).Info("Downloading", "path", dest)
	file, err := os.Create(dest)
	if err != nil {
		return "", err
	}
	defer file.Close()
	_, err = io.Copy(file, resp.Body)
	return dest, err
}

func DownloadAndUntar(ctx context.Context, logger logr.Logger, url string, dir string) error {
	tarFilepath, err := Download(ctx, logger, url, dir, "")
	if err != nil {
		return err
	}
	f, err := os.Open(tarFilepath)
	if err != nil {
		return err
	}
	defer f.Close()
	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return err
		}
		dest := filepath.Join(dir, hdr.Name)
		logger.V(4).Info("Untarring", "path", hdr.Name)
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if err := func() error {
			f, err := os.OpenFile(dest, os.O_CREATE|os.O_RDWR, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			defer f.Close()

			// copy over contents
			if _, err := io.Copy(f, tr); err != nil {
				return err
			}
			return nil
		}(); err != nil {
			return err
		}
	}
	return nil
}

func WriteFSDirToDisk(fsys fs.FS, fsysPath string, dest string) error {
	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}

	return fs.WalkDir(fsys, fsysPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		outpath := filepath.Join(dest, strings.TrimPrefix(path, fsysPath))

		if d.IsDir() {
			os.MkdirAll(outpath, 0755)
			return nil
		}

		in, err := fsys.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()

		out, err := os.Create(outpath)
		if err != nil {
			return err
		}
		defer out.Close()

		_, err = io.Copy(out, in)
		if err != nil {
			return err
		}
		return nil
	})
}
