import sys,struct; sys.path.insert(0,'.')
from gfpak import *
def rename_materials(colbytes, old, new):
    b=bytearray(colbytes); fb=FB(b); n=0
    for m in fb.vector(fb.root(),0):
        p=fb.field_pos(m,0); s=p+fb.u32(p); ln=fb.u32(s)
        if bytes(b[s+4:s+4+ln]).decode()==old:
            e=new.encode(); assert len(e)<=ln
            struct.pack_into('<I',b,s,len(e)); b[s+4:s+4+ln+1]=e+b'\0'*(ln-len(e)+1); n+=1
    return bytes(b),n
if __name__=='__main__':
    src,dst=sys.argv[1],sys.argv[2]
    pk=GFPack(open(src,'rb').read()); tot=0
    for i,f in enumerate(pk.files):
        if b'col_tsuchi' in f:
            nb,n=rename_materials(bytes(f),'col_tsuchi','col_kusa'); pk.files[i]=bytearray(nb); tot+=n
    open(dst,'wb').write(pk.write()); print(src.split('/')[-1],'renamed',tot)
