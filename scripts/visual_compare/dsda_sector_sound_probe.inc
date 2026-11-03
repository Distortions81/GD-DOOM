/* Observe sector sound requests only. No sound-channel or mover behavior changes. */
#include "sounds.h"
#include "m_random.h"
static FILE *GD_SectorSoundOut;
static char GD_SectorSoundEvents[65536];
static size_t GD_SectorSoundLength;
void GD_CaptureSectorSound(sector_t *sector, int sfx_id, int timeout)
{
 char name[16];int i,n;
 if(!GD_SectorSoundOut)return;
 snprintf(name,sizeof(name),"%s",S_sfx[sfx_id].name);
 for(i=0;name[i];++i)if(name[i]>='a'&&name[i]<='z')name[i]-='a'-'A';
 n=snprintf(GD_SectorSoundEvents+GD_SectorSoundLength,sizeof(GD_SectorSoundEvents)-GD_SectorSoundLength,
  "%s:%ld:%d:%d:%d;",name,(long)(sector-sectors),sector->soundorg.x,sector->soundorg.y,timeout);
 if(n<0||(size_t)n>=sizeof(GD_SectorSoundEvents)-GD_SectorSoundLength)I_Error("sector sound trace overflow");
 GD_SectorSoundLength+=n;
}
static void GD_SectorSoundBegin(void)
{
 const char *path=getenv("GD_SECTOR_SOUND_OUT");
 if(path&&!GD_SectorSoundOut){GD_SectorSoundOut=fopen(path,"w");if(!GD_SectorSoundOut)I_Error("sector sound output");fprintf(GD_SectorSoundOut,"name,tic,events\n");}
 GD_SectorSoundLength=0;GD_SectorSoundEvents[0]=0;
}
static void GD_SectorSoundFrame(const char *name,int tic)
{
 if(GD_SectorSoundOut)fprintf(GD_SectorSoundOut,"%s,%d,%s\n",name,tic,GD_SectorSoundEvents);
 GD_SectorSoundLength=0;GD_SectorSoundEvents[0]=0;
}

/* Match the initial random lift choice, without claiming RNG stream parity.
 * No later mover/dispatcher code or RNG draw is replaced. */
static void GD_SectorSoundSeed(const char *name)
{
 int up=!strncmp(name,"rng1_lift_",10);
 if(!strncmp(name,"lift_",5)||up){rng.seed[pr_genlift]=rng.seed[pr_all_in_one]=up?1<<20:0;boom_basetic=1;}
}
