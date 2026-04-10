package logging

import "encoding/hex"

type ByteRange struct {
	Start int
	End   int
}

func HexDump(packet []byte) string {
	return hex.Dump(packet)
}

func HexDumpRedacted(packet []byte, ranges ...ByteRange) string {
	copyPacket := append([]byte(nil), packet...)
	for _, r := range ranges {
		start := max(0, r.Start)
		end := min(len(copyPacket), r.End)
		if start >= end {
			continue
		}
		for i := start; i < end; i++ {
			copyPacket[i] = 0x00
		}
	}
	return hex.Dump(copyPacket)
}
