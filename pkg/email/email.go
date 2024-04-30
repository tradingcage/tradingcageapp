package email

import (
	"fmt"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

// EmailSenderService defines the interface for an email sender service.
type EmailSenderService interface {
	SendPasswordResetEmail(recipientEmail, resetLink string) error
}

// SendGridEmailService implements the EmailSenderService interface using SendGrid.
type SendGridEmailService struct {
	apiKey string
}

// NewSendGridEmailService creates a new instance of SendGridEmailService with the provided API key.
func NewSendGridEmailService(apiKey string) *SendGridEmailService {
	return &SendGridEmailService{apiKey: apiKey}
}

// SendPasswordResetEmail sends a password reset email to the specified recipient using SendGrid.
func (s *SendGridEmailService) SendPasswordResetEmail(recipientEmail, resetLink string) error {
	from := mail.NewEmail("Trading Cage", "mail@tradingcage.com")
	subject := "Password Reset"
	to := mail.NewEmail("User", recipientEmail)
	plainTextContent := fmt.Sprintf("Please use the following link to reset your password: %s", resetLink)
	htmlContent := fmt.Sprintf("<p>Please use the following link to reset your password: <a href='%s'>%s</a></p>", resetLink, resetLink)
	message := mail.NewSingleEmail(from, subject, to, plainTextContent, htmlContent)
	client := sendgrid.NewSendClient(s.apiKey)
	response, err := client.Send(message)
	if err != nil {
		return err
	}
	if response.StatusCode >= 300 {
		// handle non-2xx status codes
		return fmt.Errorf("received non-2xx status code: %d", response.StatusCode)
	}
	return nil
}
