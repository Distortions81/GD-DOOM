/* Scalar oracle from the original functions, called after all image captures. */
static void VisualSamplingProbe(const char *directory)
{
    static unsigned int angles[] = {0, 0x40000000u, 0x80000000u, 0xc0000000u, 0x12345678u, 0xffffffffu};
    static int columns[] = {0, 1, 17, SCREENWIDTH/2-1, SCREENWIDTH/2, SCREENWIDTH/2+1, SCREENWIDTH-19, SCREENWIDTH-1};
    static int rows[] = {SCREENHEIGHT/8, SCREENHEIGHT/4, SCREENHEIGHT/2-1, SCREENHEIGHT/2, SCREENHEIGHT*3/4, SCREENHEIGHT-1};
    static byte flat[4096];
    extern fixed_t planeheight, rw_distance;
    extern angle_t rw_normalangle;
    FILE *out;
    char path[1024];
    int a, x, y;
    snprintf(path, sizeof(path), "%s/classic-sampling.csv", directory);
    out = fopen(path, "w");
    if (!out) I_Error("Cannot write sampling oracle");
    fprintf(out, "kind,angle,x,y,u,v,du,dv\n");
    viewx = 13*FRACUNIT; viewy = -27*FRACUNIT;
    fixedcolormap = colormaps;
    ds_source = flat;
    for (a = 0; a < 6; ++a) {
        viewangle = angles[a];
        R_ClearPlanes();
        for (x = 0; x < 8; ++x) {
            fprintf(out, "sky,%u,%d,0,%u,0,0,0\n", viewangle, columns[x], (viewangle+xtoviewangle[columns[x]])>>22);
            rw_normalangle = viewangle;
            rw_distance = 64*FRACUNIT;
            fprintf(out, "scale,%u,%d,0,%d,0,0,0\n", viewangle, columns[x], R_ScaleFromGlobalAngle(viewangle+xtoviewangle[columns[x]]));
            for (y = 0; y < 6; ++y) {
                planeheight = (rows[y]<SCREENHEIGHT/2 ? 87 : 41)*FRACUNIT;
                R_MapPlane(rows[y], columns[x], columns[x]);
                fprintf(out, "plane,%u,%d,%d,%d,%d,%d,%d\n", viewangle, columns[x], rows[y], ds_xfrac, ds_yfrac, ds_xstep, ds_ystep);
            }
        }
    }
    fclose(out);
    snprintf(path, sizeof(path), "%s/classic-lighting.csv", directory);
    out = fopen(path, "w");
    if (!out) I_Error("Cannot write lighting oracle");
    fprintf(out,"kind,light,index,row\n");
    for (a=0;a<LIGHTLEVELS;++a) {
        for (x=0;x<MAXLIGHTSCALE;++x)
            fprintf(out,"wall,%d,%d,%d\n",a*16,x,(scalelight[a][x]-colormaps)/256);
        for (x=0;x<MAXLIGHTZ;++x)
            fprintf(out,"plane,%d,%d,%d\n",a*16,x,(zlight[a][x]-colormaps)/256);
    }
    fclose(out);
}
