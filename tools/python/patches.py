"""Parches de hierba: cluster de spawners -> cuadrados; recorte de triangulos de colision."""
import struct, math
def clusters(points, merge=450.0):
    cl=[]
    for p in points:
        for c in cl:
            if math.hypot(p[0]-c['x'],p[2]-c['z'])<merge and abs(p[1]-c['y'])<200:
                c['pts'].append(p); n=len(c['pts'])
                c['x']=sum(q[0] for q in c['pts'])/n; c['z']=sum(q[2] for q in c['pts'])/n; c['y']=sum(q[1] for q in c['pts'])/n
                break
        else: cl.append({'x':p[0],'y':p[1],'z':p[2],'pts':[p]})
    return cl
def _split(poly, axis, val, keep_less):
    """Sutherland-Hodgman de un poligono 3D contra el plano axis=val (axis 0=x, 2=z)."""
    out=[]; n=len(poly)
    for i in range(n):
        a=poly[i]; b=poly[(i+1)%n]
        ina=(a[axis]<=val) if keep_less else (a[axis]>=val)
        inb=(b[axis]<=val) if keep_less else (b[axis]>=val)
        if ina: out.append(a)
        if ina!=inb:
            t=(val-a[axis])/(b[axis]-a[axis]); out.append(tuple(a[k]+(b[k]-a[k])*t for k in range(3)))
    return out
def cut_triangle(tri, sq):
    """sq=(x0,z0,x1,z1). Devuelve (piezas_dentro, piezas_fuera) como poligonos."""
    x0,z0,x1,z1=sq; inside=[]; outside=[]; cur=[list(tri)]
    for axis,val,less in ((0,x0,False),(0,x1,True),(2,z0,False),(2,z1,True)):
        nxt=[]
        for poly in cur:
            a=_split(poly,axis,val,less); b=_split(poly,axis,val,not less)
            if len(b)>=3: outside.append(b)
            if len(a)>=3: nxt.append(a)
        cur=nxt
    inside=cur
    return inside,outside
def fan(poly): return [(poly[0],poly[i],poly[i+1]) for i in range(1,len(poly)-1)]
def area2(t):
    (ax,_,az),(bx,_,bz),(cx,_,cz)=t; return abs((bx-ax)*(cz-az)-(cx-ax)*(bz-az))
def apply_patches(col, squares, target_mat, skip_mats, ylo, yhi):
    """squares: lista de (x0,z0,x1,z1,yref). Convierte el suelo dentro de los cuadrados a target_mat."""
    if target_mat not in col['mats']: col['mats'].append(target_mat)
    tk=col['mats'].index(target_mat); changed=0
    for m in col['meshes']:
        V=list(struct.unpack('<%df'%(len(m['verts'])//4),m['verts'])); nv0=len(V)//3
        newsubs={}
        def addv(p):
            V.extend(p); return len(V)//3-1
        for s in m['subs']:
            mat=col['mats'][s['mat']]
            if mat in skip_mats: continue
            I=s['idx']; keep=[]
            for t in range(0,len(I)-2,3):
                ids=I[t:t+3]; tri=[tuple(V[3*j:3*j+3]) for j in ids]
                xs=[p[0] for p in tri]; zs=[p[2] for p in tri]; ys=[p[1] for p in tri]
                hit=None
                for (x0,z0,x1,z1,yr) in squares:
                    if max(xs)<x0 or min(xs)>x1 or max(zs)<z0 or min(zs)>z1: continue
                    if min(ys)>yr+ylo or max(ys)<yr-yhi: continue
                    hit=(x0,z0,x1,z1); break
                if not hit: keep.extend(ids); continue
                ins,outs=cut_triangle(tri,hit)
                if not ins: keep.extend(ids); continue
                if not outs:
                    newsubs.setdefault(tk,[]).extend(ids); changed+=1; continue
                for poly in outs:
                    for tr in fan(poly):
                        if area2(tr)<1e-3: continue
                        keep.extend(addv(p) for p in tr)
                for poly in ins:
                    for tr in fan(poly):
                        if area2(tr)<1e-3: continue
                        newsubs.setdefault(tk,[]).extend(addv(p) for p in tr)
                changed+=1
            s['idx']=keep
        for k,idx in newsubs.items():
            tgt=[s for s in m['subs'] if s['mat']==k]
            if tgt: tgt[0]['idx']+=idx
            else: m['subs'].append({'mat':k,'idx':idx})
        m['subs']=[s for s in m['subs'] if s['idx']]
        assert len(V)//3<65536, 'demasiados vertices'
        m['verts']=struct.pack('<%df'%len(V),*V)
    return changed
