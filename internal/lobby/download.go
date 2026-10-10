package lobby

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"runtime"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	MaxDownloadFiles     = MaxUploadFiles
	MaxDownloadFileBytes = MaxUploadFileBytes
	MaxDownloadBytes     = MaxUploadBytes
)

func validatePackFile(file PackFile) error {
	digest, err := hex.DecodeString(file.SHA256)
	if !validName(file.Name) || file.Name == "." || file.Name == ".." || strings.ContainsAny(file.Name, "/\\") || file.Size <= 0 || err != nil || len(digest) != sha256.Size || file.SHA256 != strings.ToLower(file.SHA256) {
		return errors.New("content file requires a plain name, positive size, and lowercase SHA-256 hash")
	}
	if file.Downloadable && file.Size > MaxDownloadFileBytes {
		return errors.New("downloadable WAD file exceeds 64 MiB")
	}
	if !validContentNotice(file.License, 256) || !validContentNotice(file.Source, 512) {
		return errors.New("content file license or source exceeds its printable text limit")
	}
	return nil
}

func validContentNotice(value string, limit int) bool {
	return utf8.ValidString(value) && strings.TrimSpace(value) == value && utf8.RuneCountInString(value) <= limit && strings.IndexFunc(value, unicode.IsControl) < 0
}

// ValidateDownloadPack checks the complete target stack before automatic
// loading. Locally available files may be private, but still count toward the
// stack bounds. Existing larger installed stacks remain valid for direct joins.
func ValidateDownloadPack(pack Pack) error {
	if err := ValidatePack(pack); err != nil {
		return err
	}
	if len(pack.Files) == 0 || len(pack.Files) > MaxDownloadFiles {
		return errors.New("automatic loading requires metadata for an ordered stack of 1–16 WAD files")
	}
	var total int64
	for _, file := range pack.Files {
		if file.Size > MaxDownloadFileBytes || file.Size > MaxDownloadBytes-total {
			return errors.New("automatic WAD loading exceeds the 64 MiB file or 128 MiB stack limit")
		}
		total += file.Size
	}
	return nil
}

// Download returns verified owned bytes for one operator-approved file. The
// caller chooses missing files by SHA-256 and preserves Pack.Files order when
// combining them with local WADs. No data is returned on cancellation, size or
// hash mismatch, or HTTP refusal. Redirects are forbidden on native and browser
// transports so content cannot silently move away from the selected lobby.
func Download(ctx context.Context, address string, file PackFile) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validatePackFile(file); err != nil {
		return nil, err
	}
	if !file.Downloadable {
		return nil, errors.New("this WAD is not approved for download; load a matching local copy")
	}
	address, err := NormalizeAddress(address)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address+"/api/v1/content/"+file.SHA256, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/octet-stream")
	if runtime.GOOS == "js" {
		request.Header.Set("js.fetch:redirect", "error")
		request.Header.Set("js.fetch:credentials", "omit")
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		// Do not render arbitrary binary/HTML bodies as a menu error.
		message := http.StatusText(response.StatusCode)
		if message == "" {
			message = "content request refused"
		}
		return nil, &HTTPError{StatusCode: response.StatusCode, Message: message}
	}
	contentType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || (contentType != "application/octet-stream" && contentType != "application/x-doom-wad") {
		return nil, errors.New("WAD download must use a binary content type")
	}
	if response.ContentLength >= 0 && response.ContentLength != file.Size {
		return nil, errors.New("WAD download does not match its declared size")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, file.Size+1))
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if int64(len(data)) != file.Size {
		return nil, errors.New("WAD download does not match its declared size")
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != file.SHA256 {
		return nil, errors.New("WAD download does not match its declared SHA-256 hash")
	}
	return data, nil
}
