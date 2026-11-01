/* Private DSDA-Doom 0.27.5 capture harness. It selects cameras and exports
 * PLAYPAL-normalized software frames; rasterizer and transfer logic are intact.
 * Include in d_main.c and call immediately after D_DoomMainSetup(). */
#include "r_main.h"
#include "r_data.h"
#include "r_draw.h"
#include "r_fps.h"
#include "p_maputl.h"
#include "p_map.h"
#include "p_tick.h"
#include "dsda/configuration.h"

#include "dsda_teleport_probe.inc"

static void GD_VisualProbe(void)
{
    const char *manifest = getenv("GD_DSDA_VISUAL_SCENES");
    const char *out = getenv("GD_DSDA_VISUAL_OUT");
    FILE *input, *image, *cameras;
    char name[128], mapname[16], path[2048];
    int episode, map, tic, bright, death, i, x, y, c;
    double px, py, pz;
    unsigned angle;
 unsigned long long usemask;
    const byte *palette;
    GD_TeleportProbe();
    if (!manifest) return;
    if (!out || !V_IsSoftwareMode()) I_Error("GD probe requires output directory and software mode");
    input = fopen(manifest, "r");
    if (!input) I_Error("GD probe cannot open manifest");
    snprintf(path, sizeof(path), "%s/dsda-cameras.csv", out);
    cameras = fopen(path, "w");
    if (!cameras) I_Error("GD probe cannot write camera log");
    fprintf(cameras, "name,x,y,z,angle,leveltime,width,height\n");
    dsda_UpdateIntConfig(dsda_config_screenblocks, 11, false);
    while (fscanf(input, "%127s %15s %d %d %lf %lf %lf %u %d %d %d %llu", name, mapname, &episode, &map, &px, &py, &pz, &angle, &tic, &bright, &death, &usemask) == 12) {
        player_t *p;
        if (death || bright) I_Error("GD fixed-camera probe expects normal lighting and no forced death");
        G_InitNew(2, episode, map, true);
        /* Each scene models a fresh process, including pre-first-tic identity
         * translations. G_InitNew alone retains the preceding map's tables. */
        for(i=0;i<numtextures;++i) texturetranslation[i]=i;
        for(i=0;i<numflats;++i) flattranslation[i]=i;
        if (getenv("GD_VISUAL_TRANMAP") && !strcmp(getenv("GD_VISUAL_TRANMAP"),"1")) {
            snprintf(path,sizeof(path),"%s/dsda-tranmap.bin",out);
            image=fopen(path,"wb");if(!image) I_Error("GD probe cannot export TRANMAP");
            fwrite(main_tranmap,1,65536,image);fclose(image);
        }
        /* Fresh linedefs have r_validcount == 0. Real play renders after a
         * game tic; invalidate that cache without advancing simulation. */
        gametic = 1;
        if (getenv("GD_VISUAL_SIMULATE") && !strcmp(getenv("GD_VISUAL_SIMULATE"),"1")) {
            menuactive=false;
            for (i=0;i<tic;++i) { gametic=i+1; players[displayplayer].cmd.buttons = i<64 && (usemask & (1ULL<<i)) ? BT_USE : 0; P_Ticker(); }
            snprintf(path,sizeof(path),"%s/%s-dsda-state.csv",out,name);
            image=fopen(path,"w"); if(!image) I_Error("GD probe cannot write state");
            p=&players[displayplayer];
            fprintf(image,"player,%d,%d,%d,%d,%d\n",p->mo->x,p->mo->y,p->mo->z,p->mo->momx,p->mo->momy);
            for(i=0;i<numsectors;++i) fprintf(image,"sector,%d,%d,%d,%d,%d,%d,%d\n",i,sectors[i].floorheight,sectors[i].ceilingheight,sectors[i].floor_xoffs,sectors[i].floor_yoffs,sectors[i].ceiling_xoffs,sectors[i].ceiling_yoffs);
            for(i=0;i<numsides;++i) fprintf(image,"side,%d,%d,%d\n",i,sides[i].textureoffset,sides[i].rowoffset);
            if (getenv("GD_VISUAL_TABLE_PROBES") && *getenv("GD_VISUAL_TABLE_PROBES")) {
                char *probes=strdup(getenv("GD_VISUAL_TABLE_PROBES")), *probe;
                for(i=0;i<numsectors;++i) fprintf(image,"light,%d,%d\n",i,sectors[i].lightlevel);
                for(i=0;i<numlines;++i) fprintf(image,"line,%d,%d\n",i,lines[i].special);
                for(i=0;i<numsides;++i) fprintf(image,"textures,%d,%.8s,%.8s,%.8s\n",i,sides[i].toptexture ? textures[sides[i].toptexture]->name : "-",sides[i].midtexture ? textures[sides[i].midtexture]->name : "-",sides[i].bottomtexture ? textures[sides[i].bottomtexture]->name : "-");
                for(i=0;i<MAXBUTTONS;++i) if(buttonlist[i].btimer) fprintf(image,"button,%ld,%d\n",buttonlist[i].line-lines,buttonlist[i].btimer);
                for(probe=strtok(probes,",");probe;probe=strtok(NULL,",")) {
                    int n; const char *translated;
                    if(probe[0]=='F') { n=R_FlatNumForName(probe+2); translated=W_LumpName(firstflat+flattranslation[n]); }
                    else { n=R_TextureNumForName(probe+2); translated=textures[texturetranslation[n]]->name; }
                    fprintf(image,"translation,%c,%s,%.8s\n",probe[0],probe+2,translated);
                }
                free(probes);
            }
            fclose(image);
        } else {
            for (i=0; i<tic; ++i) { leveltime=i; P_UpdateSpecials(); }
        }
        leveltime=tic;
        p = &players[displayplayer];
        P_UnsetThingPosition(p->mo);
        p->mo->x = (fixed_t)(px*FRACUNIT);
        p->mo->y = (fixed_t)(py*FRACUNIT);
        p->mo->angle = angle;
        P_SetThingPosition(p->mo);
        p->viewz = (fixed_t)(pz*FRACUNIT);
        p->extralight = p->fixedcolormap = 0;
        for (i=0; i<NUMPSPRITES; ++i) p->psprites[i].state=NULL;
        if (!getenv("GD_VISUAL_ACTORS") || strcmp(getenv("GD_VISUAL_ACTORS"), "1"))
            for (i=0; i<numsectors; ++i) sectors[i].thinglist=NULL;
        R_SetViewSize();
        R_ExecuteSetViewSize();
        viewx=p->mo->x; viewy=p->mo->y; viewz=p->viewz;
        viewangle=p->mo->angle; viewpitch=0;
        memset(screens[0].data, 0, screens[0].pitch*SCREENHEIGHT);
        R_RenderPlayerView(p);
        fprintf(cameras,"%s,%d,%d,%d,%u,%d,%d,%d\n",name,viewx,viewy,viewz,viewangle,leveltime,SCREENWIDTH,SCREENHEIGHT);
        snprintf(path,sizeof(path),"%s/%s-dsda.ppm",out,name);
        image=fopen(path,"wb");
        if (!image) I_Error("GD probe cannot write image");
        fprintf(image,"P6\n%d %d\n255\n",SCREENWIDTH,SCREENHEIGHT);
        palette=W_LumpByNum(W_GetNumForName("PLAYPAL"));
        for(y=0;y<SCREENHEIGHT;++y) for(x=0;x<SCREENWIDTH;++x) {
            byte index=screens[0].data[y*screens[0].pitch+x];
            for(c=0;c<3;++c) fputc(palette[index*3+c],image);
        }
        fclose(image);
        P_MapEnd();
    }
    if (!feof(input)) I_Error("GD probe malformed manifest");
    fclose(input); fclose(cameras); exit(0);
}
