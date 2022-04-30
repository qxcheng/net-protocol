package header

// Checksum 校验和的计算
func Checksum(buf []byte, initial uint16) uint16 {
	v := uint32(initial)

	l := len(buf)
	if l & 1 != 0 {
		l--
		v += uint32(buf[l]) << 8
	}

	for i := 0; i < l; i += 2 {
		v += (uint32(buf[i]) << 8) + uint32(buf[i+1])
	}

	return ChecksumCombine(uint16(v), uint16(v>>16))
}

func ChecksumCombine(a, b uint16) uint16 {
	v := uint32(a) + uint32(b)
	return uint16(v + v>>16)
}


