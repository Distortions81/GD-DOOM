/* Observe completed real G_Ticker demo tics. Gameplay code is unchanged. */
static void GD_RNGDemoFrame(void)
{
 static FILE *out;static int last_gametic=-1;
 const char *path=getenv("GD_DSDA_RNG_DEMO_OUT");thinker_t *th=NULL;int i;
 if(!path||!demoplayback||gamestate!=GS_LEVEL||leveltime<=0||last_gametic==gametic)return;
 last_gametic=gametic;
 if(!out){out=fopen(path,"w");if(!out)I_Error("RNG demo output");fprintf(out,"tic,x,y,z,angle,momx,momy,momz,health,armor,ammo,viewz,playindex,mobjs,seeds\n");}
 {player_t *p=&players[displayplayer];mobj_t *m=p->mo;
 fprintf(out,"%d,%d,%d,%d,%u,%d,%d,%d,%d,%d,%d,%d,%d,",leveltime-1,m->x,m->y,m->z,m->angle,m->momx,m->momy,m->momz,p->health,p->armorpoints[0],p->ammo[am_clip],p->viewz,rng.rndindex);}
 while((th=P_NextThinker(th,th_all)))if(th->function==P_MobjThinker){mobj_t *m=(mobj_t*)th;if(m->player || (m->flags&MF_SPECIAL))continue;
  fprintf(out,"%d:%d:%d:%d:%u:%d:%d:%d:%d:%d:%ld;",m->type,m->x,m->y,m->z,m->angle,m->momx,m->momy,m->momz,m->health,m->tics,(long)(m->state-states));}
 fputc(',',out);for(i=0;i<NUMPRCLASS;++i)if(i!=pr_misc)fprintf(out,"%u;",rng.seed[i]);fputc('\n',out);fflush(out);
}
