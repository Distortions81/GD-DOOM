/* Actor setup and integer-state export only. Reference movement, crossing and
 * teleport routines are used without changes. */
static void GD_TeleportProbe(void)
{
 const char *input_path=getenv("GD_DSDA_TELEPORT_CASES"), *out_path=getenv("GD_DSDA_TELEPORT_OUT");
 FILE *input,*out;char name[128],actor[16];int map,source,destination,special,back,blocked,missing,z,y,mx,my,mz,ticks,startx,nextx,nexty;
 unsigned angle;
 if(!input_path) return;
 input=fopen(input_path,"r");out=fopen(out_path,"w");if(!input||!out) I_Error("teleport probe files");
 fprintf(out,"name,moved,x,y,z,floor,ceil,angle,momx,momy,momz,reaction,special,blocker_alive,fogs\n");
 while(fscanf(input,"%127s %d %15s %d %d %d %d %d %d %d %d %d %d %d %u %d %d %d %d",name,&map,actor,&source,&destination,&special,&back,&blocked,&missing,&z,&y,&mx,&my,&mz,&angle,&ticks,&startx,&nextx,&nexty)==19) {
  thinker_t *th=NULL;mobj_t *mover=NULL,*blocker=NULL,*unused=NULL;player_t *p;int x=startx,next=nextx,moved,i,fogs=0;
  G_InitNew(2,1,map,true);gametic=1;p=&players[displayplayer];
  while((th=P_NextThinker(th,th_all))!=NULL) if(th->function==P_MobjThinker) {
   mobj_t *m=(mobj_t*)th;
   if(m->index==1 && m->type==MT_TROOP) unused=m;
   if(m->index==2 && m->type==MT_TROOP) blocker=m;
  }
  if(!unused||!blocker) I_Error("teleport probe monsters absent (do not use -nomonsters)");
  lines[source].special=special;if(missing) lines[source].tag=999;
  if(!strcmp(actor,"player")||!strcmp(actor,"missile")) { P_UnsetThingPosition(unused);unused->x=-704*FRACUNIT;unused->y=0;P_SetThingPosition(unused); }
  if(blocked) {
   int bx=448*FRACUNIT,by=-y;
   if(special>=262&&special<=265) by=y;
   if((special>=207&&special<=210)||special>=268) { bx=640*FRACUNIT;by=128*FRACUNIT; }
   P_UnsetThingPosition(blocker);blocker->x=bx;blocker->y=by;P_SetThingPosition(blocker);
   blocker->z=blocker->floorz=blocker->subsector->sector->floorheight;
  }
  if(!strcmp(actor,"player")) mover=p->mo;
  else if(!strcmp(actor,"caco")) { P_RemoveMobj(unused);mover=P_SpawnMobj(-464*FRACUNIT,64*FRACUNIT,ONFLOORZ,MT_HEAD); }
  else if(!strcmp(actor,"missile")) mover=P_SpawnMobj(-449*FRACUNIT,y,z,MT_ARACHPLAZ);
  else mover=unused;
  if(back) { int temp=x;x=next;next=temp; }
  P_UnsetThingPosition(mover);mover->x=x;mover->y=y;P_SetThingPosition(mover);
  mover->z=z;mover->floorz=0;mover->ceilingz=256*FRACUNIT;mover->angle=angle;mover->momx=mx;mover->momy=my;mover->momz=mz;mover->reactiontime=7;
  if(special==209||special==210) { P_UseSpecialLine(mover,&lines[source],back,false);mover->tics=100000;for(i=0;i<ticks;++i) P_MobjThinker(mover);moved=1; }
  else if(ticks) { mover->tics=100000;for(i=0;i<ticks;++i) P_MobjThinker(mover);moved=1; }
  else moved=P_TryMove(mover,next,nexty,true);
  th=NULL;while((th=P_NextThinker(th,th_all))!=NULL) if(th->function==P_MobjThinker&&((mobj_t*)th)->type==MT_TFOG) ++fogs;
  fprintf(out,"%s,%d,%d,%d,%d,%d,%d,%u,%d,%d,%d,%d,%d,%d,%d\n",name,moved,mover->x,mover->y,mover->z,mover->floorz,mover->ceilingz,mover->angle,mover->momx,mover->momy,mover->momz,mover->reactiontime,lines[source].special,blocker->health>0,fogs);
  P_MapEnd();
 }
 if(!feof(input)) I_Error("malformed teleport manifest");
 fclose(input);fclose(out);exit(0);
}
