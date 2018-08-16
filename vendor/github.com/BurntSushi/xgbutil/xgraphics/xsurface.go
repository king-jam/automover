package xgraphics

import (
	"fmt"
	"image"

	"github.com/BurntSushi/xgb/xproto"

	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/ewmh"
	"github.com/BurntSushi/xgbutil/icccm"
	"github.com/BurntSushi/xgbutil/keybind"
	"github.com/BurntSushi/xgbutil/mousebind"
	"github.com/BurntSushi/xgbutil/xevent"
	"github.com/BurntSushi/xgbutil/xwindow"
)

/*
xgraphics/xsurface.go contains methods for the Image type that perform X
related requests. Namely, methods that send image data start with 'X'.
*/

// XSurfaceSet will set the given window's background to this image's pixmap.
// Note that an image can have multiple surfaces, which is why the window
// id still needs to be passed to XPaint. A call to XSurfaceSet simply tells
// X that the window specified should use the pixmap in Image as its
// background image.
// Note that XSurfaceSet cannot be called on a sub-image. (An error will be
// returned if you do.)
// XSurfaceSet will also allocate an X pixmap if one hasn't been created for
// this image yet.
// (Generating a pixmap id can cause an error, so this call could return
// an error.)
func (im *Image) XSurfaceSet(wid xproto.Window) error {
	if im.Subimg {
		return fmt.Errorf("XSurfaceSet cannot be called on sub-images." +
			"Please set the surface using the original parent image.")
	}
	if im.Pixmap == 0 {
		if err := im.CreatePixmap(); err != nil {
			return err
		}
	}

	// Tell the surface (window) to use this pixmap.
	xproto.ChangeWindowAttributes(im.X.Conn(), wid,
		xproto.CwBackPixmap, []uint32{uint32(im.Pixmap)})
	return nil
}

// CreatePixmap allocates an X resource identifier for a pixmap. (It does not
// do any drawing.) You only need to call this if you're using XDraw/XExpPaint.
// If you're using XSurfaceSet/XDraw/XPaint, then CreatePixmap is called for
// you automatically.
func (im *Image) CreatePixmap() error {
	// Generate the pixmap id.
	pid, err := xproto.NewPixmapId(im.X.Conn())
	if err != nil {
		return err
	}

	// Now actually create the pixmap.
	err = xproto.CreatePixmapChecked(im.X.Conn(), im.X.Screen().RootDepth,
		pid, xproto.Drawable(im.X.RootWin()),
		uint16(im.Bounds().Dx()), uint16(im.Bounds().Dy())).Check()
	if err != nil {
		return err
	}

	// Now give it to the image.
	im.Pixmap = pid
	return nil
}

// XPaint will write the contents of the pixmap to a window.
// Note that painting will do nothing if XDraw hasn't been called.
// XPaint is what switches the buffer (drawn to using XDraw) into the window
// to be visible. That is, multiple calls to XDraw can be made, and the screen
// will only be updated once with a call to XPaint.
func (im *Image) XPaint(wid xproto.Window) {
	// We clear the whole window here because sometimes we rely on the tiling
	// of a background pixmap. If anyone knows if this is a significant
	// performance problem, please let me know. (It seems like the whole area
	// of the window is cleared when it is resized anyway.)
	xproto.ClearArea(im.X.Conn(), false, wid, 0, 0, 0, 0)
}

// XExpPaint achieves a similar result as XPaint and XSurfaceSet, but
// uses CopyArea instead of setting a background pixmap and using ClearArea.
// CreatePixmap must be called before using XExpPaint.
// XExpPaint can be called on sub-images.
// x and y correspond to the destination x and y to copy the image to.
//
// This should not be used on the same image with XSurfaceSet and XPaint.
func (im *Image) XExpPaint(wid xproto.Window, x, y int) {
	if im.Pixmap == 0 {
		return
	}
	xproto.CopyArea(im.X.Conn(),
		xproto.Drawable(im.Pixmap), xproto.Drawable(wid), im.X.GC(),
		int16(im.Rect.Min.X), int16(im.Rect.Min.Y),
		int16(x), int16(y),
		uint16(im.Rect.Dx()), uint16(im.Rect.Dy()))
}

// XPaintRects is a convenience function for issuing XDraw requests on
// each sub-image generated by the rects in the slice provided, and then
// painting the updated pixmap all at once to the window provided.
// This is efficient because no pixels are copied when taking a SubImage,
// and each XDraw call on a sub-image updates the pixels represented by that
// sub-image and only that sub-image.
func (im *Image) XPaintRects(wid xproto.Window, rects ...image.Rectangle) {
	for _, rect := range rects {
		if si := im.SubImage(rect).(*Image); si != nil {
			si.XDraw()
		}
	}
	im.XPaint(wid)
}

// XDraw will write the contents of Image to a pixmap.
// Note that this is more like a buffer. Drawing does not put the contents
// on the screen.
// After drawing, it is necessary to call XPaint to put the contents somewhere.
// Draw may return an X error if something has gone horribly wrong.
//
// XSurfaceSet should be called before XDraw. (If not, X will yell at you.)
// More specifically, CreatePixmap needs to be called before XDraw, but it is
// done automatically in XSurfaceSet.
//
// If you're using sub-images to update a particular region of the image, XDraw
// is where you'll see the performance benefit (not XPaint).
func (im *Image) XDraw() {
	im.xdraw(false)
}

// XDrawChecked is the same as XDraw, but issues PutImageChecked requests
// instead. This should *only* be used for debugging purposes, as each
// PutImageChecked request blocks for a round trip to the X server.
func (im *Image) XDrawChecked() error {
	return im.xdraw(true)
}

func (im *Image) xdraw(checked bool) error {
	width, height := im.Rect.Dx(), im.Rect.Dy()

	// Put the raw image data into its own slice.
	// If this isn't a sub-image, then skip because it isn't necessary.
	var data []uint8
	if !im.Subimg {
		data = im.Pix
	} else {
		data = make([]uint8, width*height*4)
		for y := im.Rect.Min.Y; y < im.Rect.Max.Y; y++ {
			i := (y - im.Rect.Min.Y) * width * 4
			copy(data[i:i+4*width], im.Pix[im.PixOffset(im.Rect.Min.X, y):])
		}
	}

	// X's max request size (by default) is (2^16) * 4 = 262144 bytes, which
	// corresponds precisely to a 256x256 sized image with 32 bits per pixel.
	// Thus, we check the size of the image data and calculate the number of
	// rows of the image we'll send in each request. If a single row of an
	// image exceeds the max request length, we're in trouble.  N.B. The
	// constant 28 comes from the fixed size part of a PutImage request.
	rowsPer := (xgbutil.MaxReqSize - 28) / (width * 4)
	bytesPer := rowsPer * width * 4

	// The start x position of what we're sending. Doesn't change.
	xpos := im.Rect.Min.X

	// The start y position of what we're sending. Increases based on the
	// number of rows of the image we send in each request.
	ypos := im.Rect.Min.Y

	// The height of each PutImage request. It's always rowsPer, unless its
	// the last request and we're not sending the maximum number of bytes.
	heightPer := 0

	// The start and end positions of the raw bytes being sent.
	start, end := 0, 0

	// The sliced data we're sending, for convenience.
	var toSend []byte

	for end < len(data) {
		end = start + bytesPer
		if end > len(data) { // make sure end doesn't extend beyond data
			end = len(data)
		}

		toSend = data[start:end]
		heightPer = len(toSend) / 4 / width

		if checked {
			err := xproto.PutImageChecked(
				im.X.Conn(), xproto.ImageFormatZPixmap,
				xproto.Drawable(im.Pixmap), im.X.GC(),
				uint16(width), uint16(heightPer), int16(xpos), int16(ypos),
				0, 24, toSend).Check()
			if err != nil {
				return err
			}
		} else {
			xproto.PutImage(im.X.Conn(), xproto.ImageFormatZPixmap,
				xproto.Drawable(im.Pixmap), im.X.GC(),
				uint16(width), uint16(heightPer), int16(xpos), int16(ypos),
				0, 24, toSend)
		}
		start = end
		ypos += rowsPer
	}

	return nil
}

// XShow creates a new window and paints the image to the window.
// This is useful for debugging, or if you're creating an image viewer.
// XShow also returns the xwindow.Window value, in case you want to do
// further processing. (Like attach event handlers.)
func (im *Image) XShow() *xwindow.Window {
	return im.XShowExtra("", false)
}

// XShowName is just like XShow, except it sets the name of the window to the
// name provided, and will quit the current event loop if 'quit' is true when
// the window is closed.
// If name is empty and quit is false, then the behavior is precisely the same
// as XShow.
func (im *Image) XShowExtra(name string, quit bool) *xwindow.Window {
	if len(name) == 0 {
		name = "xgbutil Image Window"
	}
	w, h := im.Rect.Dx(), im.Rect.Dy()

	win, err := xwindow.Generate(im.X)
	if err != nil {
		xgbutil.Logger.Printf("Could not generate new window id: %s", err)
		return nil
	}

	// Create a very simple window with dimensions equal to the image.
	win.Create(im.X.RootWin(), 0, 0, w, h, 0)

	// Make this window close gracefully.
	win.WMGracefulClose(func(w *xwindow.Window) {
		xevent.Detach(w.X, w.Id)
		keybind.Detach(w.X, w.Id)
		mousebind.Detach(w.X, w.Id)
		w.Destroy()

		if quit {
			xevent.Quit(w.X)
		}
	})

	// Set WM_STATE so it is interpreted as a top-level window.
	err = icccm.WmStateSet(im.X, win.Id, &icccm.WmState{
		State: icccm.StateNormal,
	})
	if err != nil { // not a fatal error
		xgbutil.Logger.Printf("Could not set WM_STATE: %s", err)
	}

	// Set WM_NORMAL_HINTS so the window can't be resized.
	err = icccm.WmNormalHintsSet(im.X, win.Id, &icccm.NormalHints{
		Flags:     icccm.SizeHintPMinSize | icccm.SizeHintPMaxSize,
		MinWidth:  uint(w),
		MinHeight: uint(h),
		MaxWidth:  uint(w),
		MaxHeight: uint(h),
	})
	if err != nil { // not a fatal error
		xgbutil.Logger.Printf("Could not set WM_NORMAL_HINTS: %s", err)
	}

	// Set _NET_WM_NAME so it looks nice.
	err = ewmh.WmNameSet(im.X, win.Id, name)
	if err != nil { // not a fatal error
		xgbutil.Logger.Printf("Could not set _NET_WM_NAME: %s", err)
	}

	// Paint our image before mapping.
	im.XSurfaceSet(win.Id)
	im.XDraw()
	im.XPaint(win.Id)

	// Now we can map, since we've set all our properties.
	// (The initial map is when the window manager starts managing.)
	win.Map()

	return win
}
