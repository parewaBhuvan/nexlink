package otp

import "fmt"

// ConsoleSender prints the OTP to stdout instead of sending a real SMS.
// Used for local development so no SMS provider is required.
type ConsoleSender struct{}

func NewConsoleSender() *ConsoleSender {
	return &ConsoleSender{}
}

func (c *ConsoleSender) Send(mobileNumber string, code string) error {
	fmt.Printf("[MOCK SMS] To: %s | OTP: %s\n", mobileNumber, code)
	return nil
}