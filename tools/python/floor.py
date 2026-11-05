import struct,collections
import colfb
class Floor:
    """Indice espacial simple de los triangulos de suelo de varias colisiones."""
    def __init__(self,cols,cell=500.0):
        self.cell=cell; self.grid=collections.defaultdict(list)
        for col in cols:
            for m in col['meshes']:
                V=struct.unpack('<%df'%(len(m['verts'])//4),m['verts'])
                for s in m['subs']:
                    mat=col['mats'][s['mat']]; I=s['idx']
                    for t in range(0,len(I)-2,3):
                        tri=[(V[3*j],V[3*j+1],V[3*j+2]) for j in I[t:t+3]]
                        xs=[p[0] for p in tri]; zs=[p[2] for p in tri]
                        for gx in range(int(min(xs)//cell),int(max(xs)//cell)+1):
                            for gz in range(int(min(zs)//cell),int(max(zs)//cell)+1):
                                self.grid[(gx,gz)].append((tri,mat))
    def query(self,x,z,ynear=None,dy=400):
        """Devuelve [(y,material)] de los triangulos bajo (x,z)."""
        out=[]
        for tri,mat in self.grid.get((int(x//self.cell),int(z//self.cell)),[]):
            (x1,y1,z1),(x2,y2,z2),(x3,y3,z3)=tri
            d=(z2-z3)*(x1-x3)+(x3-x2)*(z1-z3)
            if abs(d)<1e-9: continue
            a=((z2-z3)*(x-x3)+(x3-x2)*(z-z3))/d; b=((z3-z1)*(x-x3)+(x1-x3)*(z-z3))/d; c=1-a-b
            if a>=-1e-6 and b>=-1e-6 and c>=-1e-6:
                y=a*y1+b*y2+c*y3
                if ynear is None or abs(y-ynear)<=dy: out.append((y,mat))
        return out
