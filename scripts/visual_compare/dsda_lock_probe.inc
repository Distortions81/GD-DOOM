/* Exercise the unmodified Boom lock predicate and real trigger dispatchers.
 * Actors and keys are fixture inputs; no gameplay function is replaced. */
void P_CrossCompatibleSpecialLine(line_t *, int, mobj_t *, dboolean);
void P_ShootCompatibleSpecialLine(mobj_t *, line_t *);
const char *GD_LockMessage(void);
void GD_ClearLockMessage(void);
static void GD_LockProbe(void)
{
 const char *path=getenv("GD_DSDA_LOCK_OUT"); FILE *out;
 int special,mask,monster,phase,i; player_t *p; mobj_t *imp=NULL; thinker_t *th=NULL;
 if(!path)return;
 out=fopen(path,"w");if(!out)I_Error("lock output");
 G_InitNew(2,1,1,true);gametic=1;menuactive=false;p=&players[displayplayer];p->cheats=0;
 while((th=P_NextThinker(th,th_all)))if(th->function==P_MobjThinker&&((mobj_t*)th)->type==MT_TROOP)imp=(mobj_t*)th;
 if(!imp)I_Error("lock probe requires actual monster");
 fprintf(out,"special,mask,monster,phase,allowed,active,remaining,speed,wait,top,direction,kind,message\n");
 for(special=0x3800;special<=0x3bff;++special)for(mask=0;mask<64;++mask)for(monster=0;monster<2;++monster){
  line_t *line=&lines[2];vldoor_t *door;int allowed;
  if(sectors[1].ceilingdata){P_RemoveThinker(&((vldoor_t*)sectors[1].ceilingdata)->thinker);sectors[1].ceilingdata=NULL;}
  memset(buttonlist,0,sizeof(buttonlist));line->special=special;
  for(i=0;i<6;++i)p->cards[i]=(mask>>i)&1;
  allowed=P_CanUnlockGenDoor(line,p);
  for(phase=0;phase<2;++phase){
   GD_ClearLockMessage();
   if(line->special){
    if((special&7)<2)P_CrossCompatibleSpecialLine(line,0,monster?imp:p->mo,false);
    else if((special&7)==4||(special&7)==5)P_ShootCompatibleSpecialLine(monster?imp:p->mo,line);
    else P_UseSpecialLine(monster?imp:p->mo,line,0,false);
   }
   door=sectors[1].ceilingdata;
   fprintf(out,"%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%s\n",special,mask,monster,phase,allowed,door!=NULL,line->special,door?door->speed:0,door?door->topwait:0,door?door->topheight:0,door?door->direction:0,door?(door->type==genOpen||door->type==genBlazeOpen):0,GD_LockMessage());
  }
 }
 fclose(out);exit(0);
}
