package protocol

import "fmt"

func BuildLogoutPacket(config Config, session Session) []byte {
	username := []byte(config.Username)
	password := []byte(config.Password)
	packet := make([]byte, SizeLogout)

	packet[0] = 0x06
	packet[1] = 0x01
	packet[2] = 0x00
	packet[3] = byte(len(username) + 20)

	md5aPlain := make([]byte, 0, 2+4+len(password))
	md5aPlain = append(md5aPlain, 0x06, 0x01)
	md5aPlain = append(md5aPlain, session.LogoutSalt[:]...)
	md5aPlain = append(md5aPlain, password...)
	md5a := MD5(md5aPlain)
	copy(packet[4:20], md5a[:])

	copyFixed(packet[20:56], username)
	packet[56] = 0x00
	packet[57] = 0x00
	copy(packet[58:64], XOR(config.MAC[:], md5a[:], 6))
	copy(packet[64:80], session.ServerDrcomIndicator[:])
	return packet
}

func ParseLogoutResponse(packet []byte) error {
	if len(packet) == 0 {
		return fmt.Errorf("logout response is empty")
	}
	return nil
}
