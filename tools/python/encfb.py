import struct, flatbuffers
from gfpak import FB
def parse(data):
    fb=FB(bytearray(data)); r=fb.root()
    arc={'f0':fb.scalar(r,0,'I'),'tables':[]}
    for t in fb.vector(r,1):
        subs=[]
        for s in fb.vector(t,1):
            subs.append({'min':fb.scalar(s,0,'B'),'max':fb.scalar(s,1,'B'),
                         'slots':[(fb.scalar(x,0,'B'),fb.scalar(x,1,'i'),fb.scalar(x,2,'B')) for x in fb.vector(s,2)]})
        arc['tables'].append({'zone':fb.scalar(t,0,'Q'),'subs':subs})
    return arc
def build(arc):
    B=flatbuffers.Builder(1<<18)
    def vec(o):
        B.StartVector(4,len(o),4)
        for x in reversed(o): B.PrependUOffsetTRelative(x)
        return B.EndVector()
    tabs=[]
    for t in arc['tables']:
        subs=[]
        for s in t['subs']:
            sl=[]
            for p,sp,fm in s['slots']:
                B.StartObject(3); B.PrependInt32Slot(1,sp,0); B.PrependUint8Slot(2,fm,0); B.PrependUint8Slot(0,p,0); sl.append(B.EndObject())
            sv=vec(sl)
            B.StartObject(3); B.PrependUOffsetTRelativeSlot(2,sv,0); B.PrependUint8Slot(1,s['max'],0); B.PrependUint8Slot(0,s['min'],0); subs.append(B.EndObject())
        v=vec(subs)
        B.StartObject(2); B.PrependUOffsetTRelativeSlot(1,v,0); B.PrependUint64Slot(0,t['zone'],0); tabs.append(B.EndObject())
    tv=vec(tabs)
    B.StartObject(2); B.PrependUOffsetTRelativeSlot(1,tv,0); B.PrependUint32Slot(0,arc['f0'],0); B.Finish(B.EndObject())
    return bytes(B.Output())
