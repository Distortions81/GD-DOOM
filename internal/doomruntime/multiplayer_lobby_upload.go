package doomruntime

import (
	"context"
	"slices"
	"time"

	"gddoom/internal/lobby"
)

type authorityUploadReply struct {
	pack lobby.Pack
	err  error
}

type authorityUploadAttempt struct {
	ctx    context.Context
	cancel context.CancelFunc
	reply  chan authorityUploadReply
}

func (sg *sessionGame) authorityUploadAvailable() bool {
	return sg.multiplayer.lobby.state.UploadsEnabled && sg.opts.AuthorityUploadWADs != nil
}

func (sg *sessionGame) cancelAuthorityUpload() {
	m := &sg.multiplayer.lobby
	if m.uploading != nil {
		m.uploading.cancel()
		m.uploading = nil
	}
}

func (sg *sessionGame) beginAuthorityUpload() {
	menu, m := &sg.multiplayer, &sg.multiplayer.lobby
	if !sg.authorityUploadAvailable() || m.uploading != nil || m.creating != nil || menu.attempt != nil || sg.opts.AuthorityClient != nil {
		return
	}
	if len(sg.opts.AuthorityWADHashes) == 0 {
		menu.status = "LOAD WADS BEFORE UPLOADING"
		return
	}
	sg.cancelAuthorityLobbyRefresh()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	job := &authorityUploadAttempt{ctx: ctx, cancel: cancel, reply: make(chan authorityUploadReply, 1)}
	m.uploading, menu.status = job, "UPLOADING LOADED WADS..."
	upload, address := sg.opts.AuthorityUploadWADs, sg.opts.AuthorityLobbyURL
	go func() {
		pack, err := upload(ctx, address, "Custom WADs")
		job.reply <- authorityUploadReply{pack, err}
	}()
}

func (sg *sessionGame) pollAuthorityUpload() {
	menu, m := &sg.multiplayer, &sg.multiplayer.lobby
	job := m.uploading
	if job == nil {
		return
	}
	select {
	case reply := <-job.reply:
		if reply.err == nil {
			reply.err = job.ctx.Err()
		}
		job.cancel()
		m.uploading = nil
		if reply.err != nil {
			menu.status = "UPLOAD: " + reply.err.Error()
			return
		}
		pack := reply.pack
		if pack.ID == "" || len(pack.Maps) == 0 || !sg.authorityPackMatches(pack.WADHashes) {
			menu.status = "UPLOAD RETURNED A DIFFERENT WAD STACK"
			return
		}
		// The uploaded stack is content-addressed. Repeated upload attempts
		// replace the same catalog entry instead of duplicating it.
		index := slices.IndexFunc(m.state.Packs, func(p lobby.Pack) bool { return p.ID == pack.ID })
		if index >= 0 {
			m.state.Packs[index] = pack
		} else {
			m.state.Packs = append([]lobby.Pack{pack}, m.state.Packs...)
			m.state.Packs = m.state.Packs[:min(len(m.state.Packs), authorityBrowserLimit)]
		}
		before := m.request.Settings
		m.request.Settings.PackID = pack.ID
		if before.PackID != pack.ID || !slices.Contains(pack.Maps, before.Map) {
			m.request.Settings.Map = pack.Maps[0]
		}
		if before != m.request.Settings {
			m.request.RequestID = ""
		}
		m.row = sg.authorityCreateActionRow()
		menu.status = "WADS READY - CREATE YOUR GAME"
	case <-job.ctx.Done():
		sg.cancelAuthorityUpload()
		menu.status = "UPLOAD TIMED OUT - SAFE TO RETRY"
	default:
	}
}
