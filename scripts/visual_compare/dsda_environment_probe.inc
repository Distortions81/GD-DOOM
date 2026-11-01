/* Friction/force validation with live map actors and unmodified simulation.
 * Only actor initial conditions, commands and sector enable flags are set. */
static void GD_EnvironmentProbe(void)
{
 const char *input_path=getenv("GD_DSDA_ENV_CASES"),*out_path=getenv("GD_DSDA_ENV_OUT");
 FILE *input,*out;char name[128],actor[16],mode[16];int map,x,y,z,mx,my,mz,forward,side,tics,noclip,disable,enable;
 if(!input_path)return;
 if(!out_path)I_Error("environment probe needs output path");
 input=fopen(input_path,"r");out=fopen(out_path,"w");if(!input||!out)I_Error("environment probe files");
 fprintf(out,"name,tic,x,y,z,momx,momy,momz,friction,movefactor\n");
 while(fscanf(input,"%127s %d %15s %15s %d %d %d %d %d %d %d %d %d %d %d %d",name,&map,actor,mode,&x,&y,&z,&mx,&my,&mz,&forward,&side,&tics,&noclip,&disable,&enable)==16) {
  thinker_t *th=NULL;mobj_t *mover=NULL,*monster=NULL;player_t *p;int i,tic;unsigned *flags;
  G_InitNew(2,1,map,true);gametic=1;menuactive=false;p=&players[displayplayer];p->cheats=0;
  while((th=P_NextThinker(th,th_all))!=NULL)if(th->function==P_MobjThinker&&((mobj_t*)th)->type==MT_TROOP&&((mobj_t*)th)->index==1)monster=(mobj_t*)th;
  if(!monster)I_Error("environment probe requires monsters enabled");
  if(!strcmp(actor,"player"))mover=p->mo;
  else if(!strcmp(actor,"caco")){P_RemoveMobj(monster);monster=P_SpawnMobj(x,y,z,MT_HEAD);mover=monster;}
  else mover=monster;
  monster->tics=100000;
  P_UnsetThingPosition(mover);mover->x=x;mover->y=y;P_SetThingPosition(mover);
  mover->z=z;mover->floorz=mover->subsector->sector->floorheight;mover->ceilingz=mover->subsector->sector->ceilingheight;mover->momx=mx;mover->momy=my;mover->momz=mz;
  p->viewheight=41*FRACUNIT;p->viewz=p->mo->z+p->viewheight;
  if(noclip){mover->flags|=MF_NOCLIP;if(mover==p->mo)p->cheats|=CF_NOCLIP;}
  flags=malloc(sizeof(unsigned)*numsectors);for(i=0;i<numsectors;++i){flags[i]=sectors[i].flags;if(enable>=0)sectors[i].flags&=~(SECF_FRICTION|SECF_PUSH);}
  P_MapEnd(); /* End actor placement before the first real P_MapStart. */
  for(tic=0;tic<=tics;++tic){
   if(tic){
    for(i=0;i<numsectors;++i){if(tic-1==disable)sectors[i].flags&=~(SECF_FRICTION|SECF_PUSH);if(tic-1==enable)sectors[i].flags=flags[i];}
    if(!strcmp(mode,"force")){
     th=NULL;while((th=P_NextThinker(th,th_all))!=NULL){if(th->function==(think_t)T_Friction||th->function==(think_t)T_Pusher)th->function(th);}
    }else{p->cmd.forwardmove=forward;p->cmd.sidemove=side;gametic=tic;P_Ticker();}
   }
   fprintf(out,"%s,%d,%d,%d,%d,%d,%d,%d,%d,%d\n",name,tic,mover->x,mover->y,mover->z,mover->momx,mover->momy,mover->momz,mover->friction,mover->movefactor);
  }
  free(flags);P_MapEnd();
 }
 if(!feof(input))I_Error("bad environment manifest");
 fclose(input);fclose(out);exit(0);
}
