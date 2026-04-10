package protocol

import (
	"fmt"
	"io"
)

func BuildLoginChallenge(authVersion [2]byte, rng io.Reader) []byte {
	packet := make([]byte, SizeChallenge)
	packet[0] = 0x01
	packet[1] = 0x02
	mustReadRandom(rng, packet[2:4])
	copy(packet[4:6], authVersion[:])
	return packet
}

func ParseLoginChallengeResponse(packet []byte) ([4]byte, error) {
	return parseChallengeResponse(packet, 0x02)
}

func BuildLogoutChallenge(authVersion [2]byte, rng io.Reader) []byte {
	packet := make([]byte, SizeChallenge)
	packet[0] = 0x01
	packet[1] = 0x03
	mustReadRandom(rng, packet[2:4])
	copy(packet[4:6], authVersion[:])
	return packet
}

func ParseLogoutChallengeResponse(packet []byte) ([4]byte, error) {
	return parseChallengeResponse(packet, 0x03)
}

func parseChallengeResponse(packet []byte, subtype byte) ([4]byte, error) {
	var salt [4]byte
	if len(packet) < 8 {
		return salt, fmt.Errorf("challenge response too short: got %d bytes", len(packet))
	}
	if packet[0] != 0x02 || packet[1] != subtype {
		return salt, fmt.Errorf("unexpected challenge response type: got %02x %02x", packet[0], packet[1])
	}
	copy(salt[:], packet[4:8])
	return salt, nil
}

func mustReadRandom(rng io.Reader, dst []byte) {
	if rng == nil {
		panic("protocol: nil random reader")
	}
	if _, err := io.ReadFull(rng, dst); err != nil {
		panic(fmt.Sprintf("protocol: random reader failed: %v", err))
	}
}
