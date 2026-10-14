/* Read-only hooks in the private reference build; never change rasterization. */
#include <stdint.h>
static FILE *visual_span_trace;
static FILE *visual_pixel_trace;
static struct { int kind, owner, source, row, result, texel, available, frac, step; } visual_pixels[SCREENWIDTH*SCREENHEIGHT];
static int visual_kind, visual_owner;
void VisualTraceColumn(void)
{
    extern byte **texturecomposite;
    extern int *texturecompositesize;
    int y, i, source, row, k, available = -1;
    unsigned int frac;
    if (!visual_pixel_trace) return;
    frac = dc_texturemid + (dc_yl-centery)*(unsigned int)dc_iscale;
    row = (dc_colormap-colormaps)/256;
    for (k=0;k<numlumps;++k) if (lumpcache[k] &&
        (uintptr_t)dc_source >= (uintptr_t)lumpcache[k] &&
        (uintptr_t)dc_source < (uintptr_t)lumpcache[k]+lumpinfo[k].size) {
        available = lumpinfo[k].size - ((uintptr_t)dc_source-(uintptr_t)lumpcache[k]);
        break;
    }
    if (available < 0) for (k=0;k<numtextures;++k) if (texturecomposite[k] &&
        (uintptr_t)dc_source >= (uintptr_t)texturecomposite[k] &&
        (uintptr_t)dc_source < (uintptr_t)texturecomposite[k]+texturecompositesize[k]) {
        available = texturecompositesize[k] - ((uintptr_t)dc_source-(uintptr_t)texturecomposite[k]);
        break;
    }
    for (y = dc_yl; y <= dc_yh; ++y, frac += dc_iscale) {
        if (dc_x < 0 || dc_x >= SCREENWIDTH || y < 0 || y >= SCREENHEIGHT) continue;
        i = y*SCREENWIDTH+dc_x;
        source = dc_source[(frac>>16)&127];
        visual_pixels[i].kind = visual_kind;
        visual_pixels[i].owner = visual_owner;
        visual_pixels[i].source = source;
        visual_pixels[i].row = row;
        visual_pixels[i].result = dc_colormap[source];
        visual_pixels[i].texel = (frac>>16)&127;
        visual_pixels[i].available = available;
        visual_pixels[i].frac = frac; visual_pixels[i].step = dc_iscale;
    }
}
void VisualTraceMasked(void)
{
    visual_kind = 3;
    visual_owner = curline-segs;
}
static int visual_plane_id, visual_plane_height, visual_plane_pic;
void VisualTracePlane(int id, int height, int pic)
{
    visual_plane_id = id;
    visual_plane_height = height;
    visual_plane_pic = pic;
    visual_kind = pic == skyflatnum ? 4 : 2;
    visual_owner = id;
}
void VisualTraceSpan(void)
{
    int x, i, source;
    unsigned int u = ds_xfrac, v = ds_yfrac;
    if (visual_pixel_trace) for (x=ds_x1; x<=ds_x2; ++x, u+=ds_xstep, v+=ds_ystep) {
        if (x<0 || x>=SCREENWIDTH || ds_y<0 || ds_y>=SCREENHEIGHT) continue;
        i = ds_y*SCREENWIDTH+x;
        source = ds_source[((v>>16)&63)*64+((u>>16)&63)];
        visual_pixels[i].kind = 2;
        visual_pixels[i].owner = visual_plane_id;
        visual_pixels[i].source = source;
        visual_pixels[i].row = (ds_colormap-colormaps)/256;
        visual_pixels[i].result = ds_colormap[source];
        visual_pixels[i].texel = ((v>>16)&63)*64+((u>>16)&63);
        visual_pixels[i].available = 4096;
    }
    if (!visual_span_trace) return;
    fprintf(visual_span_trace, "%d,%d,%.8s,%d,%d,%d,%d,%d,%d,%d\n",
            visual_plane_id, visual_plane_height,
            lumpinfo[firstflat + flattranslation[visual_plane_pic]].name,
            ds_y, ds_x1, ds_x2, ds_xfrac, ds_yfrac, ds_xstep, ds_ystep);
}

static FILE *visual_wall_trace;
void VisualTraceWall(void)
{
    extern int rw_x, rw_stopx;
    extern fixed_t rw_scale, rw_scalestep, rw_distance, rw_offset;
    visual_kind = 1;
    visual_owner = curline-segs;
    if (!visual_wall_trace) return;
    fprintf(visual_wall_trace, "%d,%d,%d,%d,%d,%d,%d\n", curline-segs,
            rw_x, rw_stopx-1, rw_scale, rw_scalestep, rw_distance, rw_offset);
}
