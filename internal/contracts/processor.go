package contracts

import (
	"image"
	"io"
)

// ImageProcessor декодирует изображения и создаёт миниатюры.
type ImageProcessor interface {
	Decode(r io.Reader) (image.Image, string, error)
	CreateThumbnail(img image.Image, width, height int) image.Image
	Encode(w io.Writer, img image.Image, format string, quality int) error
	ValidateMagicBytes(data []byte) (string, error)
}
