package session

import "jlu-drcom-win/internal/protocol"

type Session = protocol.Session

func New() Session {
	return Session{}
}
