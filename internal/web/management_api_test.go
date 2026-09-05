package web

import (
	"encoding/base64"
	"encoding/binary"
	"testing"
)

func TestWebPSizeDecodesImagePixels(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString("UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA")
	if err != nil {
		t.Fatal(err)
	}
	width, height, ok := webPSize(data)
	if !ok || width != 1 || height != 1 {
		t.Fatalf("webPSize() = (%d, %d, %t), want (1, 1, true)", width, height, ok)
	}
}

func TestWebPSizeRejectsFabricatedAnimationFrame(t *testing.T) {
	webp := make([]byte, 56)
	copy(webp[:4], "RIFF")
	binary.LittleEndian.PutUint32(webp[4:8], uint32(len(webp)-8))
	copy(webp[8:12], "WEBP")
	copy(webp[12:16], "VP8X")
	binary.LittleEndian.PutUint32(webp[16:20], 10)
	webp[24] = 1
	copy(webp[30:34], "ANMF")
	binary.LittleEndian.PutUint32(webp[34:38], 17)
	for index := 38; index < 55; index++ {
		webp[index] = 0xff
	}

	if _, _, ok := webPSize(webp); ok {
		t.Fatal("fabricated WebP animation frame was accepted")
	}
}

func TestWebPSizeRejectsMalformedContainers(t *testing.T) {
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
}

func TestAVIFSizeRequiresPrimaryAV1ItemPropertiesAndExtent(t *testing.T) {
	tests := []struct {
		name    string
		options avifFixtureOptions
	}{
		{
			name:    "unknown primary item",
			options: avifFixtureOptions{primaryItemID: 2, itemType: "av01", associations: []byte{1, 2}, extentLength: 1},
		},
		{
			name:    "non AV1 primary item",
			options: avifFixtureOptions{primaryItemID: 1, itemType: "mime", associations: []byte{1, 2}, extentLength: 1},
		},
		{
			name:    "missing AV1 codec property association",
			options: avifFixtureOptions{primaryItemID: 1, itemType: "av01", associations: []byte{1}, extentLength: 1},
		},
		{
			name:    "missing spatial extents property association",
			options: avifFixtureOptions{primaryItemID: 1, itemType: "av01", associations: []byte{2}, extentLength: 1},
		},
		{
			name:    "extent outside media data",
			options: avifFixtureOptions{primaryItemID: 1, itemType: "av01", associations: []byte{1, 2}, extentOffsetDelta: 2, extentLength: 1},
		},
		{
			name:    "extent belongs to another item",
			options: avifFixtureOptions{primaryItemID: 1, itemType: "av01", associations: []byte{1, 2}, locationItemID: 2, extentLength: 1},
		},
		{
			name:    "empty extent",
			options: avifFixtureOptions{primaryItemID: 1, itemType: "av01", associations: []byte{1, 2}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, _, ok := avifSize(makeAVIFFixture(test.options)); ok {
				t.Fatal("invalid AVIF item linkage was accepted")
			}
		})
	}
}

func TestAVIFSizeAcceptsLinkedMdatAndIdatExtents(t *testing.T) {
	for _, constructionMethod := range []uint16{0, 1} {
		name := "mdat"
		if constructionMethod == 1 {
			name = "idat"
		}
		t.Run(name, func(t *testing.T) {
			data := makeAVIFFixture(avifFixtureOptions{
				primaryItemID:      1,
				itemType:           "av01",
				associations:       []byte{1, 2},
				constructionMethod: constructionMethod,
				extentLength:       1,
			})
			width, height, ok := avifSize(data)
			if !ok || width != 2 || height != 3 {
				t.Fatalf("avifSize() = (%d, %d, %t), want (2, 3, true)", width, height, ok)
			}
		})
	}
}

func TestAVIFSizeRejectsIncompleteBoxSequence(t *testing.T) {
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

type avifFixtureOptions struct {
	primaryItemID      uint16
	itemType           string
	associations       []byte
	constructionMethod uint16
	locationItemID     uint16
	extentOffsetDelta  int
	extentLength       uint32
}

func makeAVIFFixture(options avifFixtureOptions) []byte {
	ftyp := testBox("ftyp", []byte("avif\x00\x00\x00\x00avifmif1"))

	makeMeta := func(extentOffset uint32) []byte {
		hdlr := testFullBox("hdlr", 0, append([]byte("\x00\x00\x00\x00pict"), make([]byte, 12)...))
		pitmPayload := make([]byte, 2)
		binary.BigEndian.PutUint16(pitmPayload, options.primaryItemID)
		pitm := testFullBox("pitm", 0, pitmPayload)

		itemType := options.itemType
		if len(itemType) != 4 {
			itemType = "av01"
		}
		infePayload := make([]byte, 8)
		binary.BigEndian.PutUint16(infePayload[:2], 1)
		copy(infePayload[4:8], itemType)
		infe := testFullBox("infe", 2, append(infePayload, 0))
		iinfPayload := make([]byte, 2)
		binary.BigEndian.PutUint16(iinfPayload, 1)
		iinf := testFullBox("iinf", 0, append(iinfPayload, infe...))

		ispePayload := make([]byte, 8)
		binary.BigEndian.PutUint32(ispePayload[:4], 2)
		binary.BigEndian.PutUint32(ispePayload[4:], 3)
		ispe := testFullBox("ispe", 0, ispePayload)
		av1C := testBox("av1C", []byte{0x81, 0, 0, 0})
		ipco := testBox("ipco", append(ispe, av1C...))
		ipmaPayload := make([]byte, 7+len(options.associations))
		binary.BigEndian.PutUint32(ipmaPayload[:4], 1)
		binary.BigEndian.PutUint16(ipmaPayload[4:6], 1)
		ipmaPayload[6] = byte(len(options.associations))
		copy(ipmaPayload[7:], options.associations)
		ipma := testFullBox("ipma", 0, ipmaPayload)
		iprp := testBox("iprp", append(ipco, ipma...))

		ilocPayload := make([]byte, 20)
		ilocPayload[0] = 0x44
		binary.BigEndian.PutUint16(ilocPayload[2:4], 1)
		locationItemID := options.locationItemID
		if locationItemID == 0 {
			locationItemID = 1
		}
		binary.BigEndian.PutUint16(ilocPayload[4:6], locationItemID)
		binary.BigEndian.PutUint16(ilocPayload[6:8], options.constructionMethod)
		binary.BigEndian.PutUint16(ilocPayload[10:12], 1)
		binary.BigEndian.PutUint32(ilocPayload[12:16], extentOffset)
		binary.BigEndian.PutUint32(ilocPayload[16:20], options.extentLength)
		iloc := testFullBox("iloc", 1, ilocPayload)

		children := append(append(append(append(hdlr, pitm...), iinf...), iloc...), iprp...)
		if options.constructionMethod == 1 {
			children = append(children, testBox("idat", []byte{0x12})...)
		}
		return testFullBox("meta", 0, children)
	}

	meta := makeMeta(0)
	if options.constructionMethod == 0 {
		extentOffset := uint32(len(ftyp) + len(meta) + 8 + options.extentOffsetDelta)
		meta = makeMeta(extentOffset)
		return append(append(ftyp, meta...), testBox("mdat", []byte{0x12})...)
	}
	return append(ftyp, meta...)
}

func testFullBox(kind string, version byte, payload []byte) []byte {
	return testBox(kind, append([]byte{version, 0, 0, 0}, payload...))
}

func testBox(kind string, payload []byte) []byte {
	box := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(box[:4], uint32(len(box)))
	copy(box[4:8], kind)
	copy(box[8:], payload)
	return box
}
