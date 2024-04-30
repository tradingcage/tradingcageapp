package billing

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tradingcage/tradingcage-go/pkg/auth"
	"github.com/tradingcage/tradingcage-go/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v76"
	billingportal "github.com/stripe/stripe-go/v76/billingportal/session"
	"github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/stripe/stripe-go/v76/webhook"
	"gorm.io/gorm"
)

func BillingMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, userID, err := auth.GetCurrentUsername(c)
		if err != nil {
			c.Redirect(http.StatusFound, "/")
			c.Abort()
			return
		}

		user, err := database.GetUserByID(db, userID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if !user.HasActiveSubscription() {
			c.Redirect(http.StatusFound, "/checkout")
			c.Abort()
			return
		}

		c.Next()
	}
}

func CreateCheckoutSession(c *gin.Context, userID uint) (string, error) {
	stripe.Key = os.Getenv("STRIPE_KEY")

	username, _, err := auth.GetCurrentUsername(c)
	if err != nil {
		return "", fmt.Errorf("auth.GetCurrentUsername: %v", err)
	}

	domain := fmt.Sprintf("https://%s", strings.TrimSuffix(c.Request.Host, "/"))
	params := &stripe.CheckoutSessionParams{
		AllowPromotionCodes: stripe.Bool(true),
		CustomerEmail:       stripe.String(username), // Prefill the customer's email
		UIMode:              stripe.String("embedded"),
		ReturnURL:           stripe.String(domain + "/checkout-result?sessionID={CHECKOUT_SESSION_ID}"),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(os.Getenv("STRIPE_MONTHLY_PRICE")), // Monthly Price ID
				Quantity: stripe.Int64(1),
			},
		},
		Mode: stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			TrialPeriodDays: stripe.Int64(7), // 7-day free trial
		},
		AutomaticTax: &stripe.CheckoutSessionAutomaticTaxParams{Enabled: stripe.Bool(true)},
	}

	s, err := session.New(params)

	if err != nil {
		return "", fmt.Errorf("session.New: %v", err)
	}

	return s.ClientSecret, nil
}

func GetCheckoutSessionStatus(sessionID string) string {
	s, _ := session.Get(sessionID, nil)

	return string(s.Status)
}

func HandleStripeWebhook(db *gorm.DB) func(*gin.Context) {
	stripeKey := os.Getenv("STRIPE_WEBHOOK_KEY")
	return func(c *gin.Context) {
		const MaxBodyBytes = int64(65536)
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxBodyBytes)
		payload, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			c.Writer.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		// Verify webhook signature to ensure it's sent from Stripe
		event, err := webhook.ConstructEvent(payload, c.Request.Header.Get("Stripe-Signature"), stripeKey)
		if err != nil {
			log.Printf("Error: %s\n", err.Error())
			c.Writer.WriteHeader(http.StatusBadRequest) // Return a 400 error on a bad signature
			return
		}
		switch event.Type {
		case "checkout.session.completed":
			var session stripe.CheckoutSession
			err := json.Unmarshal(event.Data.Raw, &session)
			if err != nil {
				c.Writer.WriteHeader(http.StatusBadRequest)
				return
			}
			if session.Customer != nil && session.Customer.ID != "" && session.CustomerEmail != "" {
				var user database.User
				err = db.Transaction(func(tx *gorm.DB) error {
					if err := tx.Where("username = ?", session.CustomerEmail).First(&user).Error; err != nil {
						return err
					}
					now := time.Now()
					user.SubscriptionStartedAt = &now
					user.SubscriptionEndedAt = nil
					user.StripeCustomerID = &session.Customer.ID
					return tx.Save(&user).Error
				})
				if err != nil {
					c.Writer.WriteHeader(http.StatusInternalServerError)
					return
				}
			} else {
				c.Writer.WriteHeader(http.StatusBadRequest)
				return
			}
			// Return a 200 on success
			c.Writer.WriteHeader(http.StatusOK)
		case "customer.subscription.deleted":
			var subscription stripe.Subscription
			err := json.Unmarshal(event.Data.Raw, &subscription)
			if err != nil {
				c.Writer.WriteHeader(http.StatusBadRequest)
				return
			}
			if subscription.Customer != nil && subscription.Customer.ID != "" {
				var endsAt time.Time
				if subscription.EndedAt != 0 {
					// Convert Stripe's timestamp to time.Time
					endsAt = time.Unix(subscription.EndedAt, 0)
				} else {
					// If no EndedAt provided, use current time
					endsAt = time.Now()
				}
				// Use the customer ID to update the user in your database
				var user database.User
				if err := db.Model(&user).Where("stripe_customer_id = ?", subscription.Customer.ID).Update("subscription_ended_at", endsAt).Error; err != nil {
					c.Writer.WriteHeader(http.StatusInternalServerError)
					return
				}
			} else {
				c.Writer.WriteHeader(http.StatusBadRequest)
				return
			}
			// Return a 200 on success
			c.Writer.WriteHeader(http.StatusOK)
		default:
			c.Writer.WriteHeader(http.StatusAccepted)
		}
	}
}

func GetManageSubscriptionLink(c *gin.Context, db *gorm.DB, email string) (string, error) {
	stripe.Key = os.Getenv("STRIPE_KEY")

	user, err := database.GetUserByUsername(db, email)
	if err != nil {
		return "", err
	}
	if user.StripeCustomerID == nil || *user.StripeCustomerID == "" {
		return "", nil
	}

	params := &stripe.BillingPortalSessionParams{
		Customer:  user.StripeCustomerID,
		ReturnURL: stripe.String(fmt.Sprintf("https://%s/dashboard", c.Request.Host)),
	}

	session, err := billingportal.New(params)
	if err != nil {
		return "", err
	}

	return session.URL, nil
}
