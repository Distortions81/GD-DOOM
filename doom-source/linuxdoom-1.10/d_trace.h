// Emacs style mode select   -*- C++ -*-
//-----------------------------------------------------------------------------
//
// Doom trace output helpers.
//
//-----------------------------------------------------------------------------

#ifndef __D_TRACE__
#define __D_TRACE__

#include "doomtype.h"

void Trace_Open(char* path);
void Trace_SetPendingDemo(char* name);
boolean Trace_Enabled(void);
boolean Trace_Headless(void);
void Trace_WriteStartupMetadata(void);
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
  boolean* playeringame );
void Trace_WriteTic(void);
void Trace_Close(void);

#endif
