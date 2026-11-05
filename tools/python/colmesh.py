import struct
from gfpak import FB
class Col:
    def __init__(self,data):
        self.fb=fb=FB(bytearray(data)); r=fb.root()
        self.mats=[fb.string(m,0) for m in fb.vector(r,0)]
        self.chunks=[]
        bbs=fb.vector(r,1)
        for ci,m in enumerate(fb.vector(r,2)):
            p=fb.field_pos(m,2); v=p+fb.u32(p); n=fb.u32(v)
            verts=struct.unpack_from('<%df'%(n//4),fb.b,v+4)
            V=[verts[i:i+3] for i in range(0,len(verts),3)]
            subs=[]
            for s in fb.vector(m,0):
                mat=fb.scalar(s,0,'I'); q=fb.field_pos(s,1); iv=q+fb.u32(q); k=fb.u32(iv)
                idx=struct.unpack_from('<%dH'%k,fb.b,iv+4)
                subs.append({'tbl':s,'mat':mat,'idx':idx})
            bb=None
            if ci<len(bbs):
                pp=self.fb.field_pos(bbs[ci],1)
                if pp: bb=struct.unpack_from('<6f',fb.b,pp)
            self.chunks.append({'tbl':m,'V':V,'subs':subs,'bbox':bb})
    def tri_centroids(self,ch,sub):
        V=ch['V'];I=sub['idx']
        return [tuple((V[I[t]][k]+V[I[t+1]][k]+V[I[t+2]][k])/3 for k in range(3)) for t in range(0,len(I)-2,3)]
