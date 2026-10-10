package doomruntime

import (
	"fmt"
	"time"

	"gddoom/internal/netgame"
)

// updateAuthoritySession runs before the host decides which UI owns this frame.
// Map control is reliable and must be consumed before a new baseline. Menus send
// neutral intent on the same bounded clock; they never pause a network match.
func (sg *sessionGame) updateAuthoritySession() error {
	return sg.updateAuthoritySessionAt(time.Now())
}

func (sg *sessionGame) updateAuthoritySessionAt(now time.Time) error {
	client := sg.opts.AuthorityClient
	if client == nil {
		return nil
	}
	if resumed, ok := client.(interface {
		PollSessionChange() (netgame.ClientSessionChange, bool)
	}); ok {
		previous := client.Welcome()
		if change, changed := resumed.PollSessionChange(); changed {
			key, err := change.Manifest.Key()
			if err != nil {
				return fmt.Errorf("multiplayer resume manifest: %w", err)
			}
			if sg.g == nil || previous.Epoch != change.Welcome.Epoch || string(sg.g.m.Name) != change.Manifest.Map {
				if err := sg.applyAuthorityMapChange(netgame.MapChange{PreviousEpoch: previous.Epoch, Welcome: change.Welcome, Map: change.Manifest.Map, Compatibility: key}); err != nil {
					return err
				}
			}
			// A resumed connection has new sequence/ack state even if the same
			// vulnerable body survived in the same map epoch.
			sg.g.resetAuthorityClientPrediction()
		}
	}
	if sg.g != nil && sg.g.authorityFailure != nil {
		return nil
	}
	for {
		change, ok, err := client.PollTransition()
		if err != nil {
			return err
		}
		if !ok {
			break
		}
		if err := sg.applyAuthorityMapChange(change); err != nil {
			return err
		}
	}
	if sg.g != nil && (sg.quitPrompt.Active || sg.transitionActive() || sg.frontend.Active || sg.intermission.state.Active || sg.finale.Active) {
		return sg.g.updateAuthoritativeClientAt(now, nil)
	}
	return nil
}

func (sg *sessionGame) applyAuthorityMapChange(change netgame.MapChange) error {
	if sg.opts.AuthorityMapLoader == nil {
		return fmt.Errorf("multiplayer map loader unavailable")
	}
	m, err := sg.opts.AuthorityMapLoader(change)
	if err != nil {
		return fmt.Errorf("multiplayer map change: %w", err)
	}
	if m == nil || string(m.Name) != change.Map {
		return fmt.Errorf("multiplayer map loader returned incorrect map")
	}
	sg.stopAndClearMusic()
	if sg.rt != nil {
		sg.rt.clearPendingSoundState()
	}
	sg.levelCarryover = nil // the new baseline owns every player's inventory
	sg.opts.PlayerSlot = int(change.Welcome.PlayerID)
	sg.current = m.Name
	sg.currentTemplate = cloneMapForRestart(m)
	sg.rebuildGameWithPersistentSettings(m)
	sg.intermission = sessionIntermission{}
	sg.finale.Active = false
	sg.transition.Clear()
	sg.playMusicForMap(m.Name)
	sg.announceMapMusic(m.Name)
	return nil
}
