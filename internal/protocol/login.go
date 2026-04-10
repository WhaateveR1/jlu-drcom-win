package protocol

import (
	"fmt"
	"io"
)

var drcomIndicator = []byte{0x44, 0x72, 0x43, 0x4f, 0x4d, 0x00, 0xcf, 0x07}

var unknownLoginIndicator = []byte{
	0x66, 0x34, 0x37, 0x64, 0x62, 0x62, 0x35, 0x39,
	0x63, 0x33, 0x34, 0x35, 0x39, 0x30, 0x31, 0x62,
	0x34, 0x33, 0x31, 0x31, 0x39, 0x32, 0x62, 0x62,
	0x31, 0x66, 0x62, 0x66, 0x63, 0x66, 0x64, 0x33,
	0x33, 0x66, 0x34, 0x33, 0x34, 0x32, 0x31, 0x31,
}

func BuildLoginPacket(config Config, session *Session, rng io.Reader) []byte {
	username := []byte(config.Username)
	password := []byte(config.Password)
	rorLen := min(len(password), 16)
	loginSize := 334
	if len(password) > 0 {
		loginSize += ((len(password) - 1) / 4) * 4
	}

	packet := make([]byte, loginSize)
	packet[0] = 0x03
	packet[1] = 0x01
	packet[2] = 0x00
	packet[3] = byte(len(username) + 20)

	md5aPlain := make([]byte, 0, 2+4+len(password))
	md5aPlain = append(md5aPlain, 0x03, 0x01)
	md5aPlain = append(md5aPlain, session.LoginSalt[:]...)
	md5aPlain = append(md5aPlain, password...)
	md5a := MD5(md5aPlain)
	copy(packet[loginOffsetMD5A:loginOffsetMD5A+16], md5a[:])
	copy(session.MD5Password[:], md5a[:])

	copyFixed(packet[loginOffsetUsername:loginOffsetControlCheckStatus], username)
	packet[loginOffsetControlCheckStatus] = 0x00
	packet[loginOffsetAdapterNum] = 0x00
	copy(packet[loginOffsetMacXorMD5:loginOffsetMacXorMD5+6], XOR(config.MAC[:], md5a[:], 6))

	md5bPlain := make([]byte, 1+len(password)+4+4)
	md5bPlain[0] = 0x01
	copy(md5bPlain[1:], password)
	copy(md5bPlain[1+len(password):], session.LoginSalt[:])
	md5b := MD5(md5bPlain)
	copy(packet[loginOffsetMD5B:loginOffsetMD5B+16], md5b[:])

	packet[loginOffsetIPIndicator] = 0x01
	copy(packet[loginOffsetIP:loginOffsetIP+4], config.IP[:])

	md5cPlain := make([]byte, 0, 101)
	md5cPlain = append(md5cPlain, packet[:97]...)
	md5cPlain = append(md5cPlain, 0x14, 0x00, 0x07, 0x0b)
	md5c := MD5(md5cPlain)
	copy(packet[loginOffsetMD5C:loginOffsetMD5C+8], md5c[:8])

	packet[loginOffsetIPDog] = 0x01
	copyFixed(packet[loginOffsetHostName:loginOffsetPrimaryDNS], []byte(config.HostName))
	copy(packet[loginOffsetPrimaryDNS:loginOffsetPrimaryDNS+4], config.PrimaryDNS[:])
	copy(packet[loginOffsetDHCPServer:loginOffsetDHCPServer+4], config.DHCPServer[:])

	packet[loginOffsetDrcomFlag] = 0x01
	copy(packet[loginOffsetDrcomIndicator:loginOffsetDrcomIndicator+len(drcomIndicator)], drcomIndicator)
	copy(packet[loginOffsetAuthVersionA:loginOffsetAuthVersionA+2], config.AuthVersion[:])
	copyFixed(packet[loginOffsetOSInfo:loginOffsetUnknownIndicator], []byte(config.OSInfo))
	copy(packet[loginOffsetUnknownIndicator:loginOffsetUnknownIndicator+len(unknownLoginIndicator)], unknownLoginIndicator)
	copy(packet[loginOffsetAuthVersionB:loginOffsetAuthVersionB+2], config.AuthVersion[:])
	packet[loginOffsetPasswordLength] = byte(len(password))

	passwordXORMD5 := XOR(md5a[:], password, rorLen)
	passwordROR := ROR(passwordXORMD5)
	copy(packet[loginOffsetPasswordROR:loginOffsetPasswordROR+rorLen], passwordROR)
	packet[loginOffsetPasswordROR+rorLen] = 0x02
	packet[loginOffsetPasswordROR+rorLen+1] = 0x0c

	checksumStart := 316 + rorLen
	checksumPlain := make([]byte, 0, checksumStart+12)
	checksumPlain = append(checksumPlain, packet[:checksumStart]...)
	checksumPlain = append(checksumPlain, 0x01, 0x26, 0x07, 0x11, 0x00, 0x00)
	checksumPlain = append(checksumPlain, config.MAC[:]...)
	checksum := Checksum(checksumPlain)
	copy(packet[checksumStart:checksumStart+4], checksum[:])

	copy(packet[322+rorLen:322+rorLen+6], config.MAC[:])

	paddingLen := 0
	if len(password) > 0 {
		paddingLen = (4 - len(password)%4) % 4
	}
	randomOffset := 328 + rorLen + paddingLen
	if randomOffset+2 <= len(packet) {
		mustReadRandom(rng, packet[randomOffset:randomOffset+2])
	}

	return packet
}

func ParseLoginResponse(packet []byte, session *Session) error {
	if len(packet) < 39 {
		return fmt.Errorf("login response too short: got %d bytes", len(packet))
	}
	copy(session.ServerDrcomIndicator[:], packet[23:39])
	return nil
}

func copyFixed(dst []byte, src []byte) {
	copy(dst, src[:min(len(dst), len(src))])
}
