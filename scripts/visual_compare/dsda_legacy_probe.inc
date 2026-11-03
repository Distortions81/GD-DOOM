/* Original fixture inputs plus unmodified legacy dispatch and mover functions.
 * Actor AI and horizontal movement are deliberately not stepped here. */
#include "dsda/id_list.h"
dboolean P_ThingHeightClip(mobj_t *);
void P_SpawnCompatibleSectorSpecial(sector_t *, int);
void P_CrossCompatibleSpecialLine(line_t *, int, mobj_t *, dboolean);
static void GD_LegacyProbe(void)
{
 const char *input_path=getenv("GD_DSDA_LEGACY_CASES"),*output_path=getenv("GD_DSDA_LEGACY_OUT");
 FILE *input,*out;char name[128];int map,special,use,actor,activator,side,position,tag,busy,target,tics,retry,fs[3],cs[3],types[3];
 if(!input_path)return;
 input=fopen(input_path,"r");out=output_path?fopen(output_path,"w"):NULL;if(!input||!out)I_Error("legacy probe files");
 fprintf(out,"name,tic,remaining,f0,c0,flat0,type0,flags0,f1,c1,flat1,type1,flags1,f2,c2,flat2,type2,flags2,z,health,factive,cactive,switch,button\n");
 while(fscanf(input,"%127s %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d",name,&map,&special,&use,&actor,&activator,&side,&position,&tag,&busy,&target,&tics,&retry,&fs[0],&fs[1],&fs[2],&cs[0],&cs[1],&cs[2],&types[0],&types[1],&types[2])==22){
  int tic,i;thinker_t *th=NULL;player_t *p;mobj_t *imp=NULL,*body;line_t *line;
  G_InitNew(2,1,map,true);gametic=1;menuactive=false;p=&players[displayplayer];p->cheats=0;line=&lines[2];
  while((th=P_NextThinker(th,th_all)))if(th->function==P_MobjThinker&&((mobj_t*)th)->type==MT_TROOP)imp=(mobj_t*)th;
  if(!imp)I_Error("legacy probe requires real imp");
  for(i=0;i<3;++i){sectors[i].floorheight=fs[i];sectors[i].ceilingheight=cs[i];sectors[i].special=types[i];sectors[i].tag=i==target?7:0;P_SpawnCompatibleSectorSpecial(&sectors[i],i);}
  /* Tags are cached at map setup; rebuild after selecting the donut pillar. */
  dsda_ResetSectorIDList(numsectors);for(i=0;i<numsectors;++i)dsda_AddSectorID(sectors[i].tag,i);
  line->special=special;line->tag=tag;
  for(i=0;i<2;++i){mobj_t *mo=i?imp:p->mo;int selected=actor==(i?2:1);int x=selected?(target==0?-256:0):(target==0?320:-320);
   if(selected&&position)x=position==1?-120:120;
   P_UnsetThingPosition(mo);mo->x=x*FRACUNIT;mo->y=(i?48:-48)*FRACUNIT;P_SetThingPosition(mo);mo->floorz=mo->subsector->sector->floorheight;mo->ceilingz=mo->subsector->sector->ceilingheight;mo->z=mo->floorz;mo->momx=mo->momy=mo->momz=0;if(selected&&position)P_ThingHeightClip(mo);
  }
  P_MapEnd();
  if(busy){int old=line->tag;line->tag=7;if(busy==1)EV_DoFloor(line,lowerFloorToNearest);if(busy==2)EV_DoCeiling(line,lowerToFloor);if(busy==3)EV_DoElevator(line,elevateUp);line->tag=old;}
  body=actor==2?imp:p->mo;
  for(tic=0;tic<=tics;++tic){
   if(tic==0||tic==1||tic==retry){if(line->special){if(use){if(activator||p->health>0)P_UseSpecialLine(activator?imp:p->mo,line,side,false);}else P_CrossCompatibleSpecialLine(line,side,activator?imp:p->mo,false);}}
   if(tic){leveltime=tic-1;th=NULL;while((th=P_NextThinker(th,th_all))){
    if(th->function==(think_t)T_MoveFloor||th->function==(think_t)T_MoveCeiling||th->function==(think_t)T_PlatRaise||th->function==(think_t)T_MoveElevator||th->function==(think_t)T_VerticalDoor)th->function(th);
   }P_UpdateSpecials();}
   fprintf(out,"%s,%d,%d",name,tic,line->special);
   for(i=0;i<3;++i)fprintf(out,",%d,%d,%.8s,%d,%d",sectors[i].floorheight,sectors[i].ceilingheight,W_LumpName(firstflat+sectors[i].floorpic),sectors[i].special,(sectors[i].flags&SECF_SECRET?128:0)|(sectors[i].flags&SECF_FRICTION?256:0)|(sectors[i].flags&SECF_PUSH?512:0));
   {int timer=0;for(i=0;i<MAXBUTTONS;++i)if(buttonlist[i].btimer)timer=buttonlist[i].btimer;
    fprintf(out,",%d,%d,%d,%d,%d,%d\n",actor?body->z:0,actor?body->health:0,sectors[target].floordata!=NULL,sectors[target].ceilingdata!=NULL,sides[line->sidenum[0]].toptexture==R_TextureNumForName("SW2BRCOM"),timer);}
  }
  P_MapEnd();
 }
 if(!feof(input))I_Error("bad legacy case file");
 fclose(input);fclose(out);exit(0);
}
