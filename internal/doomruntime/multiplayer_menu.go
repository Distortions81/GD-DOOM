package doomruntime

import (
	"context"
	"fmt"
	"image/color"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gddoom/internal/mapdata"
	"gddoom/internal/runtimecfg"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

type authorityJoinReply struct {
	result runtimecfg.AuthorityJoinResult
	err    error
}
type authorityJoinAttempt struct {
	ctx    context.Context
	cancel context.CancelFunc
	timer  *time.Timer
	result chan authorityJoinReply
}
type authorityMenuState struct {
	initialized      bool
	request          runtimecfg.AuthorityJoinRequest
	row              int
	editing, replace bool
	editOriginal     string
	status           string
	attempt          *authorityJoinAttempt
	servers          []authorityBrowserServer
	selected, scroll int
	refresh          *authorityBrowserRefresh
	probeSlots       chan struct{}
	editName         bool
	editNew          bool
	moreOptions      bool
	lobby            authorityLobbyMenu
	joinLabel        string
	content          *authorityContentAttempt
	replacement      *authorityContentReplacement
	sessionCancel    context.CancelFunc
	localRules       *runtimecfg.AuthorityLocalRules
	localMap         *mapdata.Map
}

func (sg *sessionGame) multiplayerMenuAvailable() bool {
	return sg != nil && sg.nativePatches == nil && (sg.opts.AuthorityJoin != nil || sg.opts.AuthorityClient != nil)
}

// Keep the shared frontend's stock action IDs stable while placing Multiplayer
// immediately below New Game in the menu's visual and keyboard order.
const frontendMultiplayerMenuItem = len(frontendMainMenuNames)

var frontendMultiplayerMenuOrder = [...]int{0, frontendMultiplayerMenuItem, 1, 2, 3, 4, 5}

func (sg *sessionGame) frontendMainMenuRow(item int) int {
	if sg.multiplayerMenuAvailable() {
		for row, candidate := range frontendMultiplayerMenuOrder {
			if candidate == item {
				return row
			}
		}
	}
	return item
}

func (sg *sessionGame) frontendMainMenuCount() int {
	count := len(frontendMainMenuNames)
	if sg.multiplayerMenuAvailable() {
		count++
	}
	return count
}
func (sg *sessionGame) openFrontendMultiplayer() {
	menu := &sg.multiplayer
	if !menu.initialized {
		menu.request = sg.opts.AuthorityJoinDefaults
		if strings.TrimSpace(menu.request.Name) == "" {
			menu.request.Name = "Player"
		}
		menu.initialized = true
		sg.initializeAuthorityBrowser()
	}
	menu.showServers()
	menu.editing = false
	if sg.opts.AuthorityClient != nil {
		menu.row = 0
		menu.lobby.page = authorityLobbyPageNone
	}
	sg.frontend.Mode, sg.frontend.MenuActive = frontendModeMultiplayer, true
	if sg.opts.AuthorityClient == nil {
		sg.openAuthorityMultiplayerHome()
	}
}

func (sg *sessionGame) tickFrontendMultiplayer() error {
	menu := &sg.multiplayer
	sg.frontend.Tic++
	sg.pollAuthorityServers()
	sg.pollAuthorityLobby()
	sg.pollAuthorityCreate()
	sg.pollAuthorityUpload()
	escape := sg.keyJustPressed(ebiten.KeyEscape) || sg.touchJustPressed(touchActionBack)
	selectPressed := sg.keyJustPressed(ebiten.KeyEnter) || sg.keyJustPressed(ebiten.KeyKPEnter) || sg.touchJustPressed(touchActionUseEnter)
	if menu.content != nil {
		if escape {
			sg.cancelAuthorityContent()
			menu.status = "WAD DOWNLOAD CANCELED"
			sg.playMenuBackSound()
		} else {
			sg.pollAuthorityContent()
		}
		return nil
	}
	if menu.replacement != nil {
		return nil
	}
	if menu.attempt != nil {
		if escape {
			sg.cancelAuthorityJoin()
			menu.status = "JOIN CANCELED"
			sg.playMenuBackSound()
		}
		return nil
	}
	if menu.lobby.creating != nil {
		if escape {
			sg.cancelAuthorityCreate()
			menu.status = "CREATION CANCELED - RETRY TO CHECK RESULT"
			sg.playMenuBackSound()
		}
		return nil
	}
	if menu.lobby.uploading != nil {
		if escape {
			sg.cancelAuthorityUpload()
			menu.status = "UPLOAD CANCELED - SAFE TO RETRY"
			sg.playMenuBackSound()
		}
		return nil
	}
	if menu.editing {
		value := &menu.request.Address
		limit := 512
		if menu.editName {
			value, limit = &menu.request.Name, 64
		}
		if menu.lobby.editRoomName {
			value, limit = &menu.lobby.request.Name, 64
		}
		if escape {
			*value, menu.editing = menu.editOriginal, false
			menu.lobby.editRoomName = false
			sg.playMenuBackSound()
			return nil
		}
		if sg.input.controlHeld && sg.keyJustPressed(ebiten.KeyA) {
			menu.replace = true
		}
		if sg.keyJustPressed(ebiten.KeyBackspace) || sg.keyJustPressed(ebiten.KeyDelete) {
			if menu.replace {
				*value = ""
			} else if len(*value) != 0 {
				_, n := utf8.DecodeLastRuneInString(*value)
				*value = (*value)[:len(*value)-n]
			}
			menu.replace = false
		}
		if !sg.input.controlHeld {
			for _, char := range sg.input.inputChars {
				if !unicode.IsPrint(char) || (!menu.editName && unicode.IsSpace(char)) {
					continue
				}
				if menu.replace {
					*value, menu.replace = "", false
				}
				if len(*value)+utf8.RuneLen(char) <= limit {
					*value += string(char)
				}
			}
		}
		sg.input.inputChars = nil
		// Host frames can collect text and Enter before the next menu tic.
		// Apply that text before committing the field.
		if selectPressed {
			*value = strings.TrimSpace(*value)
			if !menu.editName && !sg.commitAuthorityServerEdit() {
				return nil
			}
			menu.editing = false
			if menu.lobby.editRoomName {
				if *value != menu.editOriginal {
					menu.lobby.request.RequestID = ""
				}
				menu.lobby.editRoomName = false
			}
			sg.playMenuConfirmSound()
		}
		return nil
	}
	if menu.lobby.page != authorityLobbyPageNone && sg.opts.AuthorityClient == nil {
		return sg.tickAuthorityLobby(escape, selectPressed)
	}
	if escape {
		if menu.moreOptions {
			menu.showServers()
			sg.playMenuBackSound()
			return nil
		}
		if sg.opts.AuthorityClient == nil {
			sg.openAuthorityMultiplayerHome()
			sg.playMenuBackSound()
			return nil
		}
		sg.cancelAuthorityServerRefresh()
		sg.frontend.Mode, sg.frontend.ItemOn = frontendModeTitle, frontendMultiplayerMenuItem
		sg.playMenuBackSound()
		return nil
	}
	count := menu.actionStart() + len(menu.actions())
	connected := sg.opts.AuthorityClient != nil
	if connected {
		count = 2
	}
	if sg.keyJustPressed(ebiten.KeyArrowUp) || sg.touchJustPressed(touchActionUp) {
		menu.row = (menu.row + count - 1) % count
		sg.playMenuMoveSound()
	}
	if sg.keyJustPressed(ebiten.KeyArrowDown) || sg.keyJustPressed(ebiten.KeyTab) || sg.touchJustPressed(touchActionDown) {
		menu.row = (menu.row + 1) % count
		sg.playMenuMoveSound()
	}
	if !connected && !menu.moreOptions && menu.row < len(menu.servers) {
		menu.selectServer(menu.row)
	}
	if !connected && sg.keyJustPressed(ebiten.KeyF5) {
		sg.refreshAuthorityServers()
	}
	if !connected && sg.keyJustPressed(ebiten.KeyInsert) {
		menu.moreOptions = true
		menu.row = menu.actionRow(authorityBrowserAdd)
		sg.beginAuthorityServerEdit(false, true)
	}
	if !connected && sg.keyJustPressed(ebiten.KeyF2) {
		menu.moreOptions = true
		menu.row = menu.actionRow(authorityBrowserEdit)
		sg.beginAuthorityServerEdit(false, false)
	}
	if !selectPressed {
		return nil
	}
	sg.playMenuConfirmSound()
	if connected {
		if menu.row == 0 {
			sg.frontend = frontendState{}
			if sg.rt != nil {
				sg.rt.sessionSetFrontendActive(false)
			}
			sg.clearSampledInput()
			sg.suppressTouchUntilRelease()
		} else {
			sg.leaveAuthorityMatch()
		}
		return nil
	}
	if !menu.moreOptions && menu.row < len(menu.servers) {
		sg.beginAuthorityJoin()
		return nil
	}
	switch menu.actionAtRow() {
	case authorityBrowserRefreshAction:
		sg.refreshAuthorityServers()
	case authorityBrowserAdd:
		sg.beginAuthorityServerEdit(false, true)
	case authorityBrowserEdit:
		sg.beginAuthorityServerEdit(false, false)
	case authorityBrowserMore:
		menu.moreOptions, menu.row = true, 0
	case authorityBrowserBack:
		if menu.moreOptions {
			menu.showServers()
		} else {
			sg.openAuthorityMultiplayerHome()
		}
	}
	return nil
}

func localAuthorityRules(opts Options) runtimecfg.AuthorityLocalRules {
	return runtimecfg.AuthorityLocalRules{GameMode: opts.GameMode, SkillLevel: opts.SkillLevel, PlayerSlot: opts.PlayerSlot, NoMonsters: opts.NoMonsters, FastMonsters: opts.FastMonsters, RespawnMonsters: opts.RespawnMonsters, WADHash: opts.WADHash, AllCheats: opts.AllCheats, CheatLevel: opts.CheatLevel, Invulnerable: opts.Invulnerable, ShowAllItems: opts.ShowAllItems, ShowNoSkillItems: opts.ShowNoSkillItems}
}
func (sg *sessionGame) rememberAuthorityLocalRules() {
	if sg.multiplayer.localRules != nil {
		return
	}
	rules := localAuthorityRules(sg.opts)
	if sg.opts.AuthorityClient != nil {
		if sg.opts.AuthorityLocalRules != nil {
			rules = *sg.opts.AuthorityLocalRules
		} else {
			rules = runtimecfg.AuthorityLocalRules{GameMode: gameModeSingle, SkillLevel: 3, PlayerSlot: 1}
		}
	}
	sg.multiplayer.localRules = &rules
	if sg.bootMap != nil {
		sg.multiplayer.localMap = cloneMapForRestart(sg.bootMap)
	}
}
func leaveAuthorityConnection(client runtimecfg.AuthorityClient, cancel context.CancelFunc) {
	if cancel != nil {
		defer cancel()
	}
	if client == nil {
		return
	}
	if leave, ok := client.(interface{ Leave() error }); ok {
		_ = leave.Leave()
	} else if close, ok := client.(interface{ Close() error }); ok {
		_ = close.Close()
	}
}
func (sg *sessionGame) cancelAuthorityJoin() {
	attempt := sg.multiplayer.attempt
	if attempt == nil {
		return
	}
	sg.multiplayer.attempt = nil
	attempt.timer.Stop()
	select {
	case reply := <-attempt.result:
		// Preserve the live connection long enough to explicitly release a
		// completed join before canceling its lifetime context.
		go leaveAuthorityConnection(reply.result.Client, attempt.cancel)
		return
	default:
	}
	attempt.cancel()
	// Always inspect a late result and attempt leave. A canceled handshake
	// whose Welcome was lost is eventually released by the server's grace.
	go func() { reply := <-attempt.result; leaveAuthorityConnection(reply.result.Client, nil) }()
}
func (sg *sessionGame) beginAuthorityJoin() {
	menu := &sg.multiplayer
	if menu.lobby.page == authorityLobbyPageNone {
		menu.joinLabel = ""
	}
	if menu.attempt != nil || sg.opts.AuthorityClient != nil {
		return
	}
	if sg.opts.AuthorityJoin == nil {
		menu.status = "MULTIPLAYER IS UNAVAILABLE"
		return
	}
	if sg.opts.LiveTicSource != nil || sg.opts.LiveTicSink != nil || sg.opts.CoopPeers != nil || sg.opts.DemoScript != nil || strings.TrimSpace(sg.opts.RecordDemoPath) != "" || strings.TrimSpace(sg.opts.DemoTracePath) != "" {
		menu.status = "FINISH THE RECORDING OR REPLAY FIRST"
		return
	}
	request := menu.request
	request.Address, request.Name = strings.TrimSpace(request.Address), strings.TrimSpace(request.Name)
	if request.Address == "" {
		menu.status = "ADD OR SELECT A SERVER"
		menu.moreOptions = true
		menu.row = menu.actionRow(authorityBrowserAdd)
		return
	}
	if request.Name == "" {
		sg.openAuthorityPlayerSetup(menu.lobby.page)
		menu.status = "ENTER YOUR PLAYER NAME"
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	attempt := &authorityJoinAttempt{ctx: ctx, cancel: cancel, result: make(chan authorityJoinReply, 1)}
	// Bound opening without a deadline on the context retained by the client.
	attempt.timer = time.AfterFunc(30*time.Second, cancel)
	sg.cancelAuthorityServerRefresh()
	menu.attempt, menu.status, menu.request = attempt, "CONNECTING... ESC TO CANCEL", request
	join := sg.opts.AuthorityJoin
	go func() { result, err := join(ctx, request); attempt.result <- authorityJoinReply{result, err} }()
}
func (sg *sessionGame) pollAuthorityJoin() {
	attempt := sg.multiplayer.attempt
	if attempt == nil {
		return
	}
	select {
	case reply := <-attempt.result:
		attempt.timer.Stop()
		sg.multiplayer.attempt = nil
		if reply.err == nil && attempt.ctx.Err() != nil {
			reply.err = fmt.Errorf("connection timed out")
		}
		if reply.err == nil {
			reply.err = sg.installAuthorityJoin(reply.result)
		}
		if reply.err != nil {
			sg.multiplayer.status = reply.err.Error()
			go leaveAuthorityConnection(reply.result.Client, attempt.cancel)
			return
		}
		sg.multiplayer.sessionCancel = attempt.cancel
		sg.multiplayer.status = ""
	case <-attempt.ctx.Done():
		sg.cancelAuthorityJoin()
		sg.multiplayer.status = "CONNECTION TIMED OUT - TRY AGAIN"
	default:
	}
}
func (sg *sessionGame) installAuthorityJoin(result runtimecfg.AuthorityJoinResult) error {
	if result.Client == nil || result.Map == nil || result.MapLoader == nil || string(result.Map.Name) != result.Manifest.Map {
		return fmt.Errorf("server returned an incomplete session")
	}
	content, err := result.Manifest.ContentKey()
	if err != nil {
		return err
	}
	welcome := result.Client.Welcome()
	if welcome.Epoch == 0 || welcome.PlayerID > 4 {
		return fmt.Errorf("server returned an invalid player session")
	}
	sg.rememberAuthorityLocalRules()
	sg.capturePersistentSettings()
	sg.stopAndClearMusic()
	if sg.rt != nil {
		sg.rt.clearPendingSoundState()
	}
	sg.opts.AuthorityClient, sg.opts.AuthorityMapLoader = result.Client, result.MapLoader
	sg.opts.GameMode, sg.opts.SkillLevel, sg.opts.PlayerSlot, sg.opts.WADHash = result.Manifest.Mode, result.Manifest.Skill, int(welcome.PlayerID), content
	sg.opts.NoMonsters, sg.opts.FastMonsters, sg.opts.RespawnMonsters = result.Manifest.NoMonsters, result.Manifest.FastMonsters, result.Manifest.RespawnMonsters
	sg.opts.AllCheats, sg.opts.CheatLevel, sg.opts.Invulnerable = false, 0, false
	sg.opts.ShowAllItems, sg.opts.ShowNoSkillItems = false, false
	sg.opts.DemoScript, sg.opts.RecordDemoPath, sg.opts.DemoTracePath = nil, "", ""
	if sg.g != nil {
		// A finished attract demo must not carry its playback state into the
		// newly joined world through the normal map-rebuild path.
		sg.g.opts.DemoScript, sg.g.demoWorldDone = nil, false
	}
	sg.levelCarryover = nil
	sg.current, sg.currentTemplate = result.Map.Name, cloneMapForRestart(result.Map)
	sg.rebuildGameWithPersistentSettings(result.Map)
	sg.frontend, sg.intermission, sg.finale = frontendState{}, sessionIntermission{}, sessionFinale{}
	sg.quitPrompt = quitPromptState{}
	sg.transition.Clear()
	sg.err = nil
	sg.clearSampledInput()
	sg.suppressTouchUntilRelease()
	sg.playMusicForMap(sg.current)
	sg.announceMapMusic(sg.current)
	return nil
}
func (sg *sessionGame) leaveAuthorityMatch() {
	sg.rememberAuthorityLocalRules()
	sg.cancelAuthorityJoin()
	client, cancel := sg.opts.AuthorityClient, sg.multiplayer.sessionCancel
	sg.opts.AuthorityClient, sg.opts.AuthorityMapLoader, sg.multiplayer.sessionCancel = nil, nil, nil
	if sg.g != nil {
		sg.g.opts.AuthorityClient, sg.g.opts.AuthorityMapLoader = nil, nil
		sg.g.resetAuthorityClientPrediction()
	}
	go leaveAuthorityConnection(client, cancel)
	rules := sg.multiplayer.localRules
	sg.opts.GameMode, sg.opts.SkillLevel, sg.opts.PlayerSlot, sg.opts.WADHash = rules.GameMode, rules.SkillLevel, rules.PlayerSlot, rules.WADHash
	sg.opts.NoMonsters, sg.opts.FastMonsters, sg.opts.RespawnMonsters = rules.NoMonsters, rules.FastMonsters, rules.RespawnMonsters
	sg.opts.AllCheats, sg.opts.CheatLevel, sg.opts.Invulnerable = rules.AllCheats, rules.CheatLevel, rules.Invulnerable
	sg.opts.ShowAllItems, sg.opts.ShowNoSkillItems = rules.ShowAllItems, rules.ShowNoSkillItems
	sg.opts.DemoScript, sg.opts.RecordDemoPath, sg.opts.DemoTracePath = nil, "", ""
	sg.opts.LiveTicSource, sg.opts.LiveTicSink, sg.opts.CoopPeers = nil, nil, nil
	sg.opts.AuthorityLocalRules = nil
	sg.levelCarryover = nil
	sg.intermission, sg.finale, sg.quitPrompt = sessionIntermission{}, sessionFinale{}, quitPromptState{}
	sg.transition.Clear()
	sg.err = nil
	sg.stopAndClearMusic()
	if sg.multiplayer.localMap != nil {
		sg.bootMap = cloneMapForRestart(sg.multiplayer.localMap)
	}
	if sg.bootMap != nil {
		sg.current, sg.currentTemplate = sg.bootMap.Name, cloneMapForRestart(sg.bootMap)
		sg.rebuildGameWithPersistentSettings(cloneMapForRestart(sg.bootMap))
	}
	sg.multiplayer.localRules, sg.multiplayer.localMap = nil, nil
	sg.startFrontend()
	sg.frontend.MenuActive, sg.frontend.ItemOn = true, 0
	if sg.multiplayerMenuAvailable() {
		sg.frontend.ItemOn = frontendMultiplayerMenuItem
	}
	sg.frontendStatus("LEFT MULTIPLAYER MATCH", 105)
	sg.multiplayer.status = ""
	sg.clearSampledInput()
	sg.suppressTouchUntilRelease()
}
func (sg *sessionGame) closeAuthorityMultiplayer() {
	sg.cancelAuthorityJoin()
	sg.cancelAuthorityServerRefresh()
	sg.cancelAuthorityLobbyRefresh()
	sg.cancelAuthorityCreate()
	sg.cancelAuthorityUpload()
	sg.cancelAuthorityContent()
	client, cancel := sg.opts.AuthorityClient, sg.multiplayer.sessionCancel
	sg.opts.AuthorityClient, sg.opts.AuthorityMapLoader, sg.multiplayer.sessionCancel = nil, nil, nil
	leaveAuthorityConnection(client, cancel)
}

func (sg *sessionGame) drawFrontendMultiplayer(screen *ebiten.Image, scale, ox, oy float64) {
	menu := &sg.multiplayer
	// Keep Doom's menu lettering readable over gameplay and attract-demo
	// damage flashes. The opaque panel covers every row without changing
	// other frontend pages or their animated backdrops.
	ebitenutil.DrawRect(screen, ox+6*scale, oy+6*scale, 308*scale, 192*scale, color.RGBA{R: 36, G: 24, B: 12, A: 255})
	ebitenutil.DrawRect(screen, ox+8*scale, oy+8*scale, 304*scale, 188*scale, color.RGBA{R: 8, G: 8, B: 8, A: 255})
	text := func(value string, x, y int) {
		sg.drawFrontendTextAt(screen, value, ox+float64(x)*scale, oy+float64(y)*scale, scale, scale)
	}
	text(sg.authorityMultiplayerPageTitle(), 24, 16)
	text("BACK: ESC", 240, 16)
	if menu.content != nil {
		sg.drawAuthorityContent(text)
		return
	}
	if sg.opts.AuthorityClient != nil {
		text("IN A MULTIPLAYER MATCH", 32, 46)
		mode := strings.ToUpper(sg.opts.GameMode)
		if mode == "COOP" {
			mode = "CO-OP"
		}
		text(fmt.Sprintf("%s - %s", mode, sg.current), 32, 70)
		role := fmt.Sprintf("PLAYER %d", sg.opts.AuthorityClient.Welcome().PlayerID)
		if sg.opts.AuthorityClient.Welcome().PlayerID == 0 {
			role = "SPECTATOR"
		}
		text(role, 32, 88)
		text("RETURN TO GAME", 48, 116)
		text("LEAVE MATCH", 48, 140)
		sg.drawMenuSkull(screen, 16, 112+menu.row*24, scale, ox, oy)
		return
	}
	if menu.lobby.page != authorityLobbyPageNone && menu.attempt == nil && !menu.editing {
		sg.drawAuthorityLobby(text)
		return
	}
	sg.drawAuthorityBrowser(text)
}
