package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gddoom/internal/launchcatalog"
	"gddoom/internal/lobby"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/wad"
)

func resolveAuthorityLobbyURL(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	return lobby.NormalizeAddress(raw)
}

// A lobby selects exact content identities. A different stack is prepared
// independently and installed as a complete runtime before the normal join.
func configureAuthorityLobby(opts *runtimecfg.Options, paths []string, address string) error {
	address, err := resolveAuthorityLobbyURL(address)
	if err != nil {
		return err
	}
	if address == "" {
		return nil
	}
	hashes, err := launchcatalog.HashWADStackSHA256(paths)
	if err != nil {
		return fmt.Errorf("lobby content identity: %w", err)
	}
	opts.AuthorityLobbyURL = address
	opts.AuthorityWADHashes = slices.Clone(hashes)
	opts.AuthorityLobby = lobby.Fetch
	paths = slices.Clone(paths)
	// Capture the host callbacks before NewRuntime wraps them with a closure
	// bound to this particular runtime. A replacement must not keep it alive.
	settingsChanged, bindingsChanged := opts.OnRuntimeSettingsChanged, opts.OnInputBindingsChanged
	serversChanged, voiceChanged := opts.OnAuthorityServersChanged, opts.OnVoiceSettingsChanged
	restoreHostCallbacks := func(next *runtimecfg.Options) {
		next.OnRuntimeSettingsChanged, next.OnInputBindingsChanged = settingsChanged, bindingsChanged
		next.OnAuthorityServersChanged, next.OnVoiceSettingsChanged = serversChanged, voiceChanged
	}
	opts.AuthorityPrepareRoom = func(ctx context.Context, address string, room lobby.Room, progress func(runtimecfg.AuthorityContentProgress)) (runtimecfg.AuthorityContentPreparation, error) {
		return prepareAuthorityContent(ctx, address, room, paths, hashes, progress, restoreHostCallbacks)
	}
	opts.AuthorityUploadWADs = func(ctx context.Context, address, name string) (lobby.Pack, error) {
		request, err := loadedWADUpload(ctx, name, paths, hashes)
		if err != nil {
			return lobby.Pack{}, err
		}
		return lobby.Upload(ctx, address, request)
	}
	opts.AuthorityCreateGame = func(ctx context.Context, address string, request lobby.CreateRequest) (lobby.Room, error) {
		if err := ctx.Err(); err != nil {
			return lobby.Room{}, err
		}
		// Refresh the catalog before creating a process. UI compatibility is
		// advisory, and a catalog can have changed since the create page opened.
		state, err := lobby.Fetch(ctx, address)
		if err != nil {
			return lobby.Room{}, err
		}
		for _, pack := range state.Packs {
			if pack.ID != request.Settings.PackID {
				continue
			}
			if !slices.Equal(hashes, pack.WADHashes) {
				if _, err := lobby.RequiredDownloadBytes(pack, hashes); err != nil {
					return lobby.Room{}, fmt.Errorf("cannot load selected game files: %w", err)
				}
			}
			if _, err := lobby.ValidateSettings(request.Settings, pack); err != nil {
				return lobby.Room{}, err
			}
			return lobby.Create(ctx, address, request)
		}
		return lobby.Room{}, fmt.Errorf("selected WAD is no longer in the server catalog")
	}
	return nil
}

// The upload is the exact stack loaded by this game, including the base WAD.
// Rechecking the frozen identity prevents an edited native file from creating
// a server whose content differs from the already loaded client world.
func loadedWADUpload(ctx context.Context, name string, paths, hashes []string) (lobby.UploadRequest, error) {
	request := lobby.UploadRequest{Name: name}
	if len(paths) == 0 || len(paths) != len(hashes) || len(paths) > lobby.MaxUploadFiles {
		return request, fmt.Errorf("loaded WAD stack exceeds the upload file limit")
	}
	var total int64
	for i, path := range paths {
		if err := ctx.Err(); err != nil {
			return request, err
		}
		data, ok := wad.EmbeddedDataForPath(path)
		if !ok {
			file, err := os.Open(path)
			if err != nil {
				return request, fmt.Errorf("read loaded WAD: %w", err)
			}
			data, err = io.ReadAll(io.LimitReader(file, lobby.MaxUploadFileBytes+1))
			closeErr := file.Close()
			if err != nil {
				return request, err
			}
			if closeErr != nil {
				return request, closeErr
			}
		}
		size := int64(len(data))
		total += size
		if size == 0 || size > lobby.MaxUploadFileBytes || total > lobby.MaxUploadBytes {
			return request, fmt.Errorf("loaded WAD stack exceeds the upload size limit")
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != hashes[i] {
			return request, fmt.Errorf("loaded WAD files changed; reload them in the launcher before uploading")
		}
		request.Files = append(request.Files, lobby.UploadFile{Name: filepath.Base(path), Size: size, SHA256: hashes[i], Reader: bytes.NewReader(data)})
	}
	return request, nil
}
