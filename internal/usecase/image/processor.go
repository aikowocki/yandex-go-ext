package image

import (
	"errors"
	"fmt"
	stdimage "image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"strings"

	"github.com/disintegration/imaging"
	_ "golang.org/x/image/webp"
)

// ErrUnsupportedFormat означает неподдерживаемый формат изображения.
var ErrUnsupportedFormat = errors.New("unsupported image format")

// Processor декодирует, изменяет и кодирует изображения.
type Processor struct{}

// NewProcessor создаёт обработчик изображений.
func NewProcessor() *Processor {
	return &Processor{}
}

// Decode читает изображение и определяет его формат.
func (p *Processor) Decode(r io.Reader) (stdimage.Image, string, error) {
	img, format, err := stdimage.Decode(r)
	if err != nil {
		return nil, "", fmt.Errorf("decode image: %w", err)
	}
	return img, strings.ToLower(format), nil
}

// CreateThumbnail создаёт миниатюру заданного размера.
func (p *Processor) CreateThumbnail(img stdimage.Image, width, height int) stdimage.Image {
	if img == nil || width <= 0 || height <= 0 {
		return nil
	}
	return imaging.Fill(img, width, height, imaging.Center, imaging.Lanczos)
}

// Encode сохраняет изображение в нужном формате.
func (p *Processor) Encode(w io.Writer, img stdimage.Image, format string, quality int) error {
	if img == nil {
		return fmt.Errorf("%w: nil image", ErrUnsupportedFormat)
	}
	if quality <= 0 || quality > 100 {
		quality = 85
	}

	switch strings.ToLower(format) {
	case "jpg", "jpeg":
		return jpeg.Encode(w, img, &jpeg.Options{Quality: quality})
	case "png":
		return png.Encode(w, img)
	case "webp":
		// x/image/webp намеренно поддерживает только декодирование. Вызывающий код
		// должен преобразовывать миниатюры в JPEG, если исходное изображение — WebP.
		return fmt.Errorf("%w: webp encoding is not available", ErrUnsupportedFormat)
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedFormat, format)
	}
}

// ValidateMagicBytes проверяет формат по заголовку файла.
func (p *Processor) ValidateMagicBytes(data []byte) (string, error) {
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "webp", nil
	}
	contentType := http.DetectContentType(data)
	switch contentType {
	case "image/jpeg":
		return "jpeg", nil
	case "image/png":
		return "png", nil
	case "image/gif":
		return "gif", nil
	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupportedFormat, contentType)
	}
}
