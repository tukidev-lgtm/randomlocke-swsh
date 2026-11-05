import struct,sys
b=open(sys.argv[1],'rb').read()
u32=lambda p:struct.unpack_from('<I',b,p)[0]
def fields(t):
    vt=t-struct.unpack_from('<i',b,t)[0]; vs,ts=struct.unpack_from('<HH',b,vt)
    return [struct.unpack_from('<H',b,vt+4+2*i)[0] for i in range((vs-4)//2)],ts
def looks_table(q):
    try:
        so=struct.unpack_from('<i',b,q)[0]; vt=q-so
        if not (0<=vt<len(b)-4): return False
        vs,ts=struct.unpack_from('<HH',b,vt); return 4<=vs<80 and vs%2==0 and 4<=ts<1024
    except Exception: return False
def isstr(q):
    if q+4>len(b): return None
    n=u32(q)
    if 0<n<400 and q+4+n<len(b) and all(31<c<127 for c in b[q+4:q+4+n]) and b[q+4+n]==0: return b[q+4:q+4+n].decode()
def dump(e,d,maxvec):
    f,ts=fields(e); ind='  '*d
    order=sorted([(o,i) for i,o in enumerate(f) if o])
    print(ind+f'{{table@{e} nf={len(f)}')
    for j,(o,i) in enumerate(order):
        p=e+o; w=(order[j+1][0] if j+1<len(order) else ts)-o
        if w>=4 and w%4==0 and p+4<=len(b):
            q=p+u32(p)
            if q<len(b):
                s=isstr(q)
                if s is not None and w==4: print(ind+f' f{i}: "{s}"'); continue
                n=u32(q) if q+4<=len(b) else 0
                if w==4 and 0<=n<20000 and q+4+4*n<=len(b) and (n==0 or looks_table(q+4+u32(q+4)) or isstr(q+4+u32(q+4)) is not None):
                    print(ind+f' f{i}: [vec {n}]')
                    for k in range(min(n,maxvec)):
                        ee=q+4+4*k+u32(q+4+4*k); s2=isstr(ee)
                        if s2 is not None: print(ind+f'   "{s2}"')
                        else: dump(ee,d+2,maxvec)
                    if n>maxvec: print(ind+'   ...')
                    continue
                if w==4 and looks_table(q): print(ind+f' f{i}:'); dump(q,d+1,maxvec); continue
        vals=[round(x,3) for x in struct.unpack_from('<%df'%(w//4),b,p)] if w%4==0 and w>0 else []
        print(ind+f' f{i} raw[{w}]={b[p:p+min(w,16)].hex()} f={vals[:8]}')
    print(ind+'}')
dump(u32(0),0,int(sys.argv[2]) if len(sys.argv)>2 else 3)
