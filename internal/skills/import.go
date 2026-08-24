package skills

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	MaximumImportBytes          = 4 * 1024 * 1024
	maximumBundleBytes          = 8 * 1024 * 1024
	maximumBundleFileBytes      = 1024 * 1024
	maximumRuntimeResourceBytes = 256 * 1024
	maximumBundleFiles          = 64
)

func parseImportedBundle(fileName string, data []byte) ([]File, error) {
	if len(data) == 0 {
		return nil, ErrInvalidBundle
	}
	if len(data) > MaximumImportBytes {
		return nil, ErrTooLarge
	}
	extension := strings.ToLower(path.Ext(strings.TrimSpace(fileName)))
	switch {
	case extension == ".md" || extension == ".markdown":
		if len(data) > maximumSkillBytes {
			return nil, ErrTooLarge
		}
		return []File{newFile("SKILL.md", data)}, nil
	case extension == ".zip" || bytes.HasPrefix(data, []byte{'P', 'K', 3, 4}):
		return parseZipBundle(data)
	default:
		return nil, ErrInvalidBundle
	}
}

func parseZipBundle(data []byte) ([]File, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, ErrInvalidBundle
	}
	entries := make([]*zip.File, 0, len(archive.File))
	cleanPaths := make([]string, 0, len(archive.File))
	for _, entry := range archive.File {
		cleaned, keep, err := cleanArchivePath(entry.Name, entry.FileInfo().IsDir())
		if err != nil {
			return nil, err
		}
		if !keep {
			continue
		}
		if entry.FileInfo().Mode()&os.ModeSymlink != 0 {
			return nil, ErrInvalidBundle
		}
		entries = append(entries, entry)
		cleanPaths = append(cleanPaths, cleaned)
	}
	rootPrefix, err := bundleRootPrefix(cleanPaths)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 || len(entries) > maximumBundleFiles {
		return nil, ErrInvalidBundle
	}
	files := make([]File, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	totalBytes := 0
	for index, entry := range entries {
		normalized := strings.TrimPrefix(cleanPaths[index], rootPrefix)
		if !allowedBundlePath(normalized) || len(normalized) > 240 {
			return nil, ErrInvalidBundle
		}
		if _, exists := seen[normalized]; exists {
			return nil, ErrInvalidBundle
		}
		seen[normalized] = struct{}{}
		reader, err := entry.Open()
		if err != nil {
			return nil, ErrInvalidBundle
		}
		content, readErr := io.ReadAll(io.LimitReader(reader, maximumBundleFileBytes+1))
		closeErr := reader.Close()
		if readErr != nil || closeErr != nil || len(content) > maximumBundleFileBytes {
			return nil, ErrTooLarge
		}
		totalBytes += len(content)
		if totalBytes > maximumBundleBytes {
			return nil, ErrTooLarge
		}
		files = append(files, newFile(normalized, content))
	}
	manifest, ok := fileByPath(files, "SKILL.md")
	if !ok || !utf8.Valid(manifest.Content) || len(manifest.Content) > maximumSkillBytes {
		return nil, ErrInvalidBundle
	}
	sort.Slice(files, func(left, right int) bool { return files[left].Path < files[right].Path })
	return files, nil
}

func cleanArchivePath(name string, directory bool) (string, bool, error) {
	if strings.ContainsAny(name, "\\\x00") || strings.HasPrefix(name, "/") {
		return "", false, ErrInvalidBundle
	}
	for _, segment := range strings.Split(name, "/") {
		if segment == ".." {
			return "", false, ErrInvalidBundle
		}
	}
	cleaned := path.Clean(name)
	if cleaned == "." || cleaned == "" || directory {
		return "", false, nil
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", false, ErrInvalidBundle
	}
	if strings.HasPrefix(cleaned, "__MACOSX/") || path.Base(cleaned) == ".DS_Store" {
		return "", false, nil
	}
	return cleaned, true, nil
}

func bundleRootPrefix(paths []string) (string, error) {
	for _, candidate := range paths {
		if candidate == "SKILL.md" {
			return "", nil
		}
	}
	var prefix string
	for _, candidate := range paths {
		if strings.HasSuffix(candidate, "/SKILL.md") {
			candidatePrefix := strings.TrimSuffix(candidate, "SKILL.md")
			if prefix != "" && prefix != candidatePrefix {
				return "", ErrInvalidBundle
			}
			prefix = candidatePrefix
		}
	}
	if prefix == "" {
		return "", ErrInvalidBundle
	}
	for _, candidate := range paths {
		if !strings.HasPrefix(candidate, prefix) {
			return "", ErrInvalidBundle
		}
	}
	return prefix, nil
}

func allowedBundlePath(filePath string) bool {
	return filePath == "SKILL.md" ||
		strings.HasPrefix(filePath, "references/") ||
		strings.HasPrefix(filePath, "assets/") ||
		strings.HasPrefix(filePath, "scripts/")
}

func newFile(filePath string, content []byte) File {
	hash := sha256.Sum256(content)
	mediaType := mime.TypeByExtension(strings.ToLower(path.Ext(filePath)))
	if mediaType == "" {
		mediaType = http.DetectContentType(content)
	}
	if filePath == "SKILL.md" {
		mediaType = "text/markdown"
	}
	return File{
		Path: filePath, MediaType: mediaType, SizeBytes: int64(len(content)),
		ContentHash:  "sha256:" + hex.EncodeToString(hash[:]),
		TextReadable: isTextResource(filePath, mediaType) && utf8.Valid(content) && len(content) <= maximumRuntimeResourceBytes,
		Content:      append([]byte(nil), content...),
	}
}

func isTextResource(filePath, mediaType string) bool {
	mediaType = strings.ToLower(strings.SplitN(mediaType, ";", 2)[0])
	if strings.HasPrefix(mediaType, "text/") {
		return true
	}
	switch mediaType {
	case "application/json", "application/ld+json", "application/xml", "application/yaml", "application/x-yaml", "application/toml", "application/javascript":
		return true
	}
	switch strings.ToLower(path.Ext(filePath)) {
	case ".md", ".txt", ".json", ".yaml", ".yml", ".toml", ".xml", ".csv", ".html", ".css", ".js", ".jsx", ".ts", ".tsx", ".go", ".py", ".rb", ".sh", ".bash", ".zsh", ".sql":
		return true
	default:
		return false
	}
}

func fileByPath(files []File, wanted string) (File, bool) {
	for _, file := range files {
		if file.Path == wanted {
			return file, true
		}
	}
	return File{}, false
}

func bundleContentHash(files []File) string {
	if len(files) == 1 && files[0].Path == "SKILL.md" {
		return files[0].ContentHash
	}
	ordered := append([]File(nil), files...)
	sort.Slice(ordered, func(left, right int) bool { return ordered[left].Path < ordered[right].Path })
	hash := sha256.New()
	for _, file := range ordered {
		hash.Write([]byte(file.Path))
		hash.Write([]byte{0})
		hash.Write(file.Content)
		hash.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}

func normalizedVersion(version, contentHash string) (string, error) {
	version = strings.TrimSpace(version)
	if version == "" || version == "local" {
		version = "local-" + strings.TrimPrefix(contentHash, "sha256:")[:12]
	}
	if utf8.RuneCountInString(version) > 80 || strings.ContainsAny(version, "\r\n\x00") {
		return "", ErrInvalid
	}
	return version, nil
}
