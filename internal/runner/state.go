package runner

type State string

const (
	StateDisconnected   State = "Disconnected"
	StateLoginChallenge State = "LoginChallenge"
	StateLoggingIn      State = "LoggingIn"
	StateOnline         State = "Online"
	StateReconnecting   State = "Reconnecting"
	StateLoggingOut     State = "LoggingOut"
	StateStopped        State = "Stopped"
	StateFailed         State = "Failed"
)
