package config

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/vinegarhq/vinegar/internal/dirs"
)

type WineCandidate struct {
	Name   string
	Root   string
	Binary string
}

var wineBinaryNames = []string{
	"wine64",
	"wine",
	"wine-staging",
	"wine-development",
	"wine-tkg",
	"wine-ge",
}

// ValidateWineRoot verifies that root points to a usable Wine or Proton
// installation. Empty root is valid and means Wine should be resolved from PATH.
func ValidateWineRoot(root string) error {
	if root == "" {
		return nil
	}
	if !filepath.IsAbs(root) {
		return ErrWineRootAbs
	}
	if _, err := wineBinary(root); err == nil {
		return nil
	}
	return ErrWineRootInvalid
}

// DetectWineInstallations returns Wine/Proton installations Vinegar can launch.
// PATH candidates use an empty Root, matching wine.New's system Wine behavior.
func DetectWineInstallations() []WineCandidate {
	out := make([]WineCandidate, 0, len(wineBinaryNames))
	seen := map[string]bool{}

	for _, name := range wineBinaryNames {
		bin, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		if real, err := filepath.EvalSymlinks(bin); err == nil {
			bin = real
		}
		if seen[bin] {
			continue
		}
		seen[bin] = true
		out = append(out, WineCandidate{
			Name:   name,
			Root:   "",
			Binary: bin,
		})
		break
	}

	for _, root := range commonWineRoots() {
		bin, err := wineBinary(root)
		if err != nil {
			continue
		}
		if seen[bin] {
			continue
		}
		seen[bin] = true
		out = append(out, WineCandidate{
			Name:   filepath.Base(root),
			Root:   root,
			Binary: bin,
		})
	}

	return out
}

func ResolveWineRoot(configured string) (string, WineCandidate, error) {
	if configured != "" {
		if err := ValidateWineRoot(configured); err != nil {
			return "", WineCandidate{}, fmt.Errorf("%s: %w", configured, err)
		}
		bin, _ := wineBinary(configured)
		return configured, WineCandidate{
			Name:   filepath.Base(configured),
			Root:   configured,
			Binary: bin,
		}, nil
	}

	candidates := DetectWineInstallations()
	if len(candidates) == 0 {
		return "", WineCandidate{}, fmt.Errorf("system Wine not found: %w", ErrWineRootInvalid)
	}
	return candidates[0].Root, candidates[0], nil
}

func wineBinary(root string) (string, error) {
	for _, bin := range []string{
		filepath.Join(root, "bin", "wine64"),
		filepath.Join(root, "bin", "wine"),
		filepath.Join(root, "files", "bin", "wine64"),
		filepath.Join(root, "files", "bin", "wine"),
		filepath.Join(root, "proton"),
	} {
		info, err := os.Stat(bin)
		if err == nil && !info.IsDir() {
			return bin, nil
		}
	}
	return "", ErrWineRootInvalid
}

func commonWineRoots() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	patterns := []string{
		filepath.Join(home, ".local/share/Steam/steamapps/common/Proton*"),
		filepath.Join(home, ".steam/steam/steamapps/common/Proton*"),
		filepath.Join(home, ".var/app/com.valvesoftware.Steam/data/Steam/steamapps/common/Proton*"),
		filepath.Join(home, ".local/share/lutris/runners/wine/*"),
		filepath.Join(home, ".var/app/net.lutris.Lutris/data/lutris/runners/wine/*"),
		filepath.Join(home, ".local/share/bottles/runners/*"),
	}

	var roots []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		roots = append(roots, matches...)
	}
	if _, err := os.Stat(dirs.WinePath); err == nil {
		roots = append([]string{dirs.WinePath}, roots...)
	}

	slices.SortFunc(roots, func(a, b string) int {
		return strings.Compare(strings.ToLower(filepath.Base(a)), strings.ToLower(filepath.Base(b)))
	})
	return slices.Compact(roots)
}

func IsWineRootError(err error) bool {
	return errors.Is(err, ErrWineRootAbs) || errors.Is(err, ErrWineRootInvalid)
}
