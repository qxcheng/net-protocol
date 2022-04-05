package buffer

type Prependable struct {
	buf View
	usedIdx int  // usedIdx 已使用的buffer的起始索引
}

func NewPrependable(size int) Prependable {
	return Prependable{buf: NewView(size), usedIdx: size}
}

func NewPrependableFromView(v View) Prependable {
	return Prependable{buf: v, usedIdx: 0}
}

// View 返回已使用的buf切片
func (p Prependable) View() View {
	return p.buf[p.usedIdx:]
}

// UsedLength 返回已使用的字节数
func (p Prependable) UsedLength() int {
	return len(p.buf) - p.usedIdx
}

// Prepend 在前面返回size大小的切片[p.usedIdx-size:p.usedIdx] 用于添加协议头
func (p *Prependable) Prepend(size int) []byte {
	if size > p.usedIdx {
		return nil
	}
	p.usedIdx -= size
	return p.View()[:size:size]
}

