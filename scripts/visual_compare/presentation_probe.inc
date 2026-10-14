/* Focused presentation oracle; runs only in the harness's private source copy. */
#include "st_stuff.h"
void P_CalcHeight(player_t *);
void P_SetPsprite(player_t *, int, statenum_t);
void P_MovePsprites(player_t *);
#include "d_items.h"
void VisualStatusFixture(int);
void VisualNetFixture(int);
int VisualPaletteFixture(int,int,int,int);
extern int fuzzpos;
static int presentationPalette;
void R_DrawPlayerSprites(void);

static void PresentationPPM(const char *dir, const char *name, int top, int height)
{
    char path[1024]; int i,j; FILE *f;
    byte *pal = (byte*)W_CacheLumpName("PLAYPAL", PU_CACHE)+presentationPalette*768;
    snprintf(path,sizeof(path),"%s/%s-classic.ppm",dir,name);
    f=fopen(path,"wb"); if (!f) I_Error("Cannot write presentation capture");
    fprintf(f,"P6\n320 %d\n255\n",height);
    for(i=top*320;i<(top+height)*320;i++) for(j=0;j<3;j++) fputc(pal[screens[0][i]*3+j],f);
    fclose(f);
}
static void PresentationProbe(const char *dir)
{
    char path[1024],name[128],patch[9]; FILE *f; int s,pos,l,i,t;
    int lights[]={0,64,128,192,255};
    int xs[]={FRACUNIT,8*FRACUNIT+FRACUNIT/4,-3*FRACUNIT-FRACUNIT/8};
    int ys[]={32*FRACUNIT,35*FRACUNIT+FRACUNIT/2,77*FRACUNIT+FRACUNIT/4};
    player_t *p;
    if (!getenv("GD_PRESENTATION")) return;
    G_InitNew(sk_medium,1,1); p=&players[consoleplayer];
    for(i=0;i<numsectors;i++) sectors[i].thinglist=NULL;
    p->psprites[0].state=p->psprites[1].state=NULL;
    p->extralight=p->fixedcolormap=0;
    R_SetViewSize(11,0); R_ExecuteSetViewSize(); R_RenderPlayerView(p);
    snprintf(path,sizeof(path),"%s/weapons.csv",dir); f=fopen(path,"w");
    fprintf(f,"name,patch,x,y,light,fullbright\n");
    for(s=S_PUNCH;s<=S_BFGFLASH2;s++) {
        snprintf(patch,sizeof(patch),"%s%c0",sprnames[states[s].sprite],'A'+(states[s].frame&FF_FRAMEMASK));
        if(W_CheckNumForName(patch)<0) continue;
        for(pos=0;pos<3;pos++) for(l=0;l<5;l++) {
            snprintf(name,sizeof(name),"weapon-%03d-p%d-l%d",s,pos,l);
            p->psprites[0].state=&states[s]; p->psprites[0].sx=xs[pos]; p->psprites[0].sy=ys[pos];
            p->mo->subsector->sector->lightlevel=lights[l];
            memset(screens[0],0,SCREENWIDTH*SCREENHEIGHT); R_DrawPlayerSprites();
            PresentationPPM(dir,name,0,200);
            fprintf(f,"%s,%s,%d,%d,%d,%d\n",name,patch,xs[pos],ys[pos],lights[l],!!(states[s].frame&FF_FULLBRIGHT));
        }
    }
    fclose(f);
    for(i=0;i<72;i++) {
        VisualStatusFixture(i); memset(screens[0],0,SCREENWIDTH*SCREENHEIGHT); ST_Drawer(false,true);
        snprintf(name,sizeof(name),"hud-%02d",i); PresentationPPM(dir,name,168,32);
    }

    for(i=0;i<64;i++) {
        VisualNetFixture(i); memset(screens[0],0,SCREENWIDTH*SCREENHEIGHT); ST_Drawer(false,true);
        snprintf(name,sizeof(name),"net-hud-%02d",i); PresentationPPM(dir,name,168,32);
    }
    consoleplayer=0; netgame=false; deathmatch=false;
    G_InitNew(sk_medium,1,1); p=&players[0];
    snprintf(path,sizeof(path),"%s/effects.csv",dir); f=fopen(path,"w");
    fprintf(f,"name,damage,bonus,strength,suit,palette\n");
    for(i=0;i<96;i++) {
        int values[]={0,1,7,8,9,15,16,31,32,63,64,100,127,128,129,200};
        int v=values[i%16], d=0,b=0,z=0,r=0;
        if(i<16)d=v; else if(i<32)b=v; else if(i<48)z=v*8;
        else if(i<64)r=v; else if(i<80){d=v;b=40;r=200;} else {z=v*8;b=8;r=200;}
        presentationPalette=VisualPaletteFixture(d,b,z,r);
        for(t=0;t<320*200;t++)screens[0][t]=(t%320)/2+((t/320)%2)*128;
        snprintf(name,sizeof(name),"effect-%02d",i); PresentationPPM(dir,name,0,200);
        fprintf(f,"%s,%d,%d,%d,%d,%d\n",name,d,b,z,r,presentationPalette);
    }
    fclose(f); presentationPalette=0;
    G_InitNew(sk_medium,1,1); p=&players[0];
    for(i=0;i<numsectors;i++) sectors[i].thinglist=NULL;
    p->mo->subsector->sector->lightlevel=64;
    snprintf(path,sizeof(path),"%s/powers.csv",dir);f=fopen(path,"w");
    fprintf(f,"name,invuln,infrared,invis,flash\n");
    for(i=0;i<80;i++) {
        int times[]={0,1,7,8,15,16,120,127,128,129};
        int timer=times[i%10], group=i/10;
        memset(p->powers,0,sizeof(p->powers));
        if(group==0||group==3||group==6) p->powers[pw_invulnerability]=timer;
        if(group==1||group==3||group==7) p->powers[pw_infrared]=timer;
        if(group==2||group==4||group>=6) p->powers[pw_invisibility]=timer;
        if(group==5){p->powers[pw_invulnerability]=129;p->powers[pw_invisibility]=timer;}
        p->fixedcolormap=(p->powers[pw_invulnerability]>128||(p->powers[pw_invulnerability]&8))?32:
            ((!p->powers[pw_invulnerability] && (p->powers[pw_infrared]>128||(p->powers[pw_infrared]&8)))?1:0);
        p->extralight=group>=4?1:0;
        p->psprites[0].state=&states[S_PISTOL];p->psprites[0].sx=FRACUNIT;p->psprites[0].sy=32*FRACUNIT;
        p->psprites[1]=p->psprites[0];p->psprites[1].state=group>=4?&states[S_PISTOLFLASH]:NULL;
        R_RenderPlayerView(p); fuzzpos=0;
        for(t=0;t<320*200;t++)screens[0][t]=(t%320)/2+((t/320)%2)*128;
        R_DrawPlayerSprites(); snprintf(name,sizeof(name),"power-%02d",i); PresentationPPM(dir,name,0,200);
        fprintf(f,"%s,%d,%d,%d,%d\n",name,p->powers[pw_invulnerability],p->powers[pw_infrared],p->powers[pw_invisibility],group>=4);
    }
    fclose(f);
    G_InitNew(sk_medium,1,1); p=&players[consoleplayer];
    for(i=0;i<numsectors;i++) sectors[i].thinglist=NULL;
    p->mo->subsector->sector->lightlevel=128;
    p->extralight=p->fixedcolormap=0;
    R_RenderPlayerView(p);
    snprintf(path,sizeof(path),"%s/sequence.csv",dir); f=fopen(path,"w");
    fprintf(f,"tic,x,y\n");
    for(t=0;t<100;t++) {
        leveltime=t; p->mo->momx=(t%10)*FRACUNIT; p->mo->momy=FRACUNIT/2;
        P_CalcHeight(p);
        if(t==30) P_SetPsprite(p,ps_weapon,S_PISTOL1);
        if(t==50) {p->health=0; p->playerstate=PST_DEAD; P_SetPsprite(p,ps_weapon,S_PISTOLDOWN);}
        P_MovePsprites(p); extralight=p->extralight;
        memset(screens[0],0,SCREENWIDTH*SCREENHEIGHT);R_DrawPlayerSprites();
        snprintf(name,sizeof(name),"sequence-%03d",t);PresentationPPM(dir,name,0,200);
        fprintf(f,"%d,%d,%d\n",t,p->psprites[0].sx,p->psprites[0].sy);
    }
    fclose(f);
    snprintf(path,sizeof(path),"%s/bob.csv",dir); f=fopen(path,"w");
    fprintf(f,"tic,momx,momy,x,y\n");
    for(i=0;i<4;i++) for(t=0;t<130;t++) {
        p->mo->momx=i*2*FRACUNIT+1234; p->mo->momy=i*FRACUNIT-2345;
        p->mo->z=p->mo->floorz; p->viewheight=41*FRACUNIT;
        p->readyweapon=wp_pistol; p->pendingweapon=wp_nochange; p->cmd.buttons=0; p->health=100; p->playerstate=PST_LIVE;
        leveltime=t; P_CalcHeight(p); P_SetPsprite(p,ps_weapon,S_PISTOL);
        fprintf(f,"%d,%d,%d,%d,%d\n",t,p->mo->momx,p->mo->momy,p->psprites[0].sx,p->psprites[0].sy);
    }
    fclose(f);
}
