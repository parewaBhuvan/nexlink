package otp

// Sender abstracts how an OTP gets delivered to a user.
// Implementations: ConsoleSender (dev), Fast2SMSSender (future).
type Sender interface {
	Send(mobileNumber string, code string) error
}