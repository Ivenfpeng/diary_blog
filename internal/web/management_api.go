package web

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Ivenfpeng/diary_blog/internal/repository/sqlite"
	"github.com/Ivenfpeng/diary_blog/internal/site"
	"github.com/go-chi/chi/v5"
	"golang.org/x/image/webp"
)

const adminMediaBodyLimit int64 = 10 * 1024 * 1024

type adminManagementAPI struct {
	repository *sqlite.PostRepository
	mediaDir   string
	clock      func() time.Time
	cache      *site.Cache
}
type taxonomyRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}
type mediaAltRequest struct {
	AltText string `json:"alt_text"`
}

func newAdminManagementAPI(repository *sqlite.PostRepository, mediaDir string, clock func() time.Time, cache *site.Cache) *adminManagementAPI {
	return &adminManagementAPI{repository: repository, mediaDir: mediaDir, clock: clock, cache: cache}
}
func (a *adminManagementAPI) routes(router chi.Router, requireSession, requireCSRF func(http.Handler) http.Handler) {
	router.Group(func(router chi.Router) {
		router.Use(requireSession, requireCSRF)
		router.Get("/api/admin/categories", a.listCategories)
		router.Post("/api/admin/categories", a.createCategory)
		router.Put("/api/admin/categories/{id}", a.updateCategory)
		router.Delete("/api/admin/categories/{id}", a.deleteCategory)
		router.Get("/api/admin/tags", a.listTags)
		router.Post("/api/admin/tags", a.createTag)
		router.Put("/api/admin/tags/{id}", a.updateTag)
		router.Delete("/api/admin/tags/{id}", a.deleteTag)
		router.Get("/api/admin/media", a.listMedia)
		router.Post("/api/admin/media", a.uploadMedia)
		router.Patch("/api/admin/media/{id}", a.updateMediaAltText)
		router.Get("/api/admin/settings", a.getSettings)
		router.Put("/api/admin/settings", a.saveSettings)
	})
}
func (a *adminManagementAPI) listCategories(w http.ResponseWriter, r *http.Request) {
	values, err := a.repository.ListCategories(r.Context())
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": values})
}
func (a *adminManagementAPI) listTags(w http.ResponseWriter, r *http.Request) {
	values, err := a.repository.ListTags(r.Context())
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tags": values})
}
func (a *adminManagementAPI) createCategory(w http.ResponseWriter, r *http.Request) {
	var input taxonomyRequest
	if !decodeAdminJSON(w, r, &input) {
		return
	}
	item, err := a.repository.CreateCategory(r.Context(), sqlite.TaxonomyInput{Name: input.Name, Slug: input.Slug}, a.clock())
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"category": item})
}
func (a *adminManagementAPI) createTag(w http.ResponseWriter, r *http.Request) {
	var input taxonomyRequest
	if !decodeAdminJSON(w, r, &input) {
		return
	}
	item, err := a.repository.CreateTag(r.Context(), sqlite.TaxonomyInput{Name: input.Name, Slug: input.Slug}, a.clock())
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"tag": item})
}
func (a *adminManagementAPI) updateCategory(w http.ResponseWriter, r *http.Request) {
	a.updateTaxonomy(w, r, true)
}
func (a *adminManagementAPI) updateTag(w http.ResponseWriter, r *http.Request) {
	a.updateTaxonomy(w, r, false)
}
func (a *adminManagementAPI) updateTaxonomy(w http.ResponseWriter, r *http.Request, category bool) {
	id, ok := managementID(w, r)
	if !ok {
		return
	}
	var input taxonomyRequest
	if !decodeAdminJSON(w, r, &input) {
		return
	}
	var item sqlite.Taxonomy
	var err error
	if category {
		item, err = a.repository.UpdateCategory(r.Context(), id, sqlite.TaxonomyInput{Name: input.Name, Slug: input.Slug}, a.clock())
	} else {
		item, err = a.repository.UpdateTag(r.Context(), id, sqlite.TaxonomyInput{Name: input.Name, Slug: input.Slug}, a.clock())
	}
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	key := "tag"
	if category {
		key = "category"
	}
	writeJSON(w, http.StatusOK, map[string]any{key: item})
}
func (a *adminManagementAPI) deleteCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := managementID(w, r)
	if !ok {
		return
	}
	if err := a.repository.DeleteCategory(r.Context(), id); err != nil {
		a.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (a *adminManagementAPI) deleteTag(w http.ResponseWriter, r *http.Request) {
	id, ok := managementID(w, r)
	if !ok {
		return
	}
	if err := a.repository.DeleteTag(r.Context(), id); err != nil {
		a.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (a *adminManagementAPI) listMedia(w http.ResponseWriter, r *http.Request) {
	values, err := a.repository.ListMedia(r.Context())
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"media": values})
}
func (a *adminManagementAPI) updateMediaAltText(w http.ResponseWriter, r *http.Request) {
	id, ok := managementID(w, r)
	if !ok {
		return
	}
	var input mediaAltRequest
	if !decodeAdminJSON(w, r, &input) {
		return
	}
	item, err := a.repository.UpdateMediaAltText(r.Context(), id, input.AltText)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"media": item})
}
func (a *adminManagementAPI) getSettings(w http.ResponseWriter, r *http.Request) {
	value, err := a.repository.GetSettings(r.Context())
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": value})
}
func (a *adminManagementAPI) saveSettings(w http.ResponseWriter, r *http.Request) {
	var value sqlite.Settings
	if !decodeAdminJSON(w, r, &value) {
		return
	}
	saved, err := a.repository.SaveSettings(r.Context(), value, a.clock())
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	if a.cache != nil {
		a.cache.InvalidateAll()
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": saved})
}

func (a *adminManagementAPI) uploadMedia(w http.ResponseWriter, r *http.Request) {
	if a.mediaDir == "" {
		writeAPIError(w, r, http.StatusServiceUnavailable, "media_unavailable", "Media storage is not configured.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, adminMediaBodyLimit)
	if err := r.ParseMultipartForm(adminMediaBodyLimit); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "media_validation", "The uploaded image is invalid.")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "media_validation", "Choose an image to upload.")
		return
	}
	defer file.Close()
	altText := strings.TrimSpace(r.FormValue("alt_text"))
	if altText == "" {
		writeAPIError(w, r, http.StatusBadRequest, "media_validation", "Alt text is required.")
		return
	}
	now := a.clock().UTC()
	relativeDir := filepath.Join(now.Format("2006"), now.Format("01"))
	dir := filepath.Join(a.mediaDir, relativeDir)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		a.writeError(w, r, err)
		return
	}
	tmp, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	limited := io.LimitReader(file, adminMediaBodyLimit+1)
	size, err := io.Copy(tmp, limited)
	if err != nil || size > adminMediaBodyLimit {
		_ = tmp.Close()
		writeAPIError(w, r, http.StatusBadRequest, "media_validation", "Images must be 10 MiB or smaller.")
		return
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		a.writeError(w, r, err)
		return
	}
	if err := tmp.Close(); err != nil {
		a.writeError(w, r, err)
		return
	}
	mimeType, width, height, err := imageMetadata(tmpName, header.Header.Get("Content-Type"))
	if err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "media_validation", "The uploaded image is invalid.")
		return
	}
	if int64(width)*int64(height) > adminMediaBodyLimit/4 {
		writeAPIError(w, r, http.StatusBadRequest, "media_validation", "Decoded images must be 10 MiB or smaller.")
		return
	}
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		a.writeError(w, r, err)
		return
	}
	extension := mediaExtension(mimeType)
	if extension == "" {
		writeAPIError(w, r, http.StatusBadRequest, "media_validation", "The uploaded image format is not supported.")
		return
	}
	name := hex.EncodeToString(bytes) + extension
	relative := filepath.ToSlash(filepath.Join(relativeDir, name))
	destination := filepath.Join(dir, name)
	if err := os.Rename(tmpName, destination); err != nil {
		a.writeError(w, r, err)
		return
	}
	item, err := a.repository.CreateMedia(r.Context(), sqlite.MediaInput{Path: relative, MIMEType: mimeType, Width: width, Height: height, Size: size, AltText: altText}, now)
	if err != nil {
		_ = os.Remove(destination)
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"media": item})
}
func managementID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		writeAPIError(w, r, http.StatusBadRequest, "management_validation", "The management resource is invalid.")
		return 0, false
	}
	return id, true
}
func (a *adminManagementAPI) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, sqlite.ErrValidation):
		writeAPIError(w, r, http.StatusBadRequest, "management_validation", "The management data is invalid.")
	case errors.Is(err, sqlite.ErrNotFound):
		writeAPIError(w, r, http.StatusNotFound, "management_not_found", "The management resource was not found.")
	case errors.Is(err, sqlite.ErrReferenced):
		writeAPIError(w, r, http.StatusConflict, "management_referenced", "This category is still used by an article.")
	default:
		writeAPIError(w, r, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
	}
}

func imageMetadata(path, supplied string) (string, int, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, 0, err
	}
	defer file.Close()
	config, format, err := image.DecodeConfig(file)
	if err == nil {
		mime := "image/" + format
		if format == "jpeg" {
			mime = "image/jpeg"
		}
		if !sqliteAllowedMediaType(mime) {
			return "", 0, 0, fmt.Errorf("unsupported")
		}
		return mime, config.Width, config.Height, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0, 0, err
	}
	if width, height, ok := webPSize(data); ok {
		return "image/webp", width, height, nil
	}
	if width, height, ok := avifSize(data); ok {
		return "image/avif", width, height, nil
	}
	return "", 0, 0, fmt.Errorf("decode %s", supplied)
}
func sqliteAllowedMediaType(mime string) bool {
	return mime == "image/jpeg" || mime == "image/png" || mime == "image/gif" || mime == "image/webp" || mime == "image/avif"
}
func mediaExtension(mime string) string {
	switch mime {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/avif":
		return ".avif"
	}
	return ""
}
func webPSize(b []byte) (int, int, bool) {
	if len(b) < 20 || string(b[:4]) != "RIFF" || string(b[8:12]) != "WEBP" || uint64(binary.LittleEndian.Uint32(b[4:8]))+8 != uint64(len(b)) {
		return 0, 0, false
	}
	config, err := webp.DecodeConfig(bytes.NewReader(b))
	if err != nil || config.Width < 1 || config.Height < 1 || int64(config.Width)*int64(config.Height) > adminMediaBodyLimit/4 {
		return 0, 0, false
	}
	decoded, err := webp.Decode(bytes.NewReader(b))
	if err != nil {
		return 0, 0, false
	}
	width, height := decoded.Bounds().Dx(), decoded.Bounds().Dy()
	return width, height, width == config.Width && height == config.Height
}

func avifSize(b []byte) (int, int, bool) {
	topLevel, ok := avifBoxes(b, 0, len(b))
	if !ok || len(topLevel) == 0 || topLevel[0].kind != "ftyp" || !avifBrand(b[topLevel[0].payloadStart:topLevel[0].end]) {
		return 0, 0, false
	}

	var meta avifBox
	var mediaData []avifByteRange
	for _, box := range topLevel {
		switch box.kind {
		case "meta":
			if meta.kind != "" {
				return 0, 0, false
			}
			meta = box
		case "mdat":
			mediaData = append(mediaData, avifByteRange{start: uint64(box.payloadStart), end: uint64(box.end)})
		}
	}
	if meta.kind == "" {
		return 0, 0, false
	}
	metaVersion, metaFlags, metaContent, ok := avifFullBox(b, meta)
	if !ok || metaVersion != 0 || metaFlags != 0 {
		return 0, 0, false
	}
	children, ok := avifBoxes(b, metaContent, meta.end)
	if !ok {
		return 0, 0, false
	}

	var handlerOK bool
	var primaryItemID uint32
	var havePrimaryItem bool
	var itemTypes map[uint32]string
	var locationBox avifBox
	var properties map[uint32]avifProperty
	var associations map[uint32]map[uint32]bool
	var itemData []avifByteRange
	for _, box := range children {
		switch box.kind {
		case "hdlr":
			if handlerOK || !avifPictureHandler(b, box) {
				return 0, 0, false
			}
			handlerOK = true
		case "pitm":
			if havePrimaryItem {
				return 0, 0, false
			}
			primaryItemID, ok = avifPrimaryItem(b, box)
			if !ok {
				return 0, 0, false
			}
			havePrimaryItem = true
		case "iinf":
			if itemTypes != nil {
				return 0, 0, false
			}
			itemTypes, ok = avifItemTypes(b, box)
			if !ok {
				return 0, 0, false
			}
		case "iloc":
			if locationBox.kind != "" {
				return 0, 0, false
			}
			locationBox = box
		case "iprp":
			if properties != nil {
				return 0, 0, false
			}
			properties, associations, ok = avifItemProperties(b, box)
			if !ok {
				return 0, 0, false
			}
		case "idat":
			itemData = append(itemData, avifByteRange{start: uint64(box.payloadStart), end: uint64(box.end)})
		}
	}
	if !handlerOK || !havePrimaryItem || itemTypes[primaryItemID] != "av01" || locationBox.kind == "" || properties == nil {
		return 0, 0, false
	}

	var width, height int
	var haveSpatialExtents, haveAV1Config bool
	for propertyIndex := range associations[primaryItemID] {
		property, exists := properties[propertyIndex]
		if !exists {
			return 0, 0, false
		}
		switch property.kind {
		case "ispe":
			if !haveSpatialExtents {
				width, height = property.width, property.height
			}
			haveSpatialExtents = true
		case "av1C":
			haveAV1Config = true
		}
	}
	if !haveSpatialExtents || !haveAV1Config || !avifPrimaryItemHasData(b, locationBox, primaryItemID, mediaData, itemData) {
		return 0, 0, false
	}
	return width, height, true
}

func avifBrand(ftyp []byte) bool {
	if len(ftyp) < 8 {
		return false
	}
	if brand := string(ftyp[:4]); brand == "avif" || brand == "avis" {
		return true
	}
	for offset := 8; offset+4 <= len(ftyp); offset += 4 {
		if brand := string(ftyp[offset : offset+4]); brand == "avif" || brand == "avis" {
			return true
		}
	}
	return false
}

type avifBox struct {
	kind         string
	start        int
	payloadStart int
	end          int
}

type avifByteRange struct {
	start uint64
	end   uint64
}

type avifProperty struct {
	kind   string
	width  int
	height int
}

func avifBoxes(b []byte, start, end int) ([]avifBox, bool) {
	if start < 0 || end < start || end > len(b) {
		return nil, false
	}
	var boxes []avifBox
	for offset := start; offset < end; {
		if end-offset < 8 {
			return nil, false
		}
		size := uint64(binary.BigEndian.Uint32(b[offset : offset+4]))
		headerSize := 8
		if size == 1 {
			if end-offset < 16 {
				return nil, false
			}
			size = binary.BigEndian.Uint64(b[offset+8 : offset+16])
			headerSize = 16
		} else if size == 0 {
			size = uint64(end - offset)
		}
		if size < uint64(headerSize) || size > uint64(end-offset) {
			return nil, false
		}
		boxEnd := offset + int(size)
		boxes = append(boxes, avifBox{
			kind:         string(b[offset+4 : offset+8]),
			start:        offset,
			payloadStart: offset + headerSize,
			end:          boxEnd,
		})
		offset = boxEnd
	}
	return boxes, true
}

func avifFullBox(b []byte, box avifBox) (byte, uint32, int, bool) {
	if box.payloadStart < 0 || box.end-box.payloadStart < 4 || box.end > len(b) {
		return 0, 0, 0, false
	}
	flags := uint32(b[box.payloadStart+1])<<16 | uint32(b[box.payloadStart+2])<<8 | uint32(b[box.payloadStart+3])
	return b[box.payloadStart], flags, box.payloadStart + 4, true
}

func avifPictureHandler(b []byte, box avifBox) bool {
	version, flags, content, ok := avifFullBox(b, box)
	return ok && version == 0 && flags == 0 && box.end-content >= 8 && string(b[content+4:content+8]) == "pict"
}

func avifPrimaryItem(b []byte, box avifBox) (uint32, bool) {
	version, flags, content, ok := avifFullBox(b, box)
	if !ok || flags != 0 {
		return 0, false
	}
	switch version {
	case 0:
		if box.end-content < 2 {
			return 0, false
		}
		return uint32(binary.BigEndian.Uint16(b[content : content+2])), true
	case 1:
		if box.end-content < 4 {
			return 0, false
		}
		return binary.BigEndian.Uint32(b[content : content+4]), true
	default:
		return 0, false
	}
}

func avifItemTypes(b []byte, box avifBox) (map[uint32]string, bool) {
	version, flags, content, ok := avifFullBox(b, box)
	if !ok || flags != 0 {
		return nil, false
	}
	var entryCount uint32
	switch version {
	case 0:
		if box.end-content < 2 {
			return nil, false
		}
		entryCount = uint32(binary.BigEndian.Uint16(b[content : content+2]))
		content += 2
	case 1:
		if box.end-content < 4 {
			return nil, false
		}
		entryCount = binary.BigEndian.Uint32(b[content : content+4])
		content += 4
	default:
		return nil, false
	}
	entries, ok := avifBoxes(b, content, box.end)
	if !ok || uint64(entryCount) != uint64(len(entries)) {
		return nil, false
	}
	items := make(map[uint32]string, len(entries))
	for _, entry := range entries {
		if entry.kind != "infe" {
			return nil, false
		}
		itemID, itemType, ok := avifItemInfoEntry(b, entry)
		if !ok {
			return nil, false
		}
		if _, duplicate := items[itemID]; duplicate {
			return nil, false
		}
		items[itemID] = itemType
	}
	return items, true
}

func avifItemInfoEntry(b []byte, box avifBox) (uint32, string, bool) {
	version, flags, content, ok := avifFullBox(b, box)
	if !ok || flags != 0 {
		return 0, "", false
	}
	switch version {
	case 2:
		if box.end-content < 8 {
			return 0, "", false
		}
		return uint32(binary.BigEndian.Uint16(b[content : content+2])), string(b[content+4 : content+8]), true
	case 3:
		if box.end-content < 10 {
			return 0, "", false
		}
		return binary.BigEndian.Uint32(b[content : content+4]), string(b[content+6 : content+10]), true
	default:
		return 0, "", false
	}
}

func avifItemProperties(b []byte, box avifBox) (map[uint32]avifProperty, map[uint32]map[uint32]bool, bool) {
	children, ok := avifBoxes(b, box.payloadStart, box.end)
	if !ok {
		return nil, nil, false
	}
	var properties map[uint32]avifProperty
	associations := make(map[uint32]map[uint32]bool)
	for _, child := range children {
		switch child.kind {
		case "ipco":
			if properties != nil {
				return nil, nil, false
			}
			properties, ok = avifPropertyContainer(b, child)
		case "ipma":
			ok = avifPropertyAssociations(b, child, associations)
		default:
			ok = true
		}
		if !ok {
			return nil, nil, false
		}
	}
	if properties == nil || len(associations) == 0 {
		return nil, nil, false
	}
	for _, indexes := range associations {
		for index := range indexes {
			if index == 0 || properties[index].kind == "" {
				return nil, nil, false
			}
		}
	}
	return properties, associations, true
}

func avifPropertyContainer(b []byte, box avifBox) (map[uint32]avifProperty, bool) {
	children, ok := avifBoxes(b, box.payloadStart, box.end)
	if !ok || len(children) == 0 {
		return nil, false
	}
	properties := make(map[uint32]avifProperty, len(children))
	for index, child := range children {
		property := avifProperty{kind: child.kind}
		switch child.kind {
		case "ispe":
			version, flags, content, ok := avifFullBox(b, child)
			if !ok || version != 0 || flags != 0 || child.end-content < 8 {
				return nil, false
			}
			property.width = int(binary.BigEndian.Uint32(b[content : content+4]))
			property.height = int(binary.BigEndian.Uint32(b[content+4 : content+8]))
			if property.width == 0 || property.height == 0 {
				return nil, false
			}
		case "av1C":
			if child.end-child.payloadStart < 4 || b[child.payloadStart] != 0x81 {
				return nil, false
			}
		}
		properties[uint32(index+1)] = property
	}
	return properties, true
}

func avifPropertyAssociations(b []byte, box avifBox, associations map[uint32]map[uint32]bool) bool {
	version, flags, content, ok := avifFullBox(b, box)
	if !ok || version > 1 || flags&^uint32(1) != 0 || box.end-content < 4 {
		return false
	}
	entryCount := binary.BigEndian.Uint32(b[content : content+4])
	content += 4
	for entry := uint32(0); entry < entryCount; entry++ {
		var itemID uint32
		if version == 0 {
			if box.end-content < 2 {
				return false
			}
			itemID = uint32(binary.BigEndian.Uint16(b[content : content+2]))
			content += 2
		} else {
			if box.end-content < 4 {
				return false
			}
			itemID = binary.BigEndian.Uint32(b[content : content+4])
			content += 4
		}
		if box.end-content < 1 {
			return false
		}
		associationCount := int(b[content])
		content++
		if associations[itemID] == nil {
			associations[itemID] = make(map[uint32]bool, associationCount)
		}
		for association := 0; association < associationCount; association++ {
			var propertyIndex uint32
			if flags&1 != 0 {
				if box.end-content < 2 {
					return false
				}
				propertyIndex = uint32(binary.BigEndian.Uint16(b[content:content+2]) & 0x7fff)
				content += 2
			} else {
				if box.end-content < 1 {
					return false
				}
				propertyIndex = uint32(b[content] & 0x7f)
				content++
			}
			associations[itemID][propertyIndex] = true
		}
	}
	return content == box.end
}

func avifPrimaryItemHasData(b []byte, box avifBox, primaryItemID uint32, mediaData, itemData []avifByteRange) bool {
	version, flags, content, ok := avifFullBox(b, box)
	if !ok || version > 2 || flags != 0 || box.end-content < 2 {
		return false
	}
	offsetSize := int(b[content] >> 4)
	lengthSize := int(b[content] & 0x0f)
	baseOffsetSize := int(b[content+1] >> 4)
	indexSize := int(b[content+1] & 0x0f)
	content += 2
	if offsetSize > 8 || lengthSize > 8 || baseOffsetSize > 8 || indexSize > 8 || (version == 0 && indexSize != 0) {
		return false
	}
	var itemCount uint32
	if version < 2 {
		if box.end-content < 2 {
			return false
		}
		itemCount = uint32(binary.BigEndian.Uint16(b[content : content+2]))
		content += 2
	} else {
		if box.end-content < 4 {
			return false
		}
		itemCount = binary.BigEndian.Uint32(b[content : content+4])
		content += 4
	}
	foundPrimary := false
	for item := uint32(0); item < itemCount; item++ {
		var itemID uint32
		if version < 2 {
			if box.end-content < 2 {
				return false
			}
			itemID = uint32(binary.BigEndian.Uint16(b[content : content+2]))
			content += 2
		} else {
			if box.end-content < 4 {
				return false
			}
			itemID = binary.BigEndian.Uint32(b[content : content+4])
			content += 4
		}
		constructionMethod := uint16(0)
		if version == 1 || version == 2 {
			if box.end-content < 2 {
				return false
			}
			methodField := binary.BigEndian.Uint16(b[content : content+2])
			if methodField&0xfff0 != 0 {
				return false
			}
			constructionMethod = methodField & 0x000f
			content += 2
		}
		if box.end-content < 2 {
			return false
		}
		dataReferenceIndex := binary.BigEndian.Uint16(b[content : content+2])
		content += 2
		baseOffset, next, ok := avifVariableUint(b, content, box.end, baseOffsetSize)
		if !ok {
			return false
		}
		content = next
		if box.end-content < 2 {
			return false
		}
		extentCount := int(binary.BigEndian.Uint16(b[content : content+2]))
		content += 2
		isPrimary := itemID == primaryItemID
		if isPrimary && (foundPrimary || dataReferenceIndex != 0 || extentCount == 0 || lengthSize == 0 || constructionMethod > 1) {
			return false
		}
		for extent := 0; extent < extentCount; extent++ {
			if (version == 1 || version == 2) && indexSize > 0 {
				_, content, ok = avifVariableUint(b, content, box.end, indexSize)
				if !ok {
					return false
				}
			}
			extentOffset, next, ok := avifVariableUint(b, content, box.end, offsetSize)
			if !ok {
				return false
			}
			content = next
			extentLength, next, ok := avifVariableUint(b, content, box.end, lengthSize)
			if !ok {
				return false
			}
			content = next
			if !isPrimary {
				continue
			}
			start := baseOffset + extentOffset
			if start < baseOffset || extentLength == 0 || !avifExtentInData(start, extentLength, constructionMethod, mediaData, itemData) {
				return false
			}
		}
		if isPrimary {
			foundPrimary = true
		}
	}
	return foundPrimary && content == box.end
}

func avifVariableUint(b []byte, start, end, size int) (uint64, int, bool) {
	if size < 0 || size > 8 || start < 0 || end-start < size || end > len(b) {
		return 0, start, false
	}
	var value uint64
	for _, part := range b[start : start+size] {
		value = value<<8 | uint64(part)
	}
	return value, start + size, true
}

func avifExtentInData(start, length uint64, constructionMethod uint16, mediaData, itemData []avifByteRange) bool {
	end := start + length
	if end < start {
		return false
	}
	ranges := mediaData
	if constructionMethod == 1 {
		ranges = itemData
		startOffset, endOffset := start, end
		for _, dataRange := range ranges {
			available := dataRange.end - dataRange.start
			if startOffset <= available && endOffset <= available {
				return true
			}
		}
		return false
	}
	for _, dataRange := range ranges {
		if start >= dataRange.start && end <= dataRange.end {
			return true
		}
	}
	return false
}
