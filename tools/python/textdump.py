import struct,sys,os,glob
def lines(path):
    d=open(path,'rb').read(); n=struct.unpack_from('<H',d,2)[0]; sdo=struct.unpack_from('<I',d,0xC)[0]
    out=[]; key=0x7C89
    for i in range(n):
        off,ln,fl=struct.unpack_from('<iHH',d,sdo+4+8*i); k=key; ch=[]
        for j in range(ln):
            c=struct.unpack_from('<H',d,sdo+off+2*j)[0]^k; k=((k<<3)|(k>>13))&0xFFFF; ch.append(c)
        s=[]; j=0
        while j<len(ch):
            c=ch[j]
            if c==0: break
            if c==0x10:
                cnt=ch[j+1]; var=ch[j+2]; args=ch[j+3:j+2+cnt]
                s.append('[VAR %04X%s]'%(var,(':'+','.join('%X'%a for a in args)) if args else '')); j+=2+cnt; continue
            if c==0x0A: s.append('\\n')
            elif c==0x0D: s.append('\\r')
            elif c<0x20: s.append('[%02X]'%c)
            else: s.append(chr(c))
            j+=1
        out.append(''.join(s)); key=(key+0x2983)&0xFFFF
    return out
if __name__=='__main__':
    for f in sys.argv[1:]:
        print('=====',os.path.basename(f))
        for i,l in enumerate(lines(f)): print(f'{i:03d}: {l}')
