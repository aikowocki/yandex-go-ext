package worker

import (
	"image"
	"testing"

	"github.com/aikowocki/yandex-go-ext/internal/domain"
)

func TestApplyAvatarCropReturnsSquareSelection(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1200, 800))
	cropped := applyAvatarCrop(img, domain.AvatarCrop{X: 200, Y: 0, Size: 800})
	if cropped.Bounds().Dx() != 800 || cropped.Bounds().Dy() != 800 {
		t.Fatalf("unexpected cropped bounds: %v", cropped.Bounds())
	}
}

func TestCropProcessorVersionDiffersBySelection(t *testing.T) {
	left := cropProcessorVersion(domain.AvatarCrop{X: 0, Y: 0, Size: 800})
	right := cropProcessorVersion(domain.AvatarCrop{X: 200, Y: 0, Size: 800})
	if left == right {
		t.Fatalf("expected different crop selections to produce different versions: %q", left)
	}
}
