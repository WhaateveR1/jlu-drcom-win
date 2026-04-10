package protocol

import (
	"crypto/md5"
	"encoding/binary"
)

func MD5(data []byte) [16]byte {
	return md5.Sum(data)
}

func XOR(a []byte, b []byte, outLen int) []byte {
	if outLen < 0 {
		outLen = 0
	}
	out := make([]byte, outLen)
	n := min(len(a), len(b), outLen)
	for i := 0; i < n; i++ {
		out[i] = a[i] ^ b[i]
	}
	return out
}

func ROR(data []byte) []byte {
	out := make([]byte, len(data))
	for i, b := range data {
		out[i] = byte((b << 3) + (b >> 5))
	}
	return out
}

func Checksum(data []byte) [4]byte {
	var sum uint32 = 1234
	for i := 0; i < len(data); i += 4 {
		var chunk [4]byte
		copy(chunk[:], data[i:min(i+4, len(data))])
		sum ^= binary.LittleEndian.Uint32(chunk[:])
	}
	sum = 1968 * sum

	var out [4]byte
	binary.LittleEndian.PutUint32(out[:], sum)
	return out
}

func CRC(data []byte) [4]byte {
	var sum uint32
	for i := 0; i < len(data); i += 2 {
		var chunk [2]byte
		copy(chunk[:], data[i:min(i+2, len(data))])
		sum ^= uint32(binary.LittleEndian.Uint16(chunk[:]))
	}
	sum *= 711

	var out [4]byte
	binary.LittleEndian.PutUint32(out[:], sum)
	return out
}
