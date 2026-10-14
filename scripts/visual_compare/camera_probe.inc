/* Exercise the unmodified P_DeathThink/P_CalcHeight with moving bodies and bob. */
extern boolean onground;
void P_DeathThink(player_t *player);
void P_CalcHeight(player_t *player);
static void VisualCameraProbe(const char *directory)
{
    FILE *out;
    char path[1024];
    player_t player;
    mobj_t body;
    int c, tic, before, delta;
    snprintf(path, sizeof(path), "%s/classic-camera.csv", directory);
    out = fopen(path, "w");
    if (!out) I_Error("Cannot write camera oracle");
    fprintf(out, "dead,tic,z,floor,ceiling,momx,momy,height,delta,viewz,nextheight,nextdelta\n");
    for (c=0; c<12; ++c) {
        memset(&player, 0, sizeof(player));
        memset(&body, 0, sizeof(body));
        player.mo = &body;
        player.playerstate = c<8 ? PST_DEAD : PST_LIVE;
        player.viewheight = (c==5 ? 20 : 41)*FRACUNIT;
        player.deltaviewheight = c==5 ? -FRACUNIT : 0;
        for (tic=0; tic<48; ++tic) {
            leveltime = tic;
            body.floorz = (c==1 ? -6 : c==2 ? -64 : c==3 ? tic : 0)*FRACUNIT;
            body.z = body.floorz + (c==4 || c==11 ? (48-tic)*FRACUNIT : 0);
            body.ceilingz = body.floorz + (c==6 || c==10 ? 32 : 128)*FRACUNIT;
            body.momx = (c==7 || c==9 ? 7 : 0)*FRACUNIT;
            body.momy = (c==7 || c==9 ? -3 : 0)*FRACUNIT;
            before=player.viewheight; delta=player.deltaviewheight;
            onground = body.z <= body.floorz;
            if (player.playerstate==PST_DEAD) P_DeathThink(&player);
            else P_CalcHeight(&player);
            fprintf(out,"%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d\n",player.playerstate==PST_DEAD,tic,
                body.z,body.floorz,body.ceilingz,body.momx,body.momy,before,delta,
                player.viewz,player.viewheight,player.deltaviewheight);
        }
    }
    fclose(out);
}
