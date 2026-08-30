package utils
import (
	 "bytes"
)
var (
 JpegMagic = []byte{0xFF, 0xD8, 0xFF}
 PngMagic  = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
 WebpMagic = []byte{0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50}
)

func DetectFileType(buf []byte) (string, error) {
 switch {
 case bytes.HasPrefix(buf, JpegMagic):
  return ".jpeg", nil
 case bytes.HasPrefix(buf, PngMagic):
  return ".png", nil
 case len(buf) >= 12 && bytes.Equal(buf[0:4], WebpMagic[0:4]) && bytes.Equal(buf[8:12], WebpMagic[8:12]):
  return ".webp", nil
 default:
  return "null", nil
 }
}