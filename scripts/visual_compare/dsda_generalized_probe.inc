/* Generalized movers: real dispatchers, ordered mover ticks and live bodies.
 * No AI/horizontal ticks; trigger approach/aim is outside this state matrix. */
void P_CrossCompatibleSpecialLine(line_t *, int, mobj_t *, dboolean);
void P_ShootCompatibleSpecialLine(mobj_t *, line_t *);
dboolean P_ThingHeightClip(mobj_t *);
static void GD_GeneralizedProbe(void)
{
 const char *inpath=getenv("GD_DSDA_GEN_CASES"),*outpath=getenv("GD_DSDA_GEN_OUT");
 FILE *input,*out;char name[128],flats[3][16];int special,actor,activator,side,tag,busy,secret,position,target,tics,retry,late,stop,resume,retag,fs[3],cs[3];
 if(!inpath)return;
 input=fopen(inpath,"r");out=outpath?fopen(outpath,"w"):NULL;if(!input||!out)I_Error("generalized probe files");
 fprintf(out,"name,tic,remaining");
 {int i;for(i=0;i<3;++i)fprintf(out,",f%d,c%d,lock%d,prev%d,next%d,fa%d,ca%d,fs%d,fd%d,dir%d,cs%d,cd%d,orig%d,silent%d",i,i,i,i,i,i,i,i,i,i,i,i,i,i);}
 fprintf(out,",z,health,switch,button\n");
 while(fscanf(input,"%127s %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %15s %15s %15s",name,&special,&actor,&activator,&side,&tag,&busy,&secret,&position,&target,&tics,&retry,&late,&stop,&resume,&retag,&fs[0],&fs[1],&fs[2],&cs[0],&cs[1],&cs[2],flats[0],flats[1],flats[2])==25){
  player_t *p;mobj_t *imp=NULL,*body;thinker_t *th=NULL;line_t *line;int tic,i;
  G_InitNew(2,1,1,true);gametic=1;menuactive=false;p=&players[displayplayer];p->cheats=0;line=&lines[2];
  while((th=P_NextThinker(th,th_all)))if(th->function==P_MobjThinker&&((mobj_t*)th)->type==MT_TROOP)imp=(mobj_t*)th;
  if(!imp)I_Error("generalized probe requires actual imp");
  for(i=0;i<3;++i){sectors[i].floorheight=fs[i];sectors[i].ceilingheight=cs[i];sectors[i].floorpic=R_FlatNumForName(flats[i]);sectors[i].special=0;sectors[i].tag=i==target?7:0;}
  dsda_ResetSectorIDList(numsectors);for(i=0;i<numsectors;++i)dsda_AddSectorID(sectors[i].tag,i);
  line->special=special;line->tag=tag;if(secret)line->flags|=ML_SECRET;
  for(i=0;i<2;++i){mobj_t *mo=i?imp:p->mo;int selected=actor==(i?2:1);int x=selected?0:-1024;if(!selected)mo->flags&=~MF_SHOOTABLE;if(selected&&position)x=position==1?-120:120;
   P_UnsetThingPosition(mo);mo->x=x*FRACUNIT;mo->y=(i?48:-48)*FRACUNIT;P_SetThingPosition(mo);mo->floorz=mo->subsector->sector->floorheight;mo->ceilingz=mo->subsector->sector->ceilingheight;mo->z=mo->floorz;mo->momx=mo->momy=mo->momz=0;if(selected&&position)P_ThingHeightClip(mo);
  }P_MapEnd();body=actor==2?imp:p->mo;
  if(busy){int old=line->tag,sec=busy==4?1:target;sectors[sec].tag=9;dsda_ResetSectorIDList(numsectors);for(i=0;i<numsectors;++i)dsda_AddSectorID(sectors[i].tag,i);line->tag=9;
   if(busy==1||busy==4)EV_DoFloor(line,raiseFloor24);if(busy==4)((floormove_t*)sectors[sec].floordata)->speed=FRACUNIT/4;if(busy==2)EV_DoCeiling(line,lowerToFloor);if(busy==3)EV_DoElevator(line,elevateUp);
   line->tag=old;sectors[sec].tag=sec==target?7:0;dsda_ResetSectorIDList(numsectors);for(i=0;i<numsectors;++i)dsda_AddSectorID(sectors[i].tag,i);
  }
  for(tic=0;tic<=tics;++tic){
   if(tic==stop){int old=line->tag;line->tag=7;if(retag){sectors[target].tag=9;dsda_ResetSectorIDList(numsectors);for(i=0;i<numsectors;++i)dsda_AddSectorID(sectors[i].tag,i);}EV_CeilingCrushStop(line);line->tag=old;}
   if(tic==resume){int oldspecial=line->special,oldtag=line->tag;line->special=(special&~7)|3;line->tag=7;EV_DoGenCrusher(line);line->special=oldspecial;line->tag=oldtag;}
   if(tic==0||tic==1||tic==retry||tic==late){if(line->special){int trigger=line->special&7;
    if(trigger<2)P_CrossCompatibleSpecialLine(line,side,activator?imp:p->mo,false);
    else if(trigger==4||trigger==5)P_ShootCompatibleSpecialLine(activator?imp:p->mo,line);
    else if(activator||p->health>0)P_UseSpecialLine(activator?imp:p->mo,line,side,false);
   }}
   if(tic){leveltime=tic-1;th=NULL;while((th=P_NextThinker(th,th_all)))if(th->function==(think_t)T_MoveFloor||th->function==(think_t)T_MoveCeiling||th->function==(think_t)T_PlatRaise||th->function==(think_t)T_MoveElevator)th->function(th);P_UpdateSpecials();}
   fprintf(out,"%s,%d,%d",name,tic,line->special);
   for(i=0;i<3;++i){floormove_t *f=NULL;ceiling_t *c=NULL;
    if(sectors[i].floordata&&((thinker_t*)sectors[i].floordata)->function==(think_t)T_MoveFloor)f=sectors[i].floordata;
    if(sectors[i].ceilingdata && ((thinker_t*)sectors[i].ceilingdata)->function!=(think_t)T_MoveElevator)c=sectors[i].ceilingdata;
    fprintf(out,",%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d",sectors[i].floorheight,sectors[i].ceilingheight,sectors[i].stairlock,sectors[i].prevsec,sectors[i].nextsec,sectors[i].floordata!=NULL,sectors[i].ceilingdata!=NULL,f?f->speed:0,f?f->floordestheight:0,f?f->direction:0,c?c->speed:0,c?c->direction:0,c?c->oldspeed:0,c?c->silent:0);
   }
   {int timer=0;for(i=0;i<MAXBUTTONS;++i)if(buttonlist[i].btimer)timer=buttonlist[i].btimer;fprintf(out,",%d,%d,%d,%d\n",actor?body->z:0,actor?body->health:0,sides[line->sidenum[0]].toptexture==R_TextureNumForName("SW2BRCOM"),timer);}
  }P_MapEnd();
 }
 if(!feof(input))I_Error("invalid generalized manifest");
 fclose(input);fclose(out);exit(0);
}
