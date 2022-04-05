package buffer

type View []byte

func NewView(size int) View {
	return make(View, size)
}

func NewViewFromBytes(b []byte) View {
	return append(View(nil), b...)
}

// TrimFront 移除前count个字节
func (v *View) TrimFront(count int) {
	*v = (*v)[count:]
}

// CapLength 减小切片长度至length
func (v *View) CapLength(length int) {
	*v = (*v)[:length:length]
}

func (v View) ToVectorisedView() VectorisedView {
	return NewVectorisedView(len(v), []View{v})
}


// VectorisedView is a vectorised version of View using non contigous memory.
type VectorisedView struct {
	views []View  // 所有切片
	size int      // 所有切片的总字节数
}

func NewVectorisedView(size int, views []View) VectorisedView {
	return VectorisedView{views: views, size: size}
}

// TrimFront 移除前count个字节
func (vv *VectorisedView) TrimFront(count int) {
	for count > 0 && len(vv.views) > 0 {
		if count < len(vv.views[0]) {
			vv.size -= count
			vv.views[0].TrimFront(count)
			return
		}
		count -= len(vv.views[0])
		vv.RemoveFirst()
	}
}

// CapLength 减小切片长度至length
func (vv *VectorisedView) CapLength(length int) {
	if length < 0 {
		length = 0
	}
	if vv.size < length {
		return
	}
	vv.size = length
	for i := range vv.views {
		v := &vv.views[i]
		if len(*v) >= length {
			if length == 0 {
				vv.views = vv.views[:i]
			} else {
				v.CapLength(length)
				vv.views = vv.views[:i+1]
			}
			return
		}
		length -= len(*v)
	}
}

// RemoveFirst 移除第0个View元素
func (vv *VectorisedView) RemoveFirst() {
	if len(vv.views) == 0 {
		return
	}
	vv.size -= len(vv.views[0])
	vv.views = vv.views[1:]
}

// Clone 复制VectorisedView
func (vv VectorisedView) Clone(buffer []View) VectorisedView {
	return VectorisedView{views: append(buffer[:0], vv.views...), size: vv.size}
}

// First 返回第一个View
func (vv VectorisedView) First() View {
	if len(vv.views) == 0 {
		return nil
	}
	return vv.views[0]
}

// Size 返回VectorisedView的总字节数
func (vv VectorisedView) Size() int {
	return vv.size
}

// ToView 将VectorisedView平铺到一个View
func (vv VectorisedView) ToView() View {
	u := make([]byte, 0, vv.size)
	for _, v := range vv.views {
		u = append(u, v...)
	}
	return u
}

// Views 返回views
func (vv VectorisedView) Views() []View {
	return vv.views
}




