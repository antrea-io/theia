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

package utils

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-logr/logr"
)

func DownloadAndUntar(ctx context.Context, logger logr.Logger, url string, dir string, filename string, untar bool) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if untar {
		gzr, err := gzip.NewReader(resp.Body)
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
	} else {
		dest := filepath.Join(dir, filename)
		logger.V(4).Info("Downloading", "path", dest)
		file, err := os.Create(dest)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(file, resp.Body)
		return err
	}
}
