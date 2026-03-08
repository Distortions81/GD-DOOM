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
    fprintf(tracefile, "}}\n");
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
