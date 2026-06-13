package netutil

import (
	"archive/tar"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ulikunitz/xz"
)

// ExtractURL will decompress the given XZ compressed tarball URL
// into path.
func ExtractURL(url string, dir string) error {
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %s", ErrBadStatus, resp.Status)
	}

	xz, err := xz.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("xz: %w", err)
	}
	r := tar.NewReader(xz)

	for {
		h, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}

		dest, err := secureJoin(dir, h.Name)
		if err != nil {
			return err
		}

		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dest, os.FileMode(h.Mode)); err != nil {
				return fmt.Errorf("mkdir: %w", err)
			}
			continue
		case tar.TypeSymlink:
			if err := secureLinkTarget(dir, dest, h.Linkname); err != nil {
				return err
			}
			if err := os.Symlink(h.Linkname, dest); err != nil {
				return err
			}
			continue
		case tar.TypeLink:
			src, err := secureJoin(dir, h.Linkname)
			if err != nil {
				return err
			}
			if err := os.Link(src, dest); err != nil {
				return err
			}
			continue
		case tar.TypeReg:
		default:
			continue
		}

		err = func() error {
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return fmt.Errorf("mkdir parent: %w", err)
			}

			f, err := os.OpenFile(dest,
				os.O_WRONLY|os.O_CREATE|os.O_TRUNC, h.FileInfo().Mode())
			if err != nil {
				return fmt.Errorf("create: %w", err)
			}
			defer f.Close()

			_, err = io.Copy(f, r)
			if err != nil {
				return fmt.Errorf("copy: %w", err)
			}

			if err := os.Chtimes(dest, h.AccessTime, h.ModTime); err != nil {
				return err
			}

			return nil
		}()
		if err != nil {
			return err
		}
	}

	return nil
}

func secureJoin(dir, name string) (string, error) {
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("illegal package file path: %s", name)
	}

	dest := filepath.Join(dir, name)
	base := filepath.Clean(dir)
	if dest != base && !strings.HasPrefix(dest, base+string(os.PathSeparator)) {
		return "", fmt.Errorf("illegal package file path: %s", dest)
	}

	return dest, nil
}

func secureLinkTarget(root, linkPath, target string) error {
	if filepath.IsAbs(target) {
		return fmt.Errorf("illegal package file path: %s", target)
	}

	dest := filepath.Clean(filepath.Join(filepath.Dir(linkPath), target))
	base := filepath.Clean(root)
	if dest != base && !strings.HasPrefix(dest, base+string(os.PathSeparator)) {
		return fmt.Errorf("illegal package file path: %s", dest)
	}

	return nil
}
