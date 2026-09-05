package web

import (
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
	"github.com/go-chi/chi/v5"
)

const adminMediaBodyLimit int64 = 10 * 1024 * 1024

type adminManagementAPI struct {
	repository *sqlite.PostRepository
	mediaDir   string
	clock      func() time.Time
}
type taxonomyRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}
type mediaAltRequest struct {
	AltText string `json:"alt_text"`
}

func newAdminManagementAPI(repository *sqlite.PostRepository, mediaDir string, clock func() time.Time) *adminManagementAPI {
	return &adminManagementAPI{repository: repository, mediaDir: mediaDir, clock: clock}
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
	if len(b) < 20 || string(b[:4]) != "RIFF" || string(b[8:12]) != "WEBP" || int(binary.LittleEndian.Uint32(b[4:8])) != len(b)-8 {
		return 0, 0, false
	}
	var width, height int
	imageChunk := false
	for offset := 12; offset < len(b); {
		if offset+8 > len(b) {
			return 0, 0, false
		}
		chunkType := string(b[offset : offset+4])
		size := int(binary.LittleEndian.Uint32(b[offset+4 : offset+8]))
		start, end := offset+8, offset+8+size
		if size < 0 || end > len(b) {
			return 0, 0, false
		}
		data := b[start:end]
		switch chunkType {
		case "VP8X":
			if len(data) != 10 || data[1] != 0 || data[2] != 0 {
				return 0, 0, false
			}
			width = int(data[4]) | int(data[5])<<8 | int(data[6])<<16
			width++
			height = int(data[7]) | int(data[8])<<8 | int(data[9])<<16
			height++
		case "VP8 ":
			if len(data) <= 10 || string(data[3:6]) != "\x9d\x01\x2a" {
				return 0, 0, false
			}
			width, height = int(binary.LittleEndian.Uint16(data[6:8])&0x3fff), int(binary.LittleEndian.Uint16(data[8:10])&0x3fff)
			imageChunk = true
		case "VP8L":
			if len(data) <= 5 || data[0] != 0x2f {
				return 0, 0, false
			}
			value := binary.LittleEndian.Uint32(data[1:5])
			width, height = int(value&0x3fff)+1, int((value>>14)&0x3fff)+1
			imageChunk = true
		case "ANMF":
			if len(data) <= 16 {
				return 0, 0, false
			}
			imageChunk = true
		}
		offset = end
		if size%2 == 1 {
			offset++
		}
	}
	return width, height, imageChunk && width > 0 && height > 0
}

func avifSize(b []byte) (int, int, bool) {
	if len(b) < 24 || string(b[4:8]) != "ftyp" {
		return 0, 0, false
	}
	ftypSize := int(binary.BigEndian.Uint32(b[:4]))
	if ftypSize < 16 || ftypSize > len(b) || !avifBrand(b[8:ftypSize]) {
		return 0, 0, false
	}
	var meta, iprp, ipco, hdlr, pitm, iinf, iloc, ipma, mdat, ispe bool
	var width, height int
	var walk func(int, int, int) bool
	walk = func(start, end, depth int) bool {
		if depth > 8 {
			return false
		}
		for offset := start; offset < end; {
			if offset+8 > end {
				return false
			}
			size := int(binary.BigEndian.Uint32(b[offset : offset+4]))
			if size < 8 || offset+size > end {
				return false
			}
			kind, payloadStart, payloadEnd := string(b[offset+4:offset+8]), offset+8, offset+size
			switch kind {
			case "meta":
				meta = true
				if payloadStart+4 > payloadEnd || !walk(payloadStart+4, payloadEnd, depth+1) {
					return false
				}
			case "iprp":
				iprp = true
				if !walk(payloadStart, payloadEnd, depth+1) {
					return false
				}
			case "ipco":
				ipco = true
				if !walk(payloadStart, payloadEnd, depth+1) {
					return false
				}
			case "hdlr":
				hdlr = payloadEnd-payloadStart >= 20
			case "pitm":
				pitm = payloadEnd-payloadStart >= 6
			case "iinf":
				iinf = payloadEnd-payloadStart >= 6
			case "iloc":
				iloc = payloadEnd-payloadStart >= 10
			case "ipma":
				ipma = payloadEnd-payloadStart >= 12
			case "mdat":
				mdat = payloadEnd > payloadStart
			case "ispe":
				if payloadStart+12 > payloadEnd {
					return false
				}
				width, height = int(binary.BigEndian.Uint32(b[payloadStart+4:payloadStart+8])), int(binary.BigEndian.Uint32(b[payloadStart+8:payloadStart+12]))
				ispe = width > 0 && height > 0
			}
			offset += size
		}
		return true
	}
	if !walk(0, len(b), 0) {
		return 0, 0, false
	}
	return width, height, meta && iprp && ipco && hdlr && pitm && iinf && iloc && ipma && mdat && ispe
}

func avifBrand(ftyp []byte) bool {
	if len(ftyp) < 8 {
		return false
	}
	for offset := 0; offset+4 <= len(ftyp); offset += 4 {
		if brand := string(ftyp[offset : offset+4]); brand == "avif" || brand == "avis" {
			return true
		}
	}
	return false
}
