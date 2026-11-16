package media

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"somun/internal/store"
)

// sniffMime verifies file bytes against the claimed extension and returns
// the stored MIME type. Extension alone never decides: every branch checks
// magic bytes (W14-1). docx needs the assembled file (zip central
// directory lives at the end); other kinds use the head only.
func sniffMime(ext string, head []byte, asmPath string) (string, error) {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
		if mime := http.DetectContentType(head); strings.HasPrefix(mime, "image/") {
			return mime, nil
		}
		return "", ErrForgery
	case ".heic":
		if mime, ok := ftypMime(head); ok && mime == "image/heic" {
			return mime, nil
		}
		return "", ErrForgery
	case ".mp4", ".mov", ".webm":
		if mime := http.DetectContentType(head); strings.HasPrefix(mime, "video/") {
			return mime, nil
		}
		// Go's sniffer misses MP4/MOV (ftyp boxes): check the brand.
		if mime, ok := ftypMime(head); ok && strings.HasPrefix(mime, "video/") {
			return mime, nil
		}
		return "", ErrForgery
	case ".pdf":
		if http.DetectContentType(head) == "application/pdf" {
			return "application/pdf", nil
		}
		return "", ErrForgery
	case ".txt", ".md":
		if mime := http.DetectContentType(head); strings.HasPrefix(mime, "text/plain") {
			return mime, nil
		}
		return "", ErrForgery
	case ".docx":
		if len(head) < 4 || head[0] != 'P' || head[1] != 'K' || head[2] != 3 || head[3] != 4 {
			return "", ErrForgery
		}
		zr, err := zip.OpenReader(asmPath)
		if err != nil {
			return "", ErrForgery
		}
		defer zr.Close()
		for _, f := range zr.File {
			if f.Name == "word/document.xml" {
				return "application/vnd.openxmlformats-officedocument.wordprocessingml.document", nil
			}
		}
		return "", ErrForgery
	}
	return "", ErrBadUpload
}

// ftypMime reads an ISO-BMFF brand box (MP4/MOV/HEIC). Go's sniffer does
// not know these, so real phone videos and HEIC photos need this branch.
func ftypMime(head []byte) (string, bool) {
	if len(head) < 12 || string(head[4:8]) != "ftyp" {
		return "", false
	}
	brand := string(head[8:12])
	switch brand {
	case "heic", "heix", "hevc", "hevx", "mif1", "msf1":
		return "image/heic", true
	case "qt  ":
		return "video/quicktime", true
	}
	// MP4-family brands (isom, iso2, mp41, mp42, avc1, ...): ftyp box
	// present, not HEIC/QuickTime, so treat as MP4.
	for _, c := range []byte(brand) {
		if c < 0x20 || c > 0x7e {
			return "", false
		}
	}
	return "video/mp4", true
}

// thumbMax is the long edge of generated thumbnails.
const thumbMax = 320

// makeThumb decodes srcPath (image/*) and writes a JPEG thumbnail beside
// it under thumbs/. It returns the thumb's relative path plus the source
// dimensions. Anything undecodable fails soft: ("", 0, 0, nil).
func makeThumb(uploadDir, rel, sha string) (thumbRel string, w, h int, err error) {
	raw, err := os.ReadFile(filepath.Join(uploadDir, rel))
	if err != nil {
		return "", 0, 0, nil
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return "", 0, 0, nil
	}
	b := img.Bounds()
	w, h = b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return "", 0, 0, nil
	}
	scale := 1.0
	longest := w
	if h > longest {
		longest = h
	}
	if longest > thumbMax {
		scale = float64(thumbMax) / float64(longest)
	}
	tw := int(float64(w) * scale)
	th := int(float64(h) * scale)
	if tw < 1 {
		tw = 1
	}
	if th < 1 {
		th = 1
	}
	out := image.NewRGBA(image.Rect(0, 0, tw, th))
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			sx := b.Min.X + x*w/tw
			sy := b.Min.Y + y*h/th
			out.Set(x, y, img.At(sx, sy))
		}
	}
	dir := filepath.Join(uploadDir, "thumbs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", w, h, nil
	}
	name := sha
	if len(name) > 32 {
		name = name[:32]
	}
	if name == "" {
		name = "thumb"
	}
	thumbRel = filepath.ToSlash(filepath.Join("thumbs", name+".jpg"))
	f, err := os.Create(filepath.Join(uploadDir, thumbRel))
	if err != nil {
		return "", w, h, nil
	}
	defer f.Close()
	if err := jpeg.Encode(f, out, &jpeg.Options{Quality: 80}); err != nil {
		os.Remove(f.Name())
		return "", w, h, nil
	}
	return thumbRel, w, h, nil
}

// mimeExt maps stored MIME types to download extensions.
func mimeExt(mime, fallback string) string {
	switch strings.ToLower(strings.Split(mime, ";")[0]) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "image/heic":
		return ".heic"
	case "video/mp4":
		return ".mp4"
	case "video/quicktime":
		return ".mov"
	case "video/webm":
		return ".webm"
	case "application/pdf":
		return ".pdf"
	case "text/plain":
		return ".txt"
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return ".docx"
	}
	if e := strings.ToLower(filepath.Ext(fallback)); e != "" {
		return e
	}
	return ".bin"
}

// DownloadName builds the W14-5 file name:
// <입력ID>_<채널>_<종류>_<순번>.<확장자>.
func DownloadName(m *store.Media, channel string) string {
	input := int64(0)
	if m.ProductID != nil {
		input = *m.ProductID
	}
	if channel == "" {
		channel = "post"
	}
	return fmt.Sprintf("%d_%s_%s_%d%s", input, channel, m.Kind, m.ID, mimeExt(m.Mime, m.Path))
}

// contentChannel resolves the first post channel of a work for naming.
func (s *Service) contentChannel(m *store.Media) string {
	if m.ContentID != nil {
		if c, err := s.store.GetContent(*m.ContentID); err == nil && c.Channel != "" {
			return c.Channel
		}
	}
	if m.ProductID != nil {
		if c, err := s.store.FirstContentChannel(*m.ProductID); err == nil {
			return c
		}
	}
	return ""
}

// SweepStale deletes generated-but-unused files older than retentionDays
// (rows plus bytes). Work/post-linked files are always preserved.
func (s *Service) SweepStale(retentionDays int, now time.Time) (int, error) {
	if retentionDays < 1 {
		retentionDays = 30
	}
	cutoff := now.UTC().AddDate(0, 0, -retentionDays).Format(time.RFC3339)
	stale, err := s.store.DeleteStaleMedia(cutoff)
	if err != nil {
		return 0, err
	}
	for _, m := range stale {
		os.Remove(filepath.Join(s.uploadDir, m.Path))
		if m.ThumbPath != "" {
			os.Remove(filepath.Join(s.uploadDir, m.ThumbPath))
		}
	}
	return len(stale), nil
}

// parseMediaID parses a decimal media id from a URL path value.
func parseMediaID(raw string) (int64, error) {
	return strconv.ParseInt(raw, 10, 64)
}
