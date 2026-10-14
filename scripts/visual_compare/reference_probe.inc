/* Included only in the harness's private copy of Linux Doom's d_main.c.
 * Keep the original BSP, column, span, sky and masked renderers unchanged. */
void P_UpdateSpecials(void);
#include "r_local.h"
extern int numtextures, numflats, skytexture;
#include "sampling_probe.inc"
#include "camera_probe.inc"
void G_DoPlayDemo(void);
#include "span_trace.inc"

static void VisualProbe(void)
{
    int p = M_CheckParm("-visualprobe");
    FILE *input, *output, *cameras;
    char name[128], mapname[16], path[1024];
    int episode, map, tic, death_tics, fullbright, i, j;
    int replay = getenv("GD_VISUAL_DEMO") && !strcmp(getenv("GD_VISUAL_DEMO"), "1");
    double x, y, z;
    unsigned int angle;
    byte *palette;
    player_t *player;
    if (!p) return;
    if (p + 2 >= myargc) I_Error("-visualprobe scenes.tsv output-dir");
    input = fopen(myargv[p+1], "r");
    if (!input) I_Error("Cannot read visual probe scenes");
    snprintf(path, sizeof(path), "%s/classic-cameras.csv", myargv[p+2]);
    cameras = fopen(path, "w");
    if (!cameras) I_Error("Cannot write camera states");
    fprintf(cameras, "name,x,y,z,angle,leveltime,dead,bodyz,viewheight,floorz,ceilingz\n");
    while (fscanf(input, "%127s %15s %d %d %lf %lf %lf %u %d %d %d",
                  name, mapname, &episode, &map, &x, &y, &z, &angle,
                  &tic, &fullbright, &death_tics) == 11)
    {
        demoplayback = false;
        gametic = maketic = 0;
        if (replay) {
            G_DeferedPlayDemo("check");
            G_DoPlayDemo();
        } else G_InitNew(sk_medium, episode, map);
        /* Linux 1.10 G_DoLoadLevel compares gamemode with gamemission enum
         * values: retail == pack_tnt, accidentally replacing every Ultimate
         * Doom sky with SKY1. Restore the episode sky selected by G_InitNew.
         * This corrects probe setup, not the reference rasterizers. */
        if (gamemode == retail) {
            char skyname[8];
            snprintf(skyname, sizeof(skyname), "SKY%d", episode);
            skytexture = R_TextureNumForName(skyname);
        }
        /* A new probe scene starts from fresh texture translations, rather
         * than inheriting the final animation phase of the preceding scene. */
        for (i = 0; i < numtextures; ++i) texturetranslation[i] = i;
        for (i = 0; i < numflats; ++i) flattranslation[i] = i;
        /* Advance only texture specials. Geometry, lights and actors stay at
         * their initial state in both engines; this is a renderer comparison. */
        if (replay) {
            for (i=0; i<tic; ++i) {
                if (!demoplayback || gamestate != GS_LEVEL || gameaction != ga_nothing)
                    I_Error("Demo snapshot exceeds current level");
                G_Ticker(); gametic++; maketic++;
            }
            if (death_tics) {
                demoplayback = false;
                players[consoleplayer].powers[pw_invulnerability] = 0;
                players[consoleplayer].cheats &= ~CF_GODMODE;
                if (players[consoleplayer].playerstate != PST_DEAD)
                    P_DamageMobj(players[consoleplayer].mo, NULL, NULL, 10000);
                players[consoleplayer].attacker = NULL;
                memset(netcmds, 0, sizeof(netcmds));
                for (i=0; i<death_tics; ++i) { G_Ticker(); gametic++; maketic++; }
            }
        } else {
            for (i = 0; i < tic; ++i) { leveltime = i; P_UpdateSpecials(); }
            leveltime = tic;
        }
        player = &players[consoleplayer];
        if (!replay) {
        player->mo->x = (fixed_t)(x * FRACUNIT);
        player->mo->y = (fixed_t)(y * FRACUNIT);
        player->mo->angle = angle;
        player->mo->subsector = R_PointInSubsector(player->mo->x, player->mo->y);
        player->viewz = (fixed_t)(z * FRACUNIT);
        }
        fprintf(cameras, "%s,%d,%d,%d,%u,%d,%d,%d,%d,%d,%d\n", name, player->mo->x, player->mo->y, player->viewz, player->mo->angle, leveltime, player->playerstate==PST_DEAD, player->mo->z, player->viewheight, player->mo->floorz, player->mo->ceilingz);
        player->extralight = player->fixedcolormap = 0;
        for (i = 0; i < NUMPSPRITES; ++i) player->psprites[i].state = NULL;
        for (i = 0; i < numsectors; ++i) sectors[i].thinglist = NULL;
        if (fullbright)
            for (i = 0; i < 32 * 256; ++i) colormaps[i] = i % 256;
        R_SetViewSize(11, 0);
        R_ExecuteSetViewSize();
        memset(screens[0], 0, SCREENWIDTH * SCREENHEIGHT);
        if (getenv("GD_VISUAL_PIXELS") && !strcmp(getenv("GD_VISUAL_PIXELS"), "1")) {
            snprintf(path, sizeof(path), "%s/%s-classic-pixels.csv", myargv[p+2], name);
            visual_pixel_trace = fopen(path, "w");
            if (!visual_pixel_trace) I_Error("Cannot write pixel trace");
            memset(visual_pixels, 0, sizeof(visual_pixels));
        }
        if (getenv("GD_VISUAL_TRACE") && !strcmp(getenv("GD_VISUAL_TRACE"), "1")) {
            snprintf(path, sizeof(path), "%s/%s-classic-spans.csv", myargv[p+2], name);
            visual_span_trace = fopen(path, "w");
            if (!visual_span_trace) I_Error("Cannot write span trace");
            fprintf(visual_span_trace, "plane,height,flat,y,x1,x2,u,v,du,dv\n");
            snprintf(path, sizeof(path), "%s/%s-classic-walls.csv", myargv[p+2], name);
            visual_wall_trace = fopen(path, "w");
            if (!visual_wall_trace) I_Error("Cannot write wall trace");
            fprintf(visual_wall_trace, "seg,x1,x2,scale,step,distance,offset\n");
        }
        R_RenderPlayerView(player);
        if (visual_pixel_trace) {
            fprintf(visual_pixel_trace, "x,y,kind,owner,source,row,result,texel,available,frac,step\n");
            for (i=0; i<SCREENWIDTH*SCREENHEIGHT; ++i) {
                if (visual_pixels[i].result != screens[0][i]) I_Error("Pixel trace disagrees with rasterizer");
                fprintf(visual_pixel_trace, "%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d\n", i%SCREENWIDTH,i/SCREENWIDTH,
                    visual_pixels[i].kind,visual_pixels[i].owner,visual_pixels[i].source,
                    visual_pixels[i].row,visual_pixels[i].result,visual_pixels[i].texel,visual_pixels[i].available,visual_pixels[i].frac,visual_pixels[i].step);
            }
            fclose(visual_pixel_trace); visual_pixel_trace = NULL;
        }
        if (visual_span_trace) { fclose(visual_span_trace); visual_span_trace = NULL; fclose(visual_wall_trace); visual_wall_trace=NULL; }
        snprintf(path, sizeof(path), "%s/%s-classic.ppm", myargv[p+2], name);
        output = fopen(path, "wb");
        if (!output) I_Error("Cannot write visual probe image");
        fprintf(output, "P6\n%d %d\n255\n", SCREENWIDTH, SCREENHEIGHT);
        palette = W_CacheLumpName("PLAYPAL", PU_CACHE);
        for (i = 0; i < SCREENWIDTH * SCREENHEIGHT; ++i)
            for (j = 0; j < 3; ++j)
                /* Normalize both outputs to raw PLAYPAL. Vanilla gamma
                 * table zero adds one to low channels; GD gamma zero does
                 * not. Display gamma must not masquerade as a UV error. */
                fputc(palette[screens[0][i]*3+j], output);
        fclose(output);
        printf("visual probe: %s\n", name);
    }
    if (!feof(input)) I_Error("Malformed visual probe manifest");
    fclose(input);
    fclose(cameras);
    VisualSamplingProbe(myargv[p+2]);
    VisualCameraProbe(myargv[p+2]);
    exit(0);
}
