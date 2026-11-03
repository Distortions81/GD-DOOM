/* Read-only mover identity registry preserves stasis and displaced thinkers. */
static thinker_t *GD_ManualKnown[2048];
static int GD_ManualKinds[2048],GD_ManualCount;
static int GD_ManualKind(thinker_t *th)
{
 int kind=0,i;
 if(th->function==(think_t)T_VerticalDoor)kind=1;
 if(th->function==(think_t)T_MoveCeiling)kind=2;
 if(th->function==(think_t)T_MoveFloor)kind=3;
 if(th->function==(think_t)T_PlatRaise)kind=4;
 if(th->function==(think_t)T_MoveElevator)kind=5;
 for(i=0;i<GD_ManualCount;++i)if(GD_ManualKnown[i]==th){if(kind)GD_ManualKinds[i]=kind;return kind?kind:th->function?0:GD_ManualKinds[i];}
 if(kind){if(GD_ManualCount>=2048)I_Error("manual registry overflow");GD_ManualKnown[GD_ManualCount]=th;GD_ManualKinds[GD_ManualCount++]=kind;}
 return kind;
}
static void GD_ManualRemember(void) {thinker_t *th=NULL;while((th=P_NextThinker(th,th_all)))GD_ManualKind(th);}
static void GD_ManualTick(void)
{
 thinker_t *th=NULL;while((th=P_NextThinker(th,th_all))){int k=GD_ManualKind(th);if(k&&th->function)th->function(th);}P_UpdateSpecials();
}
static int GD_ManualDoorKind(vldoor_t *d)
{
 switch(d->type){case normal:case blazeRaise:case genRaise:case genBlazeRaise:return 0;case close30ThenOpen:case genCdO:case genBlazeCdO:return 2;case closeDoor:case blazeClose:case genClose:case genBlazeClose:return 3;default:return 1;}
}
static void GD_ManualProbe(void)
{
 const char *inpath=getenv("GD_DSDA_MANUAL_CASES"),*outpath=getenv("GD_DSDA_MANUAL_OUT");FILE *input,*out;char name[128];
 int special,second,kind,age,actor,activator,side,secret,keys,tics,at2,at3,late,fs[3],cs[3];
 if(!inpath)return;input=fopen(inpath,"r");out=outpath?fopen(outpath,"w"):NULL;if(!input||!out)I_Error("manual probe files");
 fprintf(out,"name,tic,line1,line2,f0,c0,f1,c1,f2,c2,fowner,cowner,doors,ceilings,floors,plats,elevators,z,health,lock0,lock1,lock2\n");
 while(fscanf(input,"%127s %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d",name,&special,&second,&kind,&age,&actor,&activator,&side,&secret,&keys,&tics,&at2,&at3,&late,&fs[0],&fs[1],&fs[2],&cs[0],&cs[1],&cs[2])==20){
  player_t *p;mobj_t *imp=NULL,*body;thinker_t *th=NULL;int i,tic,old;line_t *line;
  rngseed=!strncmp(name,"rng1_lift_",10)?1:1993;demo_insurance=0;G_InitNew(2,1,1,true);GD_SectorSoundBegin();gametic=1;menuactive=false;p=&players[displayplayer];p->cheats=0;GD_ManualCount=0;line=&lines[2];
  while((th=P_NextThinker(th,th_all)))if(th->function==P_MobjThinker&&((mobj_t*)th)->type==MT_TROOP)imp=(mobj_t*)th;
  if(!imp)I_Error("manual probe needs actual imp");
  for(i=0;i<3;++i){sectors[i].floorheight=fs[i];sectors[i].ceilingheight=cs[i];sectors[i].floorpic=R_FlatNumForName("FLOOR0_1");sectors[i].tag=i==1?7:0;}
  dsda_ResetSectorIDList(numsectors);for(i=0;i<numsectors;++i)dsda_AddSectorID(sectors[i].tag,i);
  for(i=0;i<6;++i)p->cards[i]=keys;
  for(i=0;i<2;++i){mobj_t *mo=i?imp:p->mo;int selected=actor==(i?2:1);if(!selected)mo->flags&=~MF_SHOOTABLE;
   P_UnsetThingPosition(mo);mo->x=(selected?0:-1024)*FRACUNIT;mo->y=(i?48:-48)*FRACUNIT;P_SetThingPosition(mo);mo->floorz=mo->subsector->sector->floorheight;mo->ceilingz=mo->subsector->sector->ceilingheight;mo->z=mo->floorz;mo->momx=mo->momy=mo->momz=0;
  }P_MapEnd();body=actor==2?imp:p->mo;
  old=line->special;
  if(kind>=1&&kind<=4)EV_DoPlat(line,downWaitUpStay,0);
  if(kind==5)EV_DoElevator(line,elevateUp);if(kind==6)EV_DoElevator(line,elevateDown);
  if(kind>=7&&kind<=9){line->special=kind==8?0x2f83:0x2f93;EV_DoGenCrusher(line);}
  if(kind==10||kind==11){line->special=0x3103;EV_DoGenStairs(line);}
  if(kind==12)EV_DoFloor(line,lowerFloorToNearest);if(kind==13)EV_DoCeiling(line,lowerToFloor);
  if(kind==14||kind==16)EV_DoDoor(line,normal);if(kind==15)EV_DoDoor(line,closeDoor);if(kind==17)EV_DoDoor(line,openDoor);
  if(kind==18){line->special=0x3c07;EV_DoGenDoor(line);}
  GD_ManualRemember();if(kind==4)EV_StopPlat(line);if(kind==9)EV_CeilingCrushStop(line);
  for(i=0;i<age;++i){leveltime=i;GD_ManualTick();}
  line->special=special;lines[10].special=second;if(secret){line->flags|=ML_SECRET;lines[10].flags|=ML_SECRET;}leveltime=age;
  for(tic=0;tic<=tics;++tic){
   if(tic==late){old=line->special;line->special=0x2f93;EV_DoGenCrusher(line);line->special=old;}
   if(tic==0&&(!(!activator&&p->health<=0)))P_UseSpecialLine(activator?imp:p->mo,line,side,false);
   if((tic==at2||tic==at3)&&lines[10].special&&(activator||p->health>0))P_UseSpecialLine(activator?imp:p->mo,&lines[10],side,false);
   if(tic){leveltime=age+tic-1;GD_ManualTick();}
   GD_SectorSoundFrame(name,tic);
   fprintf(out,"%s,%d,%d,%d",name,tic,line->special,lines[10].special);for(i=0;i<3;++i)fprintf(out,",%d,%d",sectors[i].floorheight,sectors[i].ceilingheight);
   fprintf(out,",%d,%d",sectors[1].floordata?GD_ManualKind((thinker_t*)sectors[1].floordata):0,sectors[1].ceilingdata?GD_ManualKind((thinker_t*)sectors[1].ceilingdata):0);
   for(i=1;i<=5;++i){th=NULL;fputc(',',out);while((th=P_NextThinker(th,th_all))){int k=GD_ManualKind(th);if(k!=i)continue;
    if(k==1){vldoor_t *d=(vldoor_t*)th;fprintf(out,"%d:%d:%d:%d:%d:%d:%d;",d->sector->iSectorID,GD_ManualDoorKind(d),d->direction,d->speed,d->topheight,d->topwait,d->topcountdown);}
    if(k==2){ceiling_t *c=(ceiling_t*)th;fprintf(out,"%d:%d:%d:%d:%d:%d;",c->sector->iSectorID,c->direction,c->speed,c->topheight,c->bottomheight,c->oldspeed);}
    if(k==3){floormove_t *f=(floormove_t*)th;fprintf(out,"%d:%d:%d:%d;",f->sector->iSectorID,f->direction,f->speed,f->floordestheight);}
    if(k==4){plat_t *p=(plat_t*)th;fprintf(out,"%d:%d:%d:%d:%d:%d:%d:%d;",p->sector->iSectorID,p->status,p->speed,p->low,p->high,p->wait,p->count,p->status==in_stasis?p->oldstatus:0);}
    if(k==5){elevator_t *e=(elevator_t*)th;fprintf(out,"%d:%d:%d:%d:%d;",e->sector->iSectorID,e->direction,e->speed,e->floordestheight,e->ceilingdestheight);}
   }}
   fprintf(out,",%d,%d,%d,%d,%d\n",actor?body->z:0,actor?body->health:0,sectors[0].stairlock,sectors[1].stairlock,sectors[2].stairlock);
  }P_MapEnd();
 }
 if(!feof(input))I_Error("bad manual manifest");fclose(input);fclose(out);exit(0);
}
