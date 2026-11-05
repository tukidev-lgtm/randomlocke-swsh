import struct,glob,os,sys,json
ROM=sys.argv[1]; LANG=sys.argv[2] if len(sys.argv)>2 else 'Spanish'
def text(path):
    d=open(path,'rb').read(); n=struct.unpack_from('<H',d,2)[0]; sdo=struct.unpack_from('<I',d,0xC)[0]
    out=[]; key=0x7C89
    for i in range(n):
        off,ln,fl=struct.unpack_from('<iHH',d,sdo+4+8*i); k=key; s=[]
        for j in range(ln):
            c=struct.unpack_from('<H',d,sdo+off+2*j)[0]^k; k=((k<<3)|(k>>13))&0xFFFF
            if c==0: break
            if c==0x10: s.append('{v}'); 
            elif c<0x20: continue
            else: s.append(chr(c))
        out.append(''.join(s)); key=(key+0x2983)&0xFFFF
    return out
base=os.path.join(ROM,'bin','message',LANG,'common')
names=text(os.path.join(base,'trname.dat')); classes=text(os.path.join(base,'trtype.dat'))
rows=[]
for f in sorted(glob.glob(os.path.join(ROM,'bin','trainer','trainer_data','*.bin'))):
    i=int(f[-7:-4]); d=open(f,'rb').read()
    cls,mode,num=struct.unpack_from('<HBB',d,0); ai=struct.unpack_from('<I',d,0xC)[0]
    p=open(os.path.join(ROM,'bin','trainer','trainer_poke',f'trainer_poke_{i:03d}.bin'),'rb').read()
    team=[struct.unpack_from('<HH',p,o+0x0A) for o in range(0,len(p),0x20)]
    rows.append({'id':i,'name':names[i] if i<len(names) else '?','class':cls,'cname':classes[cls] if cls<len(classes) else '?','mode':mode,'num':num,'ai':ai,'team':team})
json.dump(rows,open(sys.argv[3],'w'),ensure_ascii=False)
