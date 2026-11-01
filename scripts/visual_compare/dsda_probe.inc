/* Private DSDA-Doom 0.27.5 capture harness. It selects cameras and exports
 * PLAYPAL-normalized software frames; rasterizer and transfer logic are intact.
 * Include in d_main.c and call immediately after D_DoomMainSetup(). */
#include "r_main.h"
#include "r_data.h"
#include "r_fps.h"
#include "p_maputl.h"
#include "p_map.h"
#include "dsda/configuration.h"

static void GD_VisualProbe(void)
{
    const char *manifest = getenv("GD_DSDA_VISUAL_SCENES");
    const char *out = getenv("GD_DSDA_VISUAL_OUT");
    FILE *input, *image, *cameras;
    char name[128], mapname[16], path[2048];
    int episode, map, tic, bright, death, i, x, y, c;
    double px, py, pz;
    unsigned angle;
    const byte *palette;
    if (!manifest) return;
    if (!out || !V_IsSoftwareMode()) I_Error("GD probe requires output directory and software mode");
    input = fopen(manifest, "r");
    if (!input) I_Error("GD probe cannot open manifest");
    snprintf(path, sizeof(path), "%s/dsda-cameras.csv", out);
    cameras = fopen(path, "w");
    if (!cameras) I_Error("GD probe cannot write camera log");
    fprintf(cameras, "name,x,y,z,angle,leveltime,width,height\n");
    dsda_UpdateIntConfig(dsda_config_screenblocks, 11, false);
    while (fscanf(input, "%127s %15s %d %d %lf %lf %lf %u %d %d %d", name, mapname, &episode, &map, &px, &py, &pz, &angle, &tic, &bright, &death) == 11) {
        player_t *p;
        if (death || bright) I_Error("GD fixed-camera probe expects normal lighting and no forced death");
        G_InitNew(2, episode, map, true);
        /* Fresh linedefs have r_validcount == 0. Real play renders after a
         * game tic; invalidate that cache without advancing simulation. */
        gametic = 1;
        for (i=0; i<tic; ++i) { leveltime=i; P_UpdateSpecials(); }
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
