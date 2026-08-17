package image

import (
	"bytes"
	"image/color"
	"image/jpeg"
	"testing"

	stdimage "image"
)

func TestProcessorDecodeThumbnailAndEncode(t *testing.T) {
	original := stdimage.NewRGBA(stdimage.Rect(0, 0, 20, 10))
	for y := range 10 {
		for x := range 20 {
			original.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	var source bytes.Buffer
	if err := jpeg.Encode(&source, original, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}

	processor := NewProcessor()
	format, err := processor.ValidateMagicBytes(source.Bytes()[:min(512, source.Len())])
	if err != nil || format != "jpeg" {
		t.Fatalf("unexpected magic format: %q, %v", format, err)
	}
	decoded, decodedFormat, err := processor.Decode(bytes.NewReader(source.Bytes()))
	if err != nil || decodedFormat != "jpeg" {
		t.Fatalf("decode failed: %q, %v", decodedFormat, err)
	}
	thumbnail := processor.CreateThumbnail(decoded, 100, 100)
	if thumbnail.Bounds().Dx() != 100 || thumbnail.Bounds().Dy() != 100 {
		t.Fatalf("unexpected thumbnail size: %v", thumbnail.Bounds())
	}
	var encoded bytes.Buffer
	if err := processor.Encode(&encoded, thumbnail, "jpeg", 85); err != nil {
		t.Fatal(err)
	}
	if encoded.Len() == 0 {
		t.Fatal("encoded thumbnail is empty")
	}
}

func TestProcessorValidateWebPSignature(t *testing.T) {
	processor := NewProcessor()
	format, err := processor.ValidateMagicBytes([]byte("RIFFxxxxWEBPVP8 "))
	if err != nil || format != "webp" {
		t.Fatalf("unexpected webp detection: %q, %v", format, err)
	}
}

func TestProcessorErrorAndDefaultBranches(t *testing.T) {
	processor := NewProcessor()
	if _, _, err := processor.Decode(bytes.NewReader([]byte("not an image"))); err == nil {
		t.Fatal("invalid image decoded")
	}
	if processor.CreateThumbnail(nil, 10, 10) != nil || processor.CreateThumbnail(stdimage.NewRGBA(stdimage.Rect(0, 0, 1, 1)), 0, 10) != nil {
		t.Fatal("invalid thumbnail arguments accepted")
	}
	var output bytes.Buffer
	if err := processor.Encode(&output, nil, "jpeg", 85); err == nil {
		t.Fatal("nil image encoded")
	}
	if err := processor.Encode(&output, stdimage.NewRGBA(stdimage.Rect(0, 0, 1, 1)), "webp", 85); err == nil {
		t.Fatal("webp encoding accepted")
	}
	if err := processor.Encode(&output, stdimage.NewRGBA(stdimage.Rect(0, 0, 1, 1)), "gif", 85); err == nil {
		t.Fatal("unknown encoding accepted")
	}
	if _, err := processor.ValidateMagicBytes([]byte("not an image")); err == nil {
		t.Fatal("unknown magic bytes accepted")
	}
}
