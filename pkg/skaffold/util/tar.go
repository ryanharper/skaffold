/*
Copyright 2019 The Skaffold Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/ryanharper/skaffold/v2/pkg/skaffold/constants"
	"github.com/ryanharper/skaffold/v2/pkg/skaffold/output/log"
	timeutil "github.com/ryanharper/skaffold/v2/pkg/skaffold/util/time"
)

type headerModifier func(*tar.Header)

type cancelableWriter struct {
	w   io.Writer
	ctx context.Context
}

func (cw *cancelableWriter) Write(p []byte) (n int, err error) {
	select {
	case <-cw.ctx.Done():
		return 0, cw.ctx.Err()
	default:
		return cw.w.Write(p)
	}
}

func CreateMappedTar(ctx context.Context, w io.Writer, root string, pathMap map[string][]string) error {
	tw := tar.NewWriter(w)
	defer tw.Close()

	for src, dsts := range pathMap {
		for _, dst := range dsts {
			if err := addFileToTar(ctx, root, src, dst, tw, nil); err != nil {
				return err
			}
		}
	}

	return nil
}

func CreateTar(ctx context.Context, w io.Writer, root string, paths []string) error {
	tw := tar.NewWriter(w)
	defer tw.Close()

	batchSize := len(paths) / 10
	if batchSize < 10 {
		batchSize = 5
	}

	log.Entry(ctx).Infof("Creating tar file from %d file(s)", len(paths))
	start := time.Now()
	defer func() {
		log.Entry(ctx).Infof("Creating tar file completed in %s", timeutil.Humanize(time.Since(start)))
	}()

	for i, path := range paths {
		if err := addFileToTar(ctx, root, path, "", tw, nil); err != nil {
			return err
		}

		if (i+1)%batchSize == 0 {
			log.Entry(ctx).Infof("Added %d/%d files to tar file", i+1, len(paths))
		}
	}

	return nil
}

func CreateTarWithParents(ctx context.Context, w io.Writer, root string, paths []string, uid, gid int, modTime time.Time) error {
	headerModifier := func(header *tar.Header) {
		header.ModTime = modTime
		header.Uid = uid
		header.Gid = gid
		header.Uname = ""
		header.Gname = ""
	}

	tw := tar.NewWriter(w)
	defer tw.Close()

	// Make sure parent folders are added before files
	// TODO(dgageot): this should probably also be done in CreateTar
	// but I'd rather not break things that people didn't complain about!
	added := map[string]bool{}

	for _, path := range paths {
		var parentsFirst []string
		for p := path; p != "." && !added[p]; p = filepath.Dir(p) {
			parentsFirst = append(parentsFirst, p)
			added[p] = true
		}

		for i := len(parentsFirst) - 1; i >= 0; i-- {
			if err := addFileToTar(ctx, root, parentsFirst[i], "", tw, headerModifier); err != nil {
				return err
			}
		}
	}

	return nil
}

func CreateTarGz(ctx context.Context, w io.Writer, root string, paths []string) error {
	gw := gzip.NewWriter(w)
	defer gw.Close()
	return CreateTar(ctx, gw, root, paths)
}

func addFileToTar(ctx context.Context, root string, src string, dst string, tw *tar.Writer, hm headerModifier) error {
	fi, err := os.Lstat(src)
	if err != nil {
		return err
	}

	mode := fi.Mode()
	if mode&os.ModeSocket != 0 {
		return nil
	}

	var header *tar.Header
	if mode&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}

		if filepath.IsAbs(target) {
			log.Entry(ctx).Warnf("Skipping %s. Only relative symlinks are supported.", src)
			return nil
		}

		header, err = tar.FileInfoHeader(fi, target)
		if err != nil {
			return err
		}
	} else {
		header, err = tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}
	}

	if dst == "" {
		tarPath, err := filepath.Rel(root, src)
		if err != nil {
			return err
		}

		header.Name = filepath.ToSlash(tarPath)
	} else {
		header.Name = filepath.ToSlash(dst)
	}

	// Code copied from https://github.com/moby/moby/blob/master/pkg/archive/archive_windows.go
	if runtime.GOOS == constants.Windows {
		header.Mode = int64(chmodTarEntry(os.FileMode(header.Mode)))
	}
	if hm != nil {
		hm(header)
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	if mode.IsRegular() {
		f, err := os.Open(src)
		if err != nil {
			return err
		}
		defer f.Close()

		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Wrap the tar.Writer in a cancelableWriter that checks the context
		cw := &cancelableWriter{w: tw, ctx: ctx}

		// Proceed with copying the file content using the cancelable writer
		if _, err := io.Copy(cw, f); err != nil {
			return fmt.Errorf("writing real file %q: %w", src, err)
		}
	}

	return nil
}

// Code copied from https://github.com/moby/moby/blob/master/pkg/archive/archive_windows.go
func chmodTarEntry(perm os.FileMode) os.FileMode {
	// perm &= 0755 // this 0-ed out tar flags (like link, regular file, directory marker etc.)
	permPart := perm & os.ModePerm
	noPermPart := perm &^ os.ModePerm
	// Add the x bit: make everything +x from windows
	permPart |= 0111
	permPart &= 0755

	return noPermPart | permPart
}
