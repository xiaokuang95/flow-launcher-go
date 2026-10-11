//go:build windows

package core

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

// The Win32 pieces the icon of a file is read with.
var (
	shell32                = syscall.NewLazyDLL("shell32.dll")
	user32                 = syscall.NewLazyDLL("user32.dll")
	gdi32                  = syscall.NewLazyDLL("gdi32.dll")
	procSHGetFileInfoW     = shell32.NewProc("SHGetFileInfoW")
	procDestroyIcon        = user32.NewProc("DestroyIcon")
	procGetIconInfo        = user32.NewProc("GetIconInfo")
	procGetObjectW         = gdi32.NewProc("GetObjectW")
	procGetDIBits          = gdi32.NewProc("GetDIBits")
	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
)

const (
	shgfiIcon              = 0x000000100
	shgfiLargeIcon         = 0x000000000
	shgfiSmallIcon         = 0x000000001
	shgfiUseFileAttributes = 0x000000010
	diNormal               = 0x00000003
	dibRGBColors           = 0
	biRGB                  = 0
	iconSize               = 32
)

type shFileInfo struct {
	hIcon         uintptr
	iIcon         int32
	dwAttributes  uint32
	szDisplayName [260]uint16
	szTypeName    [80]uint16
}

type iconInfo struct {
	fIcon    int32
	xHotspot uint32
	yHotspot uint32
	hbmMask  uintptr
	hbmColor uintptr
}

type bitmap struct {
	bmType       int32
	bmWidth      int32
	bmHeight     int32
	bmWidthBytes int32
	bmPlanes     uint16
	bmBitsPixel  uint16
	bmBits       uintptr
}

type bitmapInfoHeader struct {
	biSize          uint32
	biWidth         int32
	biHeight        int32
	biPlanes        uint16
	biBitCount      uint16
	biCompression   uint32
	biSizeImage     uint32
	biXPelsPerMeter int32
	biYPelsPerMeter int32
	biClrUsed       uint32
	biClrImportant  uint32
}

// windowsIcons reads the icon of a path with the shell, the way Explorer
// shows it, so that a shortcut, an executable, a document and a folder each
// show what the user expects.
type windowsIcons struct {
	mu    sync.Mutex
	cache map[string][]byte
}

// FileIcon returns the icon of a path as PNG bytes.
func (w *windowsIcons) FileIcon(path string) []byte {
	w.mu.Lock()
	if cached, ok := w.cache[path]; ok {
		w.mu.Unlock()
		return cached
	}
	w.mu.Unlock()

	icon := w.readIcon(path)

	w.mu.Lock()
	w.cache[path] = icon
	w.mu.Unlock()
	return icon
}

// LargeIconSize is the size the shell is asked for.
func (w *windowsIcons) LargeIconSize() int { return iconSize }

func (w *windowsIcons) readIcon(path string) []byte {
	absolute, err := filepath.Abs(path)
	if err != nil {
		absolute = path
	}
	pathPointer, err := syscall.UTF16PtrFromString(absolute)
	if err != nil {
		return nil
	}

	var info shFileInfo
	flags := uintptr(shgfiIcon | shgfiLargeIcon)
	ret, _, _ := procSHGetFileInfoW.Call(
		uintptr(unsafe.Pointer(pathPointer)),
		0,
		uintptr(unsafe.Pointer(&info)),
		unsafe.Sizeof(info),
		flags,
	)
	if ret == 0 || info.hIcon == 0 {
		return nil
	}
	defer procDestroyIcon.Call(info.hIcon)

	return hiconToPNG(info.hIcon)
}

// hiconToPNG turns the icon the shell handed back into PNG bytes, drawing
// its mask over the colour bitmap where the icon has no alpha channel.
func hiconToPNG(hicon uintptr) []byte {
	var info iconInfo
	ret, _, _ := procGetIconInfo.Call(hicon, uintptr(unsafe.Pointer(&info)))
	if ret == 0 {
		return nil
	}
	defer func() {
		if info.hbmColor != 0 {
			procDeleteObject.Call(info.hbmColor)
		}
		if info.hbmMask != 0 {
			procDeleteObject.Call(info.hbmMask)
		}
	}()

	var bm bitmap
	ret, _, _ = procGetObjectW.Call(
		info.hbmColor,
		unsafe.Sizeof(bm),
		uintptr(unsafe.Pointer(&bm)),
	)
	if ret == 0 {
		return nil
	}

	width, height := int(bm.bmWidth), int(bm.bmHeight)
	if width <= 0 || height <= 0 {
		return nil
	}

	colour := readBitmapPixels(info.hbmColor, width, height)
	if colour == nil {
		return nil
	}

	mask := readBitmapPixels(info.hbmMask, width, height)
	picture := image.NewRGBA(image.Rect(0, 0, width, height))

	// An icon drawn with an alpha channel needs no mask; one drawn without
	// uses the mask to say which pixels show.
	hasAlpha := false
	for i := 3; i < len(colour); i += 4 {
		if colour[i] != 0 {
			hasAlpha = true
			break
		}
	}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := (y*width + x) * 4
			r, g, b, a := colour[i+2], colour[i+1], colour[i], colour[i+3]
			if !hasAlpha {
				if mask != nil {
					// The mask is white where the pixel is transparent.
					maskIndex := (y*width + x) * 4
					if mask[maskIndex] != 0 {
						continue
					}
				}
				a = 0xff
			}
			picture.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}

	var buffer bytes.Buffer
	if err := png.Encode(&buffer, picture); err != nil {
		return nil
	}
	return buffer.Bytes()
}

// readBitmapPixels reads a bitmap as BGRA bytes, top row first.
func readBitmapPixels(handle uintptr, width, height int) []byte {
	dc, _, _ := procCreateCompatibleDC.Call(0)
	if dc == 0 {
		return nil
	}
	defer procDeleteDC.Call(dc)

	header := bitmapInfoHeader{
		biSize:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		biWidth:       int32(width),
		biHeight:      int32(-height), // negative: the rows are read top down
		biPlanes:      1,
		biBitCount:    32,
		biCompression: biRGB,
	}
	pixels := make([]byte, width*height*4)

	ret, _, _ := procGetDIBits.Call(
		dc,
		handle,
		0,
		uintptr(height),
		uintptr(unsafe.Pointer(&pixels[0])),
		uintptr(unsafe.Pointer(&header)),
		dibRGBColors,
	)
	if ret == 0 {
		return nil
	}
	return pixels
}

func init() {
	SetIconProvider(&windowsIcons{cache: map[string][]byte{}})
}

// IconForFile is the icon of a file, as the shell shows it, or nil when
// there is none.
func IconForFile(path string) []byte {
	if platformIcons == nil {
		return nil
	}
	return platformIcons.FileIcon(path)
}

// ShortcutTarget reads the path a .lnk shortcut points at. Anything else is
// returned as it is.
func ShortcutTarget(path string) string {
	if !strings.EqualFold(filepath.Ext(path), ".lnk") {
		return path
	}
	return shortcutTarget(path)
}
