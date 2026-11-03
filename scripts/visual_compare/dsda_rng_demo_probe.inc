/* Observe completed real G_Ticker demo tics. Gameplay code is unchanged. */
static void GD_RNGDemoFrame(void)
{
 static FILE *out;static int last_gametic=-1;
 const char *path=getenv("GD_DSDA_RNG_DEMO_OUT");thinker_t *th=NULL;int i;
 if(!path||!demoplayback||gamestate!=GS_LEVEL||leveltime<=0||last_gametic==gametic)return;
 last_gametic=gametic;
 /* Controlled initial corpse, not a replacement resurrection implementation. */
 if(leveltime==1 && getenv("GD_RNG_CRUSHED_CORPSE")) {
  thinker_t *it=NULL;
  while((it=P_NextThinker(it,th_all)))if(it->function==P_MobjThinker){
   mobj_t *corpse=(mobj_t*)it;
   if(corpse->type==MT_SHOTGUY){
    P_DamageMobj(corpse,NULL,players[displayplayer].mo,200);
    P_SetMobjState(corpse,S_GIBS);corpse->flags&=~MF_SOLID;
    corpse->height=corpse->radius=0;
    P_MapEnd();
   }
  }
 }
 if(!out){out=fopen(path,"w");if(!out)I_Error("RNG demo output");fprintf(out,"tic,x,y,z,angle,momx,momy,momz,health,armor,ammo,viewz,playindex,mobjs,seeds\n");}
 {player_t *p=&players[displayplayer];mobj_t *m=p->mo;
 fprintf(out,"%d,%d,%d,%d,%u,%d,%d,%d,%d,%d,%d,%d,%d,",leveltime-1,m->x,m->y,m->z,m->angle,m->momx,m->momy,m->momz,p->health,p->armorpoints[0],p->ammo[am_clip],p->viewz,rng.rndindex);}
 while((th=P_NextThinker(th,th_all)))if(th->function==P_MobjThinker){mobj_t *m=(mobj_t*)th;if(m->player || (m->flags&MF_SPECIAL))continue;
 if(getenv("GD_RNG_COMBAT_ONLY") && !(m->flags&(MF_COUNTKILL|MF_MISSILE)) && m->type!=MT_TRACER && m->type!=MT_FATSHOT && m->type!=MT_BRUISERSHOT && m->type!=MT_TROOPSHOT && m->type!=MT_HEADSHOT && m->type!=MT_ROCKET && m->type!=MT_PLASMA && m->type!=MT_BFG && m->type!=MT_ARACHPLAZ && m->type!=MT_EXTRABFG && m->type!=MT_SKULL && m->type!=MT_PUFF && m->type!=MT_BLOOD && m->type!=MT_SMOKE && m->type!=MT_FIRE && m->type!=MT_TFOG && m->type!=MT_IFOG)continue;
  fprintf(out,"%d:%d:%d:%d:%u:%d:%d:%d:%d:%d:%ld",m->type,m->x,m->y,m->z,m->angle,m->momx,m->momy,m->momz,m->health,m->tics,(long)(m->state-states));
  if(getenv("GD_RNG_COMBAT_ONLY"))fprintf(out,":%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d",m->target!=NULL,m->target?m->target->type:0,m->tracer!=NULL,m->tracer?m->tracer->type:0,m->movedir,m->movecount,m->reactiontime,m->threshold,m->lastlook,m->radius,m->height,m->floorz,m->ceilingz);
  fputc(';',out);}
 fputc(',',out);for(i=0;i<NUMPRCLASS;++i)if(i!=pr_misc)fprintf(out,"%u;",rng.seed[i]);fputc('\n',out);fflush(out);
}
