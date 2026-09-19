//go:build !windows

package geo

import "os"

func replaceFile(src, dst string) error {
	return os.Rename(src, dst)
}
