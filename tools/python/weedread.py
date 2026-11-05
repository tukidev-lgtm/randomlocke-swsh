import struct,sys,math
def read(path):
    b=open(path,'rb').read()
    u32=lambda p:struct.unpack_from('<I',b,p)[0]
    def fields(t):
        vt=t-struct.unpack_from('<i',b,t)[0]; vs,ts=struct.unpack_from('<HH',b,vt)
        return [struct.unpack_from('<H',b,vt+4+2*i)[0] for i in range((vs-4)//2)]
    ref=lambda p:p+u32(p)
    def s(p): q=ref(p); return b[q+4:q+4+u32(q)].decode()
    def vec(p): q=ref(p); return [ref(q+4+4*k) for k in range(u32(q))]
    r=u32(0); f=fields(r)
    root={'f0':u32(r+f[0]),'bbox':struct.unpack_from('<6f',b,r+f[1]),'meshes':[]}
    for m in vec(r+f[2]):
        mf=fields(m); mesh={'name':s(m+mf[0]),'bbox':struct.unpack_from('<6f',b,m+mf[1]),'groups':[]}
        for g in vec(m+mf[2]):
            gf=fields(g)
            grp={'path':s(g+gf[0]),'model':s(g+gf[1]),'lbbox':struct.unpack_from('<6f',b,g+gf[2]),'wbbox':struct.unpack_from('<6f',b,g+gf[3]),'inst':[]}
            for it in vec(g+gf[4]):
                fi=fields(it)
                grp['inst'].append(tuple(struct.unpack_from('<3f',b,it+fi[k]) for k in range(3)))
            mesh['groups'].append(grp)
        root['meshes'].append(mesh)
    return root
if __name__=='__main__':
    r=read(sys.argv[1]); print(hex(r['f0']),r['bbox'])
    for m in r['meshes']:
        for g in m['groups']:
            P=[i[2] for i in g['inst']]
            nn=[min(math.dist(p,q) for q in P if q is not p) for p in P[:50]]
            sc=sorted(set(round(i[0][0],2) for i in g['inst']))
            print(m['name'],g['model'].split('/')[-1],len(P),'nn avg %.1f'%(sum(nn)/len(nn)),'scales',sc[:3],sc[-3:],'rot sample',g['inst'][0][1],'lbbox',[round(x) for x in g['lbbox']])
