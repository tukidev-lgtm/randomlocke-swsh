"""Genera hierba alta de encuentro para areas sin hierba (cuevas) en Pokemon Espada/Escudo."""
import struct, math, random, flatbuffers
from flatbuffers import number_types as N

LARGE='instance_obj_encountweed01/instance_obj_encountweed01_large01'
SMALL='instance_obj_encountweed01/instance_obj_encountweed01_small01'
LBB={'large':(-80.191,-5.0,-80.226,83.507,77.783,80.357),'small':(-58,-2,-61,58,56,58)}

def patch_instances(cx,y,cz,size,spacing,rng):
    pts=[]; half=size/2; n=int(size//spacing)+1
    for i in range(n):
        for j in range(n):
            x=cx-half+i*spacing+rng.uniform(-spacing*0.3,spacing*0.3)
            z=cz-half+j*spacing+rng.uniform(-spacing*0.3,spacing*0.3)
            pts.append(((1.0,1.0,1.0),(0.0,rng.uniform(0,2*math.pi),0.0),(x,y,z)))
    return pts

def bbox_of(insts,lb):
    xs=[p[2][0] for p in insts]; ys=[p[2][1] for p in insts]; zs=[p[2][2] for p in insts]
    return (min(xs)+lb[0],min(ys)+lb[1],min(zs)+lb[2],max(xs)+lb[3],max(ys)+lb[4],max(zs)+lb[5])

def merge(a,b): return tuple(min(a[i],b[i]) for i in range(3))+tuple(max(a[i],b[i]) for i in range(3,6))

def struct_floats(B,vals):
    B.Prep(4,4*len(vals))
    for v in reversed(vals): B.PrependFloat32(v)
    return B.Offset()

def build_gfbmdli(area,patches,seed=1):
    """patches: list of (cx,y,cz,size). Devuelve (bytes, nombres_de_mesh, bbox_total)"""
    rng=random.Random(seed); B=flatbuffers.Builder(1<<16)
    meshes=[]; names=[]; total=None
    for pi,(cx,y,cz,size) in enumerate(patches,1):
        for kind,model,sp in (('l',LARGE,45.0),('s',SMALL,60.0)):
            insts=patch_instances(cx,y,cz,size,sp,rng)
            lb=LBB['large' if kind=='l' else 'small']; wb=bbox_of(insts,lb)
            total=wb if total is None else merge(total,wb)
            name=f'{area}_coord_encountweed001_{pi:02d}_{kind}_ReproMesh'; names.append(name)
            path_s=B.CreateString(f'|{area}_coord|instance_obj|'+model.replace('/',':'))
            model_s=B.CreateString(model)
            io=[]
            for sc,rot,pos in insts:
                B.StartObject(3)
                B.PrependStructSlot(2,struct_floats(B,pos),0)
                B.PrependStructSlot(1,struct_floats(B,rot),0)
                B.PrependStructSlot(0,struct_floats(B,sc),0)
                io.append(B.EndObject())
            B.StartVector(4,len(io),4)
            for o in reversed(io): B.PrependUOffsetTRelative(o)
            iv=B.EndVector()
            B.StartObject(5)
            B.PrependUOffsetTRelativeSlot(4,iv,0)
            B.PrependStructSlot(3,struct_floats(B,wb),0)
            B.PrependStructSlot(2,struct_floats(B,lb),0)
            B.PrependUOffsetTRelativeSlot(1,model_s,0)
            B.PrependUOffsetTRelativeSlot(0,path_s,0)
            g=B.EndObject()
            B.StartVector(4,1,4); B.PrependUOffsetTRelative(g); gv=B.EndVector()
            ns=B.CreateString(name)
            B.StartObject(3)
            B.PrependUOffsetTRelativeSlot(2,gv,0)
            B.PrependStructSlot(1,struct_floats(B,wb),0)
            B.PrependUOffsetTRelativeSlot(0,ns,0)
            meshes.append(B.EndObject())
    B.StartVector(4,len(meshes),4)
    for o in reversed(meshes): B.PrependUOffsetTRelative(o)
    mv=B.EndVector()
    B.StartObject(3)
    B.PrependUOffsetTRelativeSlot(2,mv,0)
    B.PrependStructSlot(1,struct_floats(B,total),0)
    B.PrependUint32Slot(0,0x18072019,0)
    B.Finish(B.EndObject())
    return bytes(B.Output()),names,total

def _align(buf,n):
    while len(buf)%n: buf+=b'\0'

def add_terrain_entry(old,gfbmdli_path,mesh_names,bbox):
    """Anade una entrada de instancias al descriptor de terreno del area (sin tocar las existentes)."""
    u32=lambda p:struct.unpack_from('<I',old,p)[0]
    root=u32(0); vt=root-struct.unpack_from('<i',old,root)[0]
    f0=struct.unpack_from('<H',old,vt+4)[0]; vp=root+f0; vec=vp+u32(vp); n=u32(vec)
    elems=[vec+4+4*k+u32(vec+4+4*k) for k in range(n)]
    # cabecera nueva: [rootoff][vtable root][root table][vector n+1]
    hdr_len=4+8+8+4+4*(n+1)
    pad=(-(hdr_len-4))%16; hdr_len+=pad
    delta=hdr_len-4
    out=bytearray(hdr_len)
    vt_pos=4; tbl_pos=12; vec_pos=20
    struct.pack_into('<I',out,0,tbl_pos)
    struct.pack_into('<HHH',out,vt_pos,6,8,4)
    struct.pack_into('<i',out,tbl_pos,tbl_pos-vt_pos)
    struct.pack_into('<I',out,tbl_pos+4,vec_pos-(tbl_pos+4))
    struct.pack_into('<I',out,vec_pos,n+1)
    out+=old[4:]; _align(out,16)
    new_elem_pos=None
    # --- nueva entrada al final ---
    # wrapper: vtable(6,8,4) + table(soffset, uoffset->inner)
    wvt=len(out); out+=struct.pack('<HHH',6,8,4); _align(out,4)
    wt=len(out); out+=struct.pack('<iI',wt-wvt,0)
    # inner: vtable 22 bytes, table 60 bytes
    ivt=len(out); out+=struct.pack('<11H',22,60,4,8,12,16,20,24,28,52,56); _align(out,4)
    it=len(out); out+=bytearray(60); struct.pack_into('<i',out,it,it-ivt)
    struct.pack_into('<I',out,wt+4,it-(wt+4))
    struct.pack_into('<6f',out,it+28,*bbox)
    def put_string(s):
        _align(out,4); p=len(out); e=s.encode(); out.extend(struct.pack('<I',len(e))+e+b'\0'); return p
    strs={0:'',1:gfbmdli_path,2:'',3:'',4:'',5:'',7:'bin/field/model/instance_obj/'}
    slot={0:4,1:8,2:12,3:16,4:20,5:24,7:52}
    for k,s in strs.items():
        p=put_string(s); fp=it+slot[k]; struct.pack_into('<I',out,fp,p-fp)
    _align(out,4); vp2=len(out); out.extend(struct.pack('<I',len(mesh_names))+bytes(4*len(mesh_names)))
    for i,s in enumerate(mesh_names):
        p=put_string(s); fp=vp2+4+4*i; struct.pack_into('<I',out,fp,p-fp)
    fp=it+56; struct.pack_into('<I',out,fp,vp2-fp)
    # vector de la raiz: elementos antiguos desplazados + el nuevo
    for k,e in enumerate(elems):
        fp=vec_pos+4+4*k; struct.pack_into('<I',out,fp,(e+delta)-fp)
    fp=vec_pos+4+4*n; struct.pack_into('<I',out,fp,wt-fp)
    _align(out,16)
    return bytes(out)

def build_gfbmdli_from(area, fileid, patches):
    """patches: lista de dicts {model_l, model_s, lb_l, lb_s, inst_l:[pos], inst_s:[pos]}"""
    B=flatbuffers.Builder(1<<16); meshes=[]; names=[]; total=None; rng=random.Random(fileid)
    for pi,p in enumerate(patches,1):
        for kind in ('l','s'):
            pts=p['inst_'+kind]
            if not pts: continue
            model=p['model_'+kind]; lb=p['lb_'+kind]
            insts=[((1.0,1.0,1.0),(0.0,rng.uniform(0,2*math.pi),0.0),pos) for pos in pts]
            wb=bbox_of(insts,lb); total=wb if total is None else merge(total,wb)
            name=f'{area}_coord_encountweed{fileid:03d}_{pi:02d}_{kind}_ReproMesh'; names.append(name)
            path_s=B.CreateString(f'|{area}_coord|instance_obj|'+model.replace('/',':')); model_s=B.CreateString(model)
            io=[]
            for sc,rot,pos in insts:
                B.StartObject(3); B.PrependStructSlot(2,struct_floats(B,pos),0); B.PrependStructSlot(1,struct_floats(B,rot),0); B.PrependStructSlot(0,struct_floats(B,sc),0); io.append(B.EndObject())
            B.StartVector(4,len(io),4)
            for o in reversed(io): B.PrependUOffsetTRelative(o)
            iv=B.EndVector()
            B.StartObject(5); B.PrependUOffsetTRelativeSlot(4,iv,0); B.PrependStructSlot(3,struct_floats(B,wb),0); B.PrependStructSlot(2,struct_floats(B,lb),0)
            B.PrependUOffsetTRelativeSlot(1,model_s,0); B.PrependUOffsetTRelativeSlot(0,path_s,0); g=B.EndObject()
            B.StartVector(4,1,4); B.PrependUOffsetTRelative(g); gv=B.EndVector(); ns=B.CreateString(name)
            B.StartObject(3); B.PrependUOffsetTRelativeSlot(2,gv,0); B.PrependStructSlot(1,struct_floats(B,wb),0); B.PrependUOffsetTRelativeSlot(0,ns,0); meshes.append(B.EndObject())
    B.StartVector(4,len(meshes),4)
    for o in reversed(meshes): B.PrependUOffsetTRelative(o)
    mv=B.EndVector()
    B.StartObject(3); B.PrependUOffsetTRelativeSlot(2,mv,0); B.PrependStructSlot(1,struct_floats(B,total),0); B.PrependUint32Slot(0,0x18072019,0)
    B.Finish(B.EndObject())
    return bytes(B.Output()),names,total
