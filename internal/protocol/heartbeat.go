package protocol

import (
	"fmt"
	"io"
	"time"
)

func BuildKeepAliveAuth(session Session, now time.Time) []byte {
	packet := make([]byte, SizeKeepAliveAuth)
	packet[0] = 0xff
	copy(packet[1:17], session.MD5Password[:])
	copy(packet[20:36], session.ServerDrcomIndicator[:])

	ts := now.Unix()
	packet[36] = byte(ts % 256)
	packet[37] = byte((ts / 256) % 256)
	return packet
}

func ParseKeepAliveAuthResponse(packet []byte) error {
	if len(packet) == 0 {
		return fmt.Errorf("keepalive auth response is empty")
	}
	return nil
}

func BuildFirstHeartbeat(config Config, session Session, rng io.Reader) []byte {
	packet := buildHeartbeatBase(session, 0x01, config.FirstHeartbeatVersion, rng)
	return packet
}

func BuildExtraHeartbeat(config Config, session Session, rng io.Reader) []byte {
	packet := buildHeartbeatBase(session, 0x01, config.ExtraHeartbeatVersion, rng)
	copy(packet[16:20], session.HeartbeatToken[:])
	return packet
}

func BuildHeartbeatStep1(config Config, session Session, rng io.Reader) []byte {
	packet := buildHeartbeatBase(session, 0x01, config.KeepAliveVersion, rng)
	copy(packet[16:20], session.HeartbeatToken[:])
	return packet
}

func ParseHeartbeatStep1Response(packet []byte, session *Session) error {
	if len(packet) < 20 {
		return fmt.Errorf("heartbeat step1 response too short: got %d bytes", len(packet))
	}
	copy(session.HeartbeatToken[:], packet[16:20])
	return nil
}

func ParseHeartbeatAck(packet []byte) error {
	if len(packet) == 0 {
		return fmt.Errorf("heartbeat response is empty")
	}
	return nil
}

func BuildHeartbeatStep2(config Config, session Session, randomToken [4]byte) []byte {
	packet := make([]byte, SizeKeepAliveHeartbeat)
	packet[0] = 0x07
	packet[1] = byte(session.HeartbeatCount & 0xff)
	packet[2] = 0x28
	packet[3] = 0x00
	packet[4] = 0x0b
	packet[5] = 0x03
	copy(packet[6:8], config.KeepAliveVersion[:])
	copy(packet[8:12], randomToken[:])
	copy(packet[16:20], session.HeartbeatToken[:])

	crcPlain := make([]byte, 0, 28)
	crcPlain = append(crcPlain, packet[:24]...)
	crcPlain = append(crcPlain, config.IP[:]...)
	crc := CRC(crcPlain)
	copy(packet[24:28], crc[:])
	copy(packet[28:32], config.IP[:])
	return packet
}

func buildHeartbeatBase(session Session, phase byte, version [2]byte, rng io.Reader) []byte {
	packet := make([]byte, SizeKeepAliveHeartbeat)
	packet[0] = 0x07
	packet[1] = byte(session.HeartbeatCount & 0xff)
	packet[2] = 0x28
	packet[3] = 0x00
	packet[4] = 0x0b
	packet[5] = phase
	copy(packet[6:8], version[:])
	mustReadRandom(rng, packet[8:12])
	return packet
}
