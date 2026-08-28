package avatar

import (
	"testing"

	"github.com/aikowocki/yandex-go-ext/internal/domain"
)

func TestNormalizeCropDefaultsToCenteredSquare(t *testing.T) {
	crop, err := normalizeCrop(domain.AvatarCrop{}, 1200, 800)
	if err != nil {
		t.Fatal(err)
	}
	if crop.X != 200 || crop.Y != 0 || crop.Size != 800 {
		t.Fatalf("unexpected crop: %+v", crop)
	}
}

func TestNormalizeCropRejectsOutOfBoundsSelection(t *testing.T) {
	_, err := normalizeCrop(domain.AvatarCrop{X: 100, Y: 100, Size: 500}, 400, 400)
	if err == nil {
		t.Fatal("expected out-of-bounds crop to fail")
	}
}
