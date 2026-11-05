"""Lectura/escritura completa de colisiones .gfbcol de SwSh (esquema deducido)."""
import struct, flatbuffers
from gfpak import FB
def _fields(fb,t):
    vt=t-struct.unpack_from('<i',fb.b,t)[0]; vs,ts=struct.unpack_from('<HH',fb.b,vt)
    return [struct.unpack_from('<H',fb.b,vt+4+2*i)[0] for i in range((vs-4)//2)]
def _scalars(fb,t):
    return {i:struct.unpack_from('<I',fb.b,t+o)[0] for i,o in enumerate(_fields(fb,t)) if o}
def parse(data):
    fb=FB(bytearray(data)); r=fb.root(); rf=_fields(fb,r); assert len(rf)<=4, rf
    col={'mats':[fb.string(m,0) for m in fb.vector(r,0)], 'chunks':[], 'meshes':[]}
    for c in fb.vector(r,1):
        f=_fields(fb,c); assert len(f)<=2
        col['chunks'].append({'f0':fb.scalar(c,0,'I'),'bbox':struct.unpack_from('<6f',fb.b,c+f[1])})
    for m in fb.vector(r,2):
        f=_fields(fb,m); assert len(f)<=3, f
        subs=[]
        for s in fb.vector(m,0):
            sf=_fields(fb,s); assert len(sf)<=2
            q=fb.field_pos(s,1); iv=q+fb.u32(q); k=fb.u32(iv)
            subs.append({'mat':fb.scalar(s,0,'I'),'idx':list(struct.unpack_from('<%dH'%k,fb.b,iv+4))})
        flags=[]
        for g in fb.vector(m,1):
            sc=_scalars(fb,g); flags.append(sc)
        p=fb.field_pos(m,2); v=p+fb.u32(p); n=fb.u32(v)
        col['meshes'].append({'subs':subs,'flags':flags,'verts':bytes(fb.b[v+4:v+4+n])})
    t=fb.table(r,3); tf=_fields(fb,t); assert len(tf)<=2
    col['root2']={'bbox':struct.unpack_from('<6f',fb.b,t+tf[0]),'f1':fb.scalar(t,1,'I')}
    return col
def _bbox(B,v):
    B.Prep(4,24)
    for x in reversed(v): B.PrependFloat32(x)
    return B.Offset()
def _vec(B,offs):
    B.StartVector(4,len(offs),4)
    for o in reversed(offs): B.PrependUOffsetTRelative(o)
    return B.EndVector()
def build(col):
    B=flatbuffers.Builder(1<<20)
    mats=[]
    for name in col['mats']:
        s=B.CreateString(name); B.StartObject(1); B.PrependUOffsetTRelativeSlot(0,s,0); mats.append(B.EndObject())
    chunks=[]
    for c in col['chunks']:
        B.StartObject(2); B.PrependStructSlot(1,_bbox(B,c['bbox']),0); B.PrependUint32Slot(0,c['f0'],0); chunks.append(B.EndObject())
    meshes=[]
    for m in col['meshes']:
        B.StartVector(1,len(m['verts']),4)
        for x in reversed(m['verts']): B.PrependUint8(x)
        vv=B.EndVector()
        fl=[]
        for g in m['flags']:
            n=max(g)+1 if g else 0
            B.StartObject(n)
            for i,val in g.items(): B.PrependUint32Slot(i,val,0xFFFFFFFF)
            fl.append(B.EndObject())
        flv=_vec(B,fl)
        subs=[]
        for s in m['subs']:
            B.StartVector(2,len(s['idx']),4)
            for x in reversed(s['idx']): B.PrependUint16(x)
            iv=B.EndVector()
            B.StartObject(2); B.PrependUOffsetTRelativeSlot(1,iv,0); B.PrependUint32Slot(0,s['mat'],0); subs.append(B.EndObject())
        sv=_vec(B,subs)
        B.StartObject(3); B.PrependUOffsetTRelativeSlot(2,vv,0); B.PrependUOffsetTRelativeSlot(1,flv,0); B.PrependUOffsetTRelativeSlot(0,sv,0); meshes.append(B.EndObject())
    mv=_vec(B,meshes); cv=_vec(B,chunks); matv=_vec(B,mats)
    B.StartObject(2); B.PrependUint32Slot(1,col['root2']['f1'],0); B.PrependStructSlot(0,_bbox(B,col['root2']['bbox']),0); r2=B.EndObject()
    B.StartObject(4); B.PrependUOffsetTRelativeSlot(3,r2,0); B.PrependUOffsetTRelativeSlot(2,mv,0); B.PrependUOffsetTRelativeSlot(1,cv,0); B.PrependUOffsetTRelativeSlot(0,matv,0)
    B.Finish(B.EndObject())
    return bytes(B.Output())
