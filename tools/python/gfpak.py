import struct, lz4.block, zlib
MAGIC=0x4B434150584C4647
def fnv1a64(s):
    h=0xCBF29CE484222645
    for b in s.encode():
        h^=b; h=(h*0x100000001B3)&0xFFFFFFFFFFFFFFFF
    return h
class GFPack:
    def __init__(self,data):
        self.magic,self.version,self.reloc,nf,nd=struct.unpack_from('<QIIii',data,0)
        assert self.magic==MAGIC
        p=0x18
        self.ptr_ft,self.ptr_hp=struct.unpack_from('<qq',data,p); p+=16
        self.ptr_folders=list(struct.unpack_from('<%dq'%nd,data,p)); p+=8*nd
        self.hash_abs=list(struct.unpack_from('<%dQ'%nf,data,self.ptr_hp))
        self.folders=[]
        for fp in self.ptr_folders:
            h,cnt,pad=struct.unpack_from('<QiI',data,fp)
            files=[struct.unpack_from('<QiI',data,fp+16+16*i) for i in range(cnt)]
            self.folders.append([(h,cnt,pad),files])
        self.ft=[list(struct.unpack_from('<HBBiiiiI',data,self.ptr_ft+0x18*i)) for i in range(nf)]
        # fields: level, type, pad?, sizeDec, sizeComp, padding, offset, unused
        self.files=[]
        for e in self.ft:
            lvl,typ,_b,sd,sc,pad,off,un=e
            c=data[off:off+sc]
            if typ==0: d=c
            elif typ==1: d=zlib.decompress(c)
            elif typ==2: d=lz4.block.decompress(c,uncompressed_size=sd)
            else: raise Exception('compression %d'%typ)
            self.files.append(bytearray(d))
    def index_of_name(self,name):
        h=fnv1a64(name)
        for _,files in self.folders:
            for fh,idx,_ in files:
                if fh==h: return idx
        return -1
    def write(self):
        nf=len(self.files); nd=len(self.folders)
        out=bytearray()
        def header():
            b=bytearray(struct.pack('<QIIii',MAGIC,self.version,self.reloc,nf,nd))
            b+=struct.pack('<qq',self.ptr_ft,self.ptr_hp)+struct.pack('<%dq'%nd,*self.ptr_folders)
            hp=len(b); b+=struct.pack('<%dQ'%nf,*self.hash_abs)
            pf=[]
            for (h,cnt,pad),files in self.folders:
                pf.append(len(b)); b+=struct.pack('<QiI',h,len(files),pad)
                for f in files: b+=struct.pack('<QiI',*f)
            pft=len(b)
            for e in self.ft: b+=struct.pack('<HBBiiiiI',*e)
            return b,hp,pf,pft
        b,hp,pf,pft=header()
        self.ptr_hp,self.ptr_folders,self.ptr_ft=hp,pf,pft
        b,_,_,_=header()
        out=bytearray(b)
        for i,d in enumerate(self.files):
            e=self.ft[i]; typ=e[1]
            if typ==0: c=bytes(d)
            elif typ==1: c=zlib.compress(bytes(d),9)
            else: c=lz4.block.compress(bytes(d),mode='high_compression',store_size=False)
            e[3]=len(d); e[4]=len(c); e[6]=len(out)
            out+=c
            while len(out)%16: out+=b'\0'
        b,_,_,_=header()
        out[:len(b)]=b
        return bytes(out)

# minimal flatbuffer navigation over a bytearray
class FB:
    def __init__(self,buf): self.b=buf
    def u32(self,o): return struct.unpack_from('<I',self.b,o)[0]
    def root(self): return self.u32(0)
    def field_pos(self,t,i):
        vt=t-struct.unpack_from('<i',self.b,t)[0]
        vsz=struct.unpack_from('<H',self.b,vt)[0]
        if 4+2*i>=vsz: return None
        off=struct.unpack_from('<H',self.b,vt+4+2*i)[0]
        return t+off if off else None
    def table(self,t,i):
        p=self.field_pos(t,i); return None if p is None else p+self.u32(p)
    def vector(self,t,i):
        p=self.field_pos(t,i)
        if p is None: return []
        v=p+self.u32(p); n=self.u32(v)
        return [v+4+4*k+self.u32(v+4+4*k) for k in range(n)]
    def scalar(self,t,i,fmt,default=0):
        p=self.field_pos(t,i); return default if p is None else struct.unpack_from('<'+fmt,self.b,p)[0]
    def set_scalar(self,t,i,fmt,val):
        p=self.field_pos(t,i)
        if p is None: return False
        struct.pack_into('<'+fmt,self.b,p,val); return True
    def string(self,t,i):
        p=self.field_pos(t,i)
        if p is None: return ''
        s=p+self.u32(p); n=self.u32(s); return bytes(self.b[s+4:s+4+n]).decode('utf-8','replace')
