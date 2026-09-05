package web

import (
	"encoding/binary"
	"testing"
)

func TestMediaContainerMetadataRejectsMalformedWebPAndAVIF(t *testing.T) {
	webp := make([]byte, 30)
	copy(webp[:4], "RIFF")
	binary.LittleEndian.PutUint32(webp[4:8], uint32(len(webp)-8))
	copy(webp[8:12], "WEBP")
	copy(webp[12:16], "VP8X")
	binary.LittleEndian.PutUint32(webp[16:20], 10)
	webp[24], webp[27] = 1, 1
	if _, _, ok := webPSize(webp); ok {
		t.Fatal("header-only WebP was accepted")
	}
	minimalVP8 := make([]byte, 30)
	copy(minimalVP8[:4], "RIFF")
	binary.LittleEndian.PutUint32(minimalVP8[4:8], uint32(len(minimalVP8)-8))
	copy(minimalVP8[8:12], "WEBP")
	copy(minimalVP8[12:16], "VP8 ")
	binary.LittleEndian.PutUint32(minimalVP8[16:20], 10)
	copy(minimalVP8[23:26], "\x9d\x01\x2a")
	minimalVP8[26], minimalVP8[28] = 1, 1
	if _, _, ok := webPSize(minimalVP8); ok {
		t.Fatal("WebP frame header without compressed payload was accepted")
	}

	avif := make([]byte, 32)
	binary.BigEndian.PutUint32(avif[:4], 24)
	copy(avif[4:8], "ftyp")
	copy(avif[8:12], "avif")
	copy(avif[16:20], "avif")
	binary.BigEndian.PutUint32(avif[24:28], 8)
	copy(avif[28:32], "ispe")
	if _, _, ok := avifSize(avif); ok {
		t.Fatal("incomplete AVIF box sequence was accepted")
	}
}
