"""Randomlocke: IA de entrenadores + equipos progresivos para entrenadores importantes."""
import struct,glob,os,sys,json,random,collections
ROM,OUT=sys.argv[1],sys.argv[2]; LANG='Spanish'
def text(path):
    d=open(path,'rb').read(); n=struct.unpack_from('<H',d,2)[0]; sdo=struct.unpack_from('<I',d,0xC)[0]
    out=[]; key=0x7C89
    for i in range(n):
        off,ln,fl=struct.unpack_from('<iHH',d,sdo+4+8*i); k=key; s=[]
        for j in range(ln):
            c=struct.unpack_from('<H',d,sdo+off+2*j)[0]^k; k=((k<<3)|(k>>13))&0xFFFF
            if c==0: break
            if c>=0x20: s.append(chr(c))
        out.append(''.join(s)); key=(key+0x2983)&0xFFFF
    return out
B=os.path.join(ROM,'bin')
names=text(os.path.join(B,'message',LANG,'common','trname.dat'))
# --- datos de especies ---
per=open(os.path.join(B,'pml','personal','personal_total.bin'),'rb').read(); PS=0xB0
lrn=open(os.path.join(B,'pml','waza_oboe','wazaoboe_total.bin'),'rb').read(); LS=0x104
def P(i): return per[i*PS:(i+1)*PS]
def learnset(i):
    d=lrn[i*LS:(i+1)*LS]; out=[]
    for k in range(0,LS,4):
        if d[k+3]==0xFF: break
        out.append(struct.unpack_from('<hh',d,k))
    return out
LEGEND={144,145,146,150,151,243,244,245,249,250,251,377,378,379,380,381,382,383,384,385,386,480,481,482,483,484,485,486,487,488,489,490,491,492,493,494,
        638,639,640,641,642,643,644,645,646,647,648,649,716,717,718,719,720,721,772,773,785,786,787,788,789,790,791,792,793,794,795,796,797,798,799,800,801,802,
        803,804,805,806,807,808,809,888,889,890,891,892,893,894,895,896,897,898,132,235}
pool=[]
for s in range(1,899):
    p=P(s)
    if not (p[0x21]>>6)&1 or s in LEGEND: continue
    bst=sum(p[0:6]); pool.append({'sp':s,'bst':bst,'stage':p[9],'t':{p[6],p[7]}})
def suitable(level):
    if level<20: return [c for c in pool if c['stage']==1 and c['bst']<=420 or c['stage']==1 and c['bst']<=330]
    if level<35: return [c for c in pool if c['stage']<=2 and c['bst']<=500]
    if level<50: return [c for c in pool if c['bst']<=560]
    return [c for c in pool if c['bst']<=600]
def moves_at(sp,level):
    ms=[m for m,l in learnset(sp) if l<=level and m>0]
    last=[]
    for m in ms:
        if m in last: last.remove(m)
        last.append(m)
    last=last[-4:]
    return last+[0]*(4-len(last))
# --- quienes son importantes ---
SIGN={'Paul','Berto','Roxy','Percy','Cathy','Naboru','Judith','Alistair','Sally','Morris','Mel','Nerio','Roy','Olivia','Rose','Lionel',
      'Mostaz','Dreo','Sófora','Peony','Tizonio','Dargo'}
def target_size(ace):
    if ace<=9: return None
    if ace<=16: return 3
    if ace<=21: return 4
    if ace<=26: return 5
    return 6
rng=random.Random(2026); log=collections.Counter(); changes=[]
os.makedirs(os.path.join(OUT,'trainer_data'),exist_ok=True); os.makedirs(os.path.join(OUT,'trainer_poke'),exist_ok=True)
for f in sorted(glob.glob(os.path.join(B,'trainer','trainer_data','*.bin'))):
    i=int(f[-7:-4]); d=bytearray(open(f,'rb').read())
    pf=os.path.join(B,'trainer','trainer_poke',f'trainer_poke_{i:03d}.bin'); p=bytearray(open(pf,'rb').read())
    num=d[3]; mode=d[2]; ai=struct.unpack_from('<I',d,0xC)[0]
    if num==0: continue
    # IA: Basic+Strong+Expert, sin cambiar de Pokemon; objetos y dobles como estaban
    new_ai=(ai|0b111)&~0x40
    team=[p[o:o+0x20] for o in range(0,len(p),0x20)]
    name=names[i] if i<len(names) else ''
    lv=[struct.unpack_from('<H',t,0x0A)[0] for t in team]; ace=max(lv)
    tgt=target_size(ace) if name in SIGN and num>=2 and (mode==0 or i==144) else None
    if tgt and tgt>num:
        tcount=collections.Counter(); 
        for t in team:
            sp=struct.unpack_from('<H',t,0x0C)[0]; tcount.update({P(sp)[6],P(sp)[7]})
        tt,tn=tcount.most_common(1)[0]; theme=tt if tn>=max(2,len(team)-1) else None
        level=min(lv)
        used={struct.unpack_from('<H',t,0x0C)[0] for t in team}
        cand=[c for c in suitable(level) if c['sp'] not in used]
        if theme is not None and name not in ('Paul','Berto','Roxy','Lionel','Olivia','Rose','Mostaz','Dreo','Sófora','Peony','Tizonio','Dargo'):
            th=[c for c in cand if theme in c['t']]
            if len(th)>=6: cand=th
        rng.shuffle(cand); added=[]
        tmpl=team[0]
        while len(team)<tgt and cand:
            c=cand.pop(); nt=bytearray(tmpl)
            struct.pack_into('<HHHH',nt,0x0A,level,c['sp'],0,0)   # level, species, form, item
            struct.pack_into('<4H',nt,0x12,*moves_at(c['sp'],level))
            nt[0]=0; nt[8]=0; nt[9]=0
            iv=struct.unpack_from('<I',nt,0x1C)[0]&0x3FFFFFFF; struct.pack_into('<I',nt,0x1C,iv)
            team.insert(len(team)-1,bytes(nt)); added.append(c['sp'])
        d[3]=len(team); changes.append((i,name,num,len(team),ace,added))
    struct.pack_into('<I',d,0xC,new_ai)
    if new_ai!=ai: log['ia']+=1
    open(os.path.join(OUT,'trainer_data',os.path.basename(f)),'wb').write(d)
    open(os.path.join(OUT,'trainer_poke',os.path.basename(pf)),'wb').write(b''.join(team))
print('entrenadores con IA mejorada:',log['ia'])
for c in changes: print(c)
