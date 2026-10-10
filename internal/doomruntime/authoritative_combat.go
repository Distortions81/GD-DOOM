package doomruntime

type authorityPlayerIdentity struct {
	Slot       int
	Generation uint32
}

// Barrel ownership outlives the damage callback and can propagate through a
// delayed chain reaction. It must never be inferred from the current anchor or
// from a monster target that changes when a player leaves or respawns.
type authorityBarrelSource struct {
	PlayerSlot       int
	PlayerGeneration uint32
	Thing            int
}

func (g *game) authoritativeSourceSlot() int {
	if len(g.authorityPlayers) == 0 {
		return 0
	}
	return g.localSlot
}

// applyPlayerDamageTarget binds damage to an explicit live slot. Source
// identity is scoped so later world damage cannot retain a previous shooter.
func (g *game) applyPlayerDamageTarget(targetSlot, sourceSlot int, damage func()) {
	g.applyPlayerDamageTargetFrom(targetSlot, authorityPlayerIdentity{Slot: sourceSlot, Generation: g.authorityPlayerGeneration(sourceSlot)}, damage)
}

func (g *game) applyPlayerDamageTargetFrom(targetSlot int, source authorityPlayerIdentity, damage func()) {
	if len(g.authorityPlayers) == 0 {
		damage()
		return
	}
	if targetSlot == 0 {
		targetSlot = g.localSlot
	}
	p := g.authoritativePlayerForSlot(targetSlot)
	if p == nil || !g.authorityPlayerDamageAllowed(source.Slot, targetSlot) {
		return
	}
	g.withAuthoritativePlayer(p, func() {
		previous := g.authorityDamageSource
		g.authorityDamageSource = 0
		if g.authoritativeSourceIdentityValid(source) {
			g.authorityDamageSource = source.Slot
		}
		defer func() { g.authorityDamageSource = previous }()
		damage()
	})
}

func (g *game) authoritativeSourceIdentityValid(source authorityPlayerIdentity) bool {
	return source.Slot >= 1 && source.Slot <= 4 && g.authoritativePlayerForSlot(source.Slot) != nil && source.Generation == g.authorityPlayerGeneration(source.Slot)
}

// withProjectileSource supplies context only for the original incarnation. A
// missile from a removed body still deals physical damage, but cannot borrow a
// replacement's position, inventory, frag credit, or monster retaliation target.
// The original identity remains available to delayed barrel chains and friendly
// fire policy even when no current player can receive credit.
func (g *game) withProjectileSource(slot int, generation uint32, action func(validPlayerSource bool)) {
	if len(g.authorityPlayers) == 0 {
		action(true)
		return
	}
	source := authorityPlayerIdentity{Slot: slot, Generation: generation}
	valid := g.authoritativeSourceIdentityValid(source)
	step := func() {
		previous, previousOwner := g.authorityDamageSource, g.authorityDamageOwner
		g.authorityDamageSource, g.authorityDamageOwner = 0, source
		if valid {
			g.authorityDamageSource = slot
		}
		defer func() { g.authorityDamageSource, g.authorityDamageOwner = previous, previousOwner }()
		action(valid)
	}
	if valid {
		g.withAuthoritativePlayer(g.authoritativePlayerForSlot(slot), step)
		return
	}
	step()
}
