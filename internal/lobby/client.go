package lobby

import (
	"bytes"
	"context"
	"encoding/json"
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

// HTTPError is a bounded server-supplied refusal, such as exhausted capacity.
// It never contains the raw response body or an HTML error page.
type HTTPError struct {
	StatusCode int
	Message    string
}

func (err *HTTPError) Error() string {
	return fmt.Sprintf("lobby: %s (HTTP %d)", err.Message, err.StatusCode)
}

func Fetch(ctx context.Context, address string) (State, error) {
	var state State
	if err := requestJSON(ctx, address, "/api/v1/lobby", http.MethodGet, nil, &state, 10*time.Second); err != nil {
		return State{}, err
	}
	if err := validateState(state); err != nil {
		return State{}, err
	}
	return state, nil
}

// Create performs one idempotent request. A canceled request may have created a
// room; callers can refresh or retry with the same RequestID, never a fresh ID.
func Create(ctx context.Context, address string, request CreateRequest) (Room, error) {
	if err := ValidateCreateRequest(request); err != nil {
		return Room{}, err
	}
	body, err := json.Marshal(request)
	if err != nil {
		return Room{}, err
	}
	if len(body) > MaxRequestBytes {
		return Room{}, errors.New("lobby creation request exceeds size limit")
	}
	var room Room
	if err := requestJSON(ctx, address, "/api/v1/rooms", http.MethodPost, body, &room, 30*time.Second); err != nil {
		return Room{}, err
	}
	if err := validateRoom(room, nil); err != nil {
		return Room{}, err
	}
	if room.State != "ready" || room.Name != request.Name || room.Settings != request.Settings {
		return Room{}, errors.New("lobby creation response does not match the requested ready room")
	}
	return room, nil
}

func requestJSON(ctx context.Context, address, suffix, method string, body []byte, target any, timeout time.Duration) error {
	contentType := ""
	if body != nil {
		contentType = "application/json"
	}
	return requestWithBody(ctx, address, suffix, method, bytes.NewReader(body), contentType, target, timeout)
}

func requestWithBody(ctx context.Context, address, suffix, method string, body io.Reader, contentType string, target any, timeout time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	address, err := NormalizeAddress(address)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, method, address+suffix, body)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	if runtime.GOOS == "js" {
		// Go's Fetch transport follows redirects inside the browser before the
		// ordinary Client redirect hook can intervene. Use its browser options.
		request.Header.Set("js.fetch:redirect", "error")
		request.Header.Set("js.fetch:credentials", "omit")
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.ContentLength > MaxResponseBytes {
		return errors.New("lobby response exceeds size limit")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, MaxResponseBytes+1))
	if err != nil {
		return err
	}
	if len(data) > MaxResponseBytes {
		return errors.New("lobby response exceeds size limit")
	}
	responseType, _, typeErr := mime.ParseMediaType(response.Header.Get("Content-Type"))
	validJSON := typeErr == nil && responseType == "application/json"
	success := response.StatusCode == http.StatusOK || (method == http.MethodPost && response.StatusCode == http.StatusCreated)
	if !success {
		message := http.StatusText(response.StatusCode)
		if message == "" {
			message = "request refused"
		}
		var failure struct {
			Error string `json:"error"`
		}
		if validJSON && decodeStrict(data, &failure) == nil && utf8.ValidString(failure.Error) && utf8.RuneCountInString(failure.Error) <= 512 && strings.TrimSpace(failure.Error) != "" && strings.IndexFunc(failure.Error, unicode.IsControl) < 0 {
			message = failure.Error
		}
		return &HTTPError{StatusCode: response.StatusCode, Message: message}
	}
	if !validJSON {
		return errors.New("lobby response must use application/json")
	}
	if err := decodeStrict(data, target); err != nil {
		return fmt.Errorf("invalid lobby response: %w", err)
	}
	return nil
}
