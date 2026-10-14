/* Included in st_stuff.c to control face selection without replacing widgets. */
void VisualStatusFixture(int n)
{
    int i; int health[]={100,0,1,9,10,99,101,200,-5};
    player_t *p=&players[consoleplayer];
    ST_Start();
    p->health=health[n%9]; p->armorpoints=(n*13)%201;
    p->readyweapon=(n%8==0)?wp_fist:wp_pistol;
    for(i=0;i<NUMWEAPONS;i++) p->weaponowned[i]=((n>> (i%6))&1);
    p->weaponowned[wp_pistol]=true;
    for(i=0;i<NUMAMMO;i++) {p->ammo[i]=(n*17+i*23)%201; p->maxammo[i]=maxammo[i]*(1+(n%2));}
    for(i=0;i<NUMCARDS;i++) p->cards[i]=(n>>i)&1;
    ST_updateWidgets(); st_faceindex=n%ST_NUMFACES;
}

int VisualPaletteFixture(int damage, int bonus, int strength, int suit)
{
    player_t *p=&players[consoleplayer];
    ST_Start(); p->damagecount=damage; p->bonuscount=bonus;
    p->powers[pw_strength]=strength; p->powers[pw_ironfeet]=suit;
    ST_doPaletteStuff(); return st_palette;
}
void VisualNetFixture(int n)
{
    int i; int scores[]={0,1,-1,9,10,99,100,-100};
    consoleplayer=n%4; netgame=true; deathmatch=n>=32;
    ST_loadGraphics(); VisualStatusFixture(n);
    for(i=0;i<MAXPLAYERS;i++) players[consoleplayer].frags[i]=0;
    /* Include suicide subtraction, not just a precomputed positive score. */
    players[consoleplayer].frags[(consoleplayer+1)%4]=scores[(n/4)%8]+3;
    players[consoleplayer].frags[consoleplayer]=3;
    ST_updateWidgets(); st_faceindex=n%ST_NUMFACES;
}
