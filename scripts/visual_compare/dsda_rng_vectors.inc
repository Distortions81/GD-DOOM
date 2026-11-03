#include "m_random.h"
static void GD_RNGVectors(void)
{
 const char *path=getenv("GD_DSDA_RNG_VECTORS"),*output=getenv("GD_DSDA_RNG_OUT");FILE *in,*out;char name[128];unsigned seed,tic;int insurance,cls,count,i,value;
 if(!path)return;in=fopen(path,"r");out=output?fopen(output,"w"):NULL;if(!in||!out)I_Error("RNG vector files");
 fprintf(out,"name,draw,value,playindex,menuindex,seeds\n");
 while(fscanf(in,"%127s %u %d %u %d %d",name,&seed,&insurance,&tic,&cls,&count)==6){
  compatibility_level=boom_202_compatibility;demo_insurance=insurance;rngseed=seed;M_ClearRandom();gametic=tic;boom_basetic=0;
  for(i=0;i<count;++i){int c=cls<0?(i*17+13)%NUMPRCLASS:cls;value=P_Random(c);
   fprintf(out,"%s,%d,%d,%d,%d,",name,i,value,rng.rndindex,rng.prndindex);{int j;for(j=0;j<NUMPRCLASS;++j)fprintf(out,"%u;",rng.seed[j]);}fputc('\n',out);
  }
 }
 if(!feof(in))I_Error("bad RNG vectors");fclose(in);fclose(out);exit(0);
}
