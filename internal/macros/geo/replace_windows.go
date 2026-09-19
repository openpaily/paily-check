//go:build windows

package geo

import (
	"syscall"
	"unsafe"
)

const movefileReplaceExisting = 0x1

var moveFileExW = syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")

func replaceFile(src, dst string) error {
	srcPtr, err := syscall.UTF16PtrFromString(src)
	if err != nil {
		return err
	}
	dstPtr, err := syscall.UTF16PtrFromString(dst)
	if err != nil {
		return err
	}
	r1, _, err := moveFileExW.Call(
		uintptr(unsafe.Pointer(srcPtr)),
		uintptr(unsafe.Pointer(dstPtr)),
		uintptr(movefileReplaceExisting),
	)
	if r1 == 0 {
		return err
	}
	return nil
}
