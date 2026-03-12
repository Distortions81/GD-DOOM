// Emacs style mode select   -*- C++ -*-
//-----------------------------------------------------------------------------
//
// Doom trace output helpers.
//
//-----------------------------------------------------------------------------

static const char
rcsid[] = "$Id: d_trace.c,v 1.0 2026/03/08 00:00:00 codex Exp $";

#include <stdio.h>
#include <string.h>

#include "doomdef.h"
#include "doomstat.h"
#include "d_event.h"
#include "d_player.h"
#include "p_local.h"
#include "p_spec.h"
#include "w_wad.h"
#include "i_system.h"

static FILE* tracefile;
static char* tracepath;
static char* pendingdemoname;
static boolean startupwritten;
static boolean demowritten;

extern int rndindex;
extern int prndindex;

static void Trace_WriteJSONString(char* value)
{
    char ch;

    fputc('"', tracefile);
    if (!value)
    {
        fputc('"', tracefile);
        return;
    }

    while ((ch = *value++) != 0)
    {
        if (ch == '\\' || ch == '"')
            fputc('\\', tracefile);
        fputc(ch, tracefile);
    }
    fputc('"', tracefile);
}

static char* Trace_GameStateName(gamestate_t state)
{
    switch (state)
    {
      case GS_LEVEL:
        return "GS_LEVEL";
      case GS_INTERMISSION:
        return "GS_INTERMISSION";
      case GS_FINALE:
        return "GS_FINALE";
      case GS_DEMOSCREEN:
        return "GS_DEMOSCREEN";
      default:
        return "GS_UNKNOWN";
    }
}

static char* Trace_GameActionName(gameaction_t action)
{
    switch (action)
    {
      case ga_nothing:
        return "ga_nothing";
      case ga_loadlevel:
        return "ga_loadlevel";
      case ga_newgame:
        return "ga_newgame";
      case ga_loadgame:
        return "ga_loadgame";
      case ga_savegame:
        return "ga_savegame";
      case ga_playdemo:
        return "ga_playdemo";
      case ga_completed:
        return "ga_completed";
      case ga_victory:
        return "ga_victory";
      case ga_worlddone:
        return "ga_worlddone";
      case ga_screenshot:
        return "ga_screenshot";
      default:
        return "ga_unknown";
    }
}

static int Trace_SectorIndex(sector_t* sector)
{
    if (!sector || !sectors)
        return -1;

    return sector - sectors;
}

static int Trace_StateIndex(state_t* state)
{
    if (!state)
        return -1;

    return state - states;
}

static void Trace_WriteMobj(mobj_t* mo)
{
    fprintf(tracefile, "{\"type\":%d", mo->type);
    fprintf(tracefile, ",\"x\":%d", mo->x);
    fprintf(tracefile, ",\"y\":%d", mo->y);
    fprintf(tracefile, ",\"z\":%d", mo->z);
    fprintf(tracefile, ",\"angle\":%u", mo->angle);
    fprintf(tracefile, ",\"momx\":%d", mo->momx);
    fprintf(tracefile, ",\"momy\":%d", mo->momy);
    fprintf(tracefile, ",\"momz\":%d", mo->momz);
    fprintf(tracefile, ",\"floorz\":%d", mo->floorz);
    fprintf(tracefile, ",\"ceilingz\":%d", mo->ceilingz);
    fprintf(tracefile, ",\"radius\":%d", mo->radius);
    fprintf(tracefile, ",\"height\":%d", mo->height);
    fprintf(tracefile, ",\"tics\":%d", mo->tics);
    fprintf(tracefile, ",\"state\":%d", Trace_StateIndex(mo->state));
    fprintf(tracefile, ",\"flags\":%d", mo->flags);
    fprintf(tracefile, ",\"health\":%d", mo->health);
    fprintf(tracefile, ",\"movedir\":%d", mo->movedir);
    fprintf(tracefile, ",\"movecount\":%d", mo->movecount);
    fprintf(tracefile, ",\"reactiontime\":%d", mo->reactiontime);
    fprintf(tracefile, ",\"threshold\":%d", mo->threshold);
    fprintf(tracefile, ",\"lastlook\":%d", mo->lastlook);
    fprintf(tracefile, ",\"subsector\":%d", mo->subsector != NULL);
    if (mo->subsector)
        fprintf(tracefile, ",\"sector\":%d",
                Trace_SectorIndex(mo->subsector->sector));
    fprintf(tracefile, ",\"player\":%d", mo->player != NULL);
    fprintf(tracefile, ",\"target\":%d", mo->target != NULL);
    if (mo->target)
        fprintf(tracefile, ",\"target_type\":%d", mo->target->type);
    fprintf(tracefile, ",\"tracer\":%d", mo->tracer != NULL);
    if (mo->tracer)
        fprintf(tracefile, ",\"tracer_type\":%d", mo->tracer->type);
    fprintf(tracefile, "}");
}

static void Trace_WriteDoor(vldoor_t* door)
{
    fprintf(tracefile, "{\"kind\":\"door\"");
    fprintf(tracefile, ",\"sector\":%d", Trace_SectorIndex(door->sector));
    fprintf(tracefile, ",\"type\":%d", door->type);
    fprintf(tracefile, ",\"topheight\":%d", door->topheight);
    fprintf(tracefile, ",\"speed\":%d", door->speed);
    fprintf(tracefile, ",\"direction\":%d", door->direction);
    fprintf(tracefile, ",\"topwait\":%d", door->topwait);
    fprintf(tracefile, ",\"topcountdown\":%d", door->topcountdown);
    fprintf(tracefile, "}");
}

static void Trace_WriteFloor(floormove_t* floor)
{
    fprintf(tracefile, "{\"kind\":\"floor\"");
    fprintf(tracefile, ",\"sector\":%d", Trace_SectorIndex(floor->sector));
    fprintf(tracefile, ",\"type\":%d", floor->type);
    fprintf(tracefile, ",\"crush\":%d", floor->crush);
    fprintf(tracefile, ",\"direction\":%d", floor->direction);
    fprintf(tracefile, ",\"newspecial\":%d", floor->newspecial);
    fprintf(tracefile, ",\"texture\":%d", floor->texture);
    fprintf(tracefile, ",\"floordestheight\":%d", floor->floordestheight);
    fprintf(tracefile, ",\"speed\":%d", floor->speed);
    fprintf(tracefile, "}");
}

static void Trace_WritePlat(plat_t* plat)
{
    fprintf(tracefile, "{\"kind\":\"plat\"");
    fprintf(tracefile, ",\"sector\":%d", Trace_SectorIndex(plat->sector));
    fprintf(tracefile, ",\"speed\":%d", plat->speed);
    fprintf(tracefile, ",\"low\":%d", plat->low);
    fprintf(tracefile, ",\"high\":%d", plat->high);
    fprintf(tracefile, ",\"wait\":%d", plat->wait);
    fprintf(tracefile, ",\"count\":%d", plat->count);
    fprintf(tracefile, ",\"status\":%d", plat->status);
    fprintf(tracefile, ",\"oldstatus\":%d", plat->oldstatus);
    fprintf(tracefile, ",\"crush\":%d", plat->crush);
    fprintf(tracefile, ",\"tag\":%d", plat->tag);
    fprintf(tracefile, ",\"type\":%d", plat->type);
    fprintf(tracefile, "}");
}

static void Trace_WriteCeiling(ceiling_t* ceiling)
{
    fprintf(tracefile, "{\"kind\":\"ceiling\"");
    fprintf(tracefile, ",\"sector\":%d", Trace_SectorIndex(ceiling->sector));
    fprintf(tracefile, ",\"type\":%d", ceiling->type);
    fprintf(tracefile, ",\"bottomheight\":%d", ceiling->bottomheight);
    fprintf(tracefile, ",\"topheight\":%d", ceiling->topheight);
    fprintf(tracefile, ",\"speed\":%d", ceiling->speed);
    fprintf(tracefile, ",\"crush\":%d", ceiling->crush);
    fprintf(tracefile, ",\"direction\":%d", ceiling->direction);
    fprintf(tracefile, ",\"tag\":%d", ceiling->tag);
    fprintf(tracefile, ",\"olddirection\":%d", ceiling->olddirection);
    fprintf(tracefile, "}");
}

static void Trace_WriteThinkers(void)
{
    thinker_t* thinker;
    int firstmobj;
    int firstspecial;
    int mobjs;
    int specials;

    firstmobj = 1;
    firstspecial = 1;
    mobjs = 0;
    specials = 0;

    fprintf(tracefile, ",\"mobjs\":[");
    for (thinker = thinkercap.next; thinker != &thinkercap; thinker = thinker->next)
    {
        if (thinker->function.acp1 != (actionf_p1)P_MobjThinker)
            continue;

        if (!firstmobj)
            fputc(',', tracefile);
        Trace_WriteMobj((mobj_t*)thinker);
        firstmobj = 0;
        mobjs++;
    }
    fprintf(tracefile, "]");

    fprintf(tracefile, ",\"specials\":[");
    for (thinker = thinkercap.next; thinker != &thinkercap; thinker = thinker->next)
    {
        if (thinker->function.acp1 == (actionf_p1)T_VerticalDoor)
        {
            if (!firstspecial)
                fputc(',', tracefile);
            Trace_WriteDoor((vldoor_t*)thinker);
            firstspecial = 0;
            specials++;
        }
        else if (thinker->function.acp1 == (actionf_p1)T_MoveFloor)
        {
            if (!firstspecial)
                fputc(',', tracefile);
            Trace_WriteFloor((floormove_t*)thinker);
            firstspecial = 0;
            specials++;
        }
        else if (thinker->function.acp1 == (actionf_p1)T_PlatRaise)
        {
            if (!firstspecial)
                fputc(',', tracefile);
            Trace_WritePlat((plat_t*)thinker);
            firstspecial = 0;
            specials++;
        }
        else if (thinker->function.acp1 == (actionf_p1)T_MoveCeiling)
        {
            if (!firstspecial)
                fputc(',', tracefile);
            Trace_WriteCeiling((ceiling_t*)thinker);
            firstspecial = 0;
            specials++;
        }
    }
    fprintf(tracefile, "]");
    fprintf(tracefile, ",\"mobj_count\":%d", mobjs);
    fprintf(tracefile, ",\"special_count\":%d", specials);
}

void Trace_Open(char* path)
{
    if (!path)
        path = "doom-trace.jsonl";

    tracefile = fopen(path, "w");
    if (!tracefile)
        I_Error("Trace_Open: couldn't open %s", path);

    tracepath = path;
    startupwritten = false;
    demowritten = false;
}

void Trace_SetPendingDemo(char* name)
{
    pendingdemoname = name;
}

boolean Trace_Enabled(void)
{
    return tracefile != NULL;
}

boolean Trace_Headless(void)
{
    return Trace_Enabled();
}

void Trace_WriteStartupMetadata(void)
{
    char* iwad;

    if (!tracefile || startupwritten)
        return;

    iwad = W_SelectedIWADPath();

    fprintf(tracefile, "{\"kind\":\"meta\"");
    fprintf(tracefile, ",\"trace_path\":");
    Trace_WriteJSONString(tracepath);
    fprintf(tracefile, ",\"iwad\":");
    Trace_WriteJSONString(iwad);
    fprintf(tracefile, ",\"demo\":");
    Trace_WriteJSONString(pendingdemoname);
    fprintf(tracefile, ",\"gamemode\":%d", gamemode);
    fprintf(tracefile, "}\n");
    fflush(tracefile);
    startupwritten = true;
}

void Trace_WriteDemoMetadata
( char* demo_name,
  int version,
  int skill,
  int episode,
  int map,
  int deathmatch,
  int respawn,
  int fast,
  int nomonsters,
  int consoleplayer,
  boolean* playeringame )
{
    if (!tracefile || demowritten)
        return;

    fprintf(tracefile, "{\"kind\":\"demo\"");
    fprintf(tracefile, ",\"demo\":");
    Trace_WriteJSONString(demo_name);
    fprintf(tracefile, ",\"version\":%d", version);
    fprintf(tracefile, ",\"skill\":%d", skill);
    fprintf(tracefile, ",\"episode\":%d", episode);
    fprintf(tracefile, ",\"map\":%d", map);
    fprintf(tracefile, ",\"deathmatch\":%d", deathmatch);
    fprintf(tracefile, ",\"respawn\":%d", respawn);
    fprintf(tracefile, ",\"fast\":%d", fast);
    fprintf(tracefile, ",\"nomonsters\":%d", nomonsters);
    fprintf(tracefile, ",\"consoleplayer\":%d", consoleplayer);
    fprintf(tracefile, ",\"playeringame\":[%d,%d,%d,%d]",
            playeringame[0], playeringame[1], playeringame[2], playeringame[3]);
    fprintf(tracefile, "}\n");
    fflush(tracefile);
    demowritten = true;
}

void Trace_WriteTic(void)
{
    player_t* player;
    mobj_t* mo;

    if (!tracefile)
        return;

    player = &players[consoleplayer];
    mo = player->mo;

    fprintf(tracefile, "{\"kind\":\"tic\"");
    fprintf(tracefile, ",\"gametic\":%d", gametic);
    fprintf(tracefile, ",\"rndindex\":%d", rndindex);
    fprintf(tracefile, ",\"prndindex\":%d", prndindex);
    fprintf(tracefile, ",\"gamestate\":%d", gamestate);
    fprintf(tracefile, ",\"gamestate_name\":\"%s\"", Trace_GameStateName(gamestate));
    fprintf(tracefile, ",\"gameaction\":%d", gameaction);
    fprintf(tracefile, ",\"gameaction_name\":\"%s\"", Trace_GameActionName(gameaction));
    fprintf(tracefile, ",\"leveltime\":%d", leveltime);
    fprintf(tracefile, ",\"consoleplayer\":%d", consoleplayer);
    fprintf(tracefile, ",\"displayplayer\":%d", displayplayer);
    fprintf(tracefile, ",\"playeringame\":[%d,%d,%d,%d]",
            playeringame[0], playeringame[1], playeringame[2], playeringame[3]);
    fprintf(tracefile, ",\"player\":{\"playerstate\":%d", player->playerstate);
    fprintf(tracefile, ",\"health\":%d", player->health);
    fprintf(tracefile, ",\"armorpoints\":%d", player->armorpoints);
    fprintf(tracefile, ",\"armortype\":%d", player->armortype);
    fprintf(tracefile, ",\"readyweapon\":%d", player->readyweapon);
    fprintf(tracefile, ",\"pendingweapon\":%d", player->pendingweapon);
    fprintf(tracefile, ",\"mo\":%d", mo != NULL);
    if (mo)
    {
        fprintf(tracefile, ",\"x\":%d", mo->x);
        fprintf(tracefile, ",\"y\":%d", mo->y);
        fprintf(tracefile, ",\"z\":%d", mo->z);
        fprintf(tracefile, ",\"angle\":%u", mo->angle);
        fprintf(tracefile, ",\"momx\":%d", mo->momx);
        fprintf(tracefile, ",\"momy\":%d", mo->momy);
        fprintf(tracefile, ",\"momz\":%d", mo->momz);
        fprintf(tracefile, ",\"mo_health\":%d", mo->health);
    }
    fprintf(tracefile, "}");
    Trace_WriteThinkers();
    fprintf(tracefile, "}\n");
}

void Trace_Close(void)
{
    if (!tracefile)
        return;

    fclose(tracefile);
    tracefile = NULL;
    tracepath = NULL;
    pendingdemoname = NULL;
    startupwritten = false;
    demowritten = false;
}
