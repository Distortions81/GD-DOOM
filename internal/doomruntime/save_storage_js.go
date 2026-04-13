//go:build js && wasm

package doomruntime

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"syscall/js"
	"time"
)

const browserSaveSlotKeyPrefix = "gddoom-save:"

type browserSaveSlotEnvelope struct {
	Data    string `json:"d"`
	ModTime int64  `json:"t"`
}

func readSavedSlotData(slot int) ([]byte, error) {
	data, _, err := readSavedSlotDataWithModTime(slot)
	return data, err
}

func readSavedSlotDataWithModTime(slot int) ([]byte, time.Time, error) {
	raw, ok, err := browserGetItem(browserSaveSlotKey(slot))
	if err != nil {
		return nil, time.Time{}, err
	}
	if !ok {
		return nil, time.Time{}, os.ErrNotExist
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, time.Time{}, os.ErrNotExist
	}
	if strings.HasPrefix(raw, "{") {
		var env browserSaveSlotEnvelope
		if err := json.Unmarshal([]byte(raw), &env); err == nil {
			decoded, err := decodeBrowserSaveData(env.Data)
			if err != nil {
				return nil, time.Time{}, err
			}
			modTime := time.Time{}
			if env.ModTime > 0 {
				modTime = time.Unix(0, env.ModTime)
			}
			return decoded, modTime, nil
		}
	}
	decoded, err := decodeBrowserSaveData(raw)
	if err != nil {
		return nil, time.Time{}, err
	}
	return decoded, time.Time{}, nil
}

func writeSavedSlotData(slot int, data []byte) error {
	if len(data) == 0 {
		return errors.New("empty save data")
	}
	env := browserSaveSlotEnvelope{
		Data:    encodeBrowserSaveData(data),
		ModTime: time.Now().UTC().UnixNano(),
	}
	payload, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return browserSetItem(browserSaveSlotKey(slot), string(payload))
}

func listSavedSlots() ([]int, error) {
	store := browserSaveStorage()
	if store.IsUndefined() || store.IsNull() {
		return nil, nil
	}
	keys, err := browserStorageKeys()
	if err != nil {
		return nil, nil
	}
	seen := map[int]struct{}{}
	out := make([]int, 0, len(keys))
	for _, key := range keys {
		name := strings.TrimSpace(key)
		if strings.HasPrefix(strings.ToLower(name), browserSaveSlotKeyPrefix) {
			name = strings.TrimPrefix(name, browserSaveSlotKeyPrefix)
		}
		slot, ok := saveGameSlotFromFileName(name)
		if !ok {
			continue
		}
		if _, exists := seen[slot]; exists {
			continue
		}
		seen[slot] = struct{}{}
		out = append(out, slot)
	}
	sort.Ints(out)
	return out, nil
}

func browserSaveSlotKey(slot int) string {
	return browserSaveSlotKeyPrefix + saveGameBaseName(slot) + ".dsg"
}

func browserSaveStorage() js.Value {
	return js.Global().Get("localStorage")
}

func browserStorageKeys() ([]string, error) {
	store := browserSaveStorage()
	if store.IsUndefined() || store.IsNull() {
		return nil, errors.New("localStorage unavailable")
	}
	length := store.Get("length").Int()
	if length < 0 {
		length = 0
	}
	keys := make([]string, 0, length)
	for i := 0; i < length; i++ {
		key := store.Call("key", i).String()
		if key == "" {
			continue
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func browserGetItem(key string) (string, bool, error) {
	store := browserSaveStorage()
	if store.IsUndefined() || store.IsNull() {
		return "", false, errors.New("localStorage unavailable")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", false, nil
	}
	var (
		value js.Value
		err   error
	)
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("localStorage getItem failed: %v", r)
			value = js.Undefined()
		}
	}()
	value = store.Call("getItem", key)
	if value.IsUndefined() || value.IsNull() {
		return "", false, err
	}
	if err != nil {
		return "", false, err
	}
	return value.String(), true, nil
}

func browserSetItem(key, value string) error {
	store := browserSaveStorage()
	if store.IsUndefined() || store.IsNull() {
		return errors.New("localStorage unavailable")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("invalid save slot key")
	}
	var err error
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("localStorage setItem failed: %v", r)
		}
	}()
	store.Call("setItem", key, value)
	return err
}

func decodeBrowserSaveData(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, os.ErrNotExist
	}
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("decode save data: %w", err)
	}
	return data, nil
}

func encodeBrowserSaveData(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
