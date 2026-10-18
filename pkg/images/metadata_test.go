package images

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"testing"

	"github.com/HugoSmits86/nativewebp"
)

func TestJPEGOrientationBeforeMetadataRemoval(t *testing.T) {
	jpeg := encodedImage(t, "jpeg", 100, 200)
	exif := append([]byte("Exif\x00\x00"), []byte{
		'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0,
		0x12, 1, 3, 0, 1, 0, 0, 0, 6, 0, 0, 0,
		0, 0, 0, 0,
	}...)
	data := append([]byte{0xff, 0xd8, 0xff, 0xe1, 0, byte(len(exif) + 2)}, exif...)
	data = append(data, jpeg[2:]...)
	result, err := Process(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(result.Data))
	if err != nil || config.Width != 200 || config.Height != 100 {
		t.Fatalf("orientation not preserved: %+v %v", config, err)
	}
	if bytes.Contains(result.Data, []byte("Exif")) {
		t.Fatal("EXIF metadata retained")
	}
}

func TestAnimatedImagesRejected(t *testing.T) {
	var webp bytes.Buffer
	animation := &nativewebp.Animation{
		Images: []image.Image{
			image.NewNRGBA(image.Rect(0, 0, 100, 100)), image.NewNRGBA(image.Rect(0, 0, 100, 100)),
		},
		Durations: []uint{100, 100}, Disposals: []uint{0, 0},
	}
	if err := nativewebp.EncodeAll(&webp, animation, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := Process(bytes.NewReader(webp.Bytes())); err != ErrInvalid {
		t.Fatalf("animated WebP accepted: %v", err)
	}
	png := encodedImage(t, "png", 100, 100)
	chunk := make([]byte, 20)
	binary.BigEndian.PutUint32(chunk[:4], 8)
	copy(chunk[4:8], "acTL")
	binary.BigEndian.PutUint32(chunk[8:12], 2)
	binary.BigEndian.PutUint32(chunk[16:], crc32.ChecksumIEEE(chunk[4:16]))
	apng := append(append(append([]byte{}, png[:33]...), chunk...), png[33:]...)
	if _, err := Process(bytes.NewReader(apng)); err != ErrInvalid {
		t.Fatalf("animated PNG accepted: %v", err)
	}
}
