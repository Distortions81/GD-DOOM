static void GD_RNGLifts(void)
{
 const char *path=getenv("GD_DSDA_RNG_LIFTS"),*output=getenv("GD_DSDA_RNG_OUT");FILE *in,*out;char name[128];unsigned seed;int insurance,special,draws,tics,stop,resume;
 if(!path)return;in=fopen(path,"r");out=output?fopen(output,"w"):NULL;if(!in||!out)I_Error("RNG lift files");
 fprintf(out,"name,tic,floor,status,count,low,high,playindex,seeds\n");
 while(fscanf(in,"%127s %u %d %d %d %d %d %d",name,&seed,&insurance,&special,&draws,&tics,&stop,&resume)==8){
  int tic,j;line_t *line;player_t *p;
  compatibility_level=boom_202_compatibility;demo_insurance=insurance;rngseed=seed;gametic=boom_basetic=0;G_InitNew(2,1,1,true);menuactive=false;
  line=&lines[2];line->special=special;line->tag=7;p=&players[0];
  for(j=0;j<draws;++j)P_Random(pr_see);
  for(tic=0;tic<=tics;++tic){
   if(tic==0||tic==resume){if(special==87)P_CrossCompatibleSpecialLine(line,0,p->mo,false);else P_UseSpecialLine(p->mo,line,0,false); }
   if(tic==stop)EV_StopPlat(line);
   if(tic){thinker_t *th=NULL;leveltime=tic-1;gametic=tic-1;while((th=P_NextThinker(th,th_all)))if(th->function==(think_t)T_PlatRaise)th->function(th);}
   {plat_t *plat=sectors[1].floordata;fprintf(out,"%s,%d,%d,%d,%d,%d,%d,%d,",name,tic,sectors[1].floorheight,plat?plat->status:-1,plat?plat->count:0,plat?plat->low:0,plat?plat->high:0,rng.rndindex);}
   for(j=0;j<NUMPRCLASS;++j)if(j!=pr_misc)fprintf(out,"%u;",rng.seed[j]);fputc('\n',out);
  }
 }
 if(!feof(in))I_Error("bad RNG lift manifest");fclose(in);fclose(out);exit(0);
}
