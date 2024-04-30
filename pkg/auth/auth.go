package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/tradingcage/tradingcage-go/pkg/database"
	"github.com/tradingcage/tradingcage-go/pkg/email"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	"gorm.io/gorm"
)

var ErrNotAuthorized = errors.New("not authorized")

type AuthContext struct {
	Username string
	UserID   uint
}

var AuthContextKey = "auth_context"

var signKey = []byte(os.Getenv("SECRET_KEY"))

// GenerateRandomToken generates a random token for password reset functionality.
func GenerateRandomToken() (string, error) {
	tokenBytes := make([]byte, 16) // 128 bits
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(tokenBytes), nil
}

func CheckAuthenticated(c *gin.Context) (jwt.Claims, error) {
	authToken, err := c.Cookie("auth_token")
	if err != nil {
		return nil, err
	}
	if authToken == "" {
		return nil, errors.New("no auth token")
	}

	token, err := jwt.Parse(authToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return signKey, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.Claims)
	if !ok && !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

func safeFloat64ToUint(f float64) (uint, error) {
	if f < 0 {
		return 0, errors.New("value is negative, cannot convert to uint")
	}
	if f > float64(^uint(0)) {
		return 0, errors.New("value is too large to convert to uint")
	}
	return uint(f), nil
}

func GetCurrentUsername(c *gin.Context) (string, uint, error) {
	claims, err := CheckAuthenticated(c)
	if err != nil {
		return "", 0, err
	}
	mapClaims, ok := claims.(jwt.MapClaims)
	if !ok {
		return "", 0, errors.New("invalid claims")
	}
	usernameVal, ok := mapClaims["username"]
	if !ok {
		return "", 0, errors.New("invalid claims")
	}
	username, ok := usernameVal.(string)
	if !ok {
		return "", 0, errors.New("invalid claims")
	}
	userID, ok := mapClaims["userID"]
	if !ok {
		return "", 0, errors.New("invalid claims")
	}
	userIDVal, ok := userID.(float64)
	if !ok {
		return "", 0, errors.New("invalid claims")
	}
	userIDValActual, err := safeFloat64ToUint(userIDVal)
	if err != nil {
		return "", 0, err
	}
	return username, userIDValActual, nil
}

func RegisterEndpoint(db *gorm.DB) func(c *gin.Context) {
	return func(c *gin.Context) {
		var userReq struct {
			Username string `form:"username"`
			Password string `form:"password"`
		}
		if err := c.ShouldBind(&userReq); err != nil {
			c.HTML(http.StatusBadRequest, "register.tmpl", gin.H{"Error": err.Error()})
			return
		}

		if _, err := mail.ParseAddress(userReq.Username); err != nil {
			c.HTML(http.StatusBadRequest, "register.tmpl", gin.H{"Error": "Username must be a valid email address"})
			return
		}

		user := database.User{
			Username: userReq.Username,
			Password: userReq.Password,
		}

		err := user.Insert(db)
		if err != nil {
			if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
				c.HTML(http.StatusConflict, "register.tmpl", gin.H{"Error": "A user with that email address already exists. Please log in instead."})
			} else {
				c.HTML(http.StatusInternalServerError, "register.tmpl", gin.H{"Error": err.Error()})
			}
			return
		}

		loc, _ := time.LoadLocation("America/New_York")
		startingDate := time.Date(2019, 1, 2, 9, 30, 0, 0, loc)
		startingAccount := database.Account{
			Name:        "My First Account",
			UserID:      user.ID,
			Date:        startingDate,
			RealizedPnL: 25000,
		}
		err = startingAccount.Create(db)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "register.tmpl", gin.H{"Error": err.Error()})
			return
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"username": user.Username,
			"userID":   user.ID,
		})

		tokenString, err := token.SignedString(signKey)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "login.tmpl", gin.H{"error": err.Error()})
			return
		}

		c.SetCookie("auth_token", tokenString, 2147483647, "", "", false, true)

		c.Redirect(http.StatusFound, "/")
	}
}

func LoginEndpoint(db *gorm.DB) func(c *gin.Context) {
	return func(c *gin.Context) {
		var userReq struct {
			Username string `form:"username"`
			Password string `form:"password"`
		}
		if err := c.ShouldBind(&userReq); err != nil {
			c.HTML(http.StatusBadRequest, "login.tmpl", gin.H{"error": err.Error()})
			return
		}

		user := database.User{
			Username: userReq.Username,
			Password: userReq.Password,
		}

		isValid, err := user.IsValid(db)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "login.tmpl", gin.H{"error": err.Error()})
			return
		}
		if !isValid {
			c.HTML(http.StatusUnauthorized, "login.tmpl", gin.H{"error": "Invalid Credentials"})
			return
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"username": user.Username,
			"userID":   user.ID,
		})

		tokenString, err := token.SignedString(signKey)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "login.tmpl", gin.H{"error": err.Error()})
			return
		}

		// Set the token in a secure HttpOnly cookie
		c.SetCookie("auth_token", tokenString, 2147483647, "", "", false, true)

		// Redirect to /
		c.Redirect(http.StatusFound, "/")
	}
}

func ForgotPasswordEndpoint(emailSenderService email.EmailSenderService, db *gorm.DB) func(c *gin.Context) {
	return func(c *gin.Context) {
		var req struct {
			Email string `form:"email"`
		}
		if err := c.ShouldBind(&req); err != nil {
			c.HTML(http.StatusBadRequest, "forgot-password.tmpl", gin.H{"Error": "Invalid form input"})
			return
		}

		if _, err := mail.ParseAddress(req.Email); err != nil {
			c.HTML(http.StatusBadRequest, "forgot-password.tmpl", gin.H{"Error": "Please enter a valid email"})
			return
		}

		user, err := database.GetUserByUsername(db, req.Email)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// Consider whether to reveal whether email exists
				c.HTML(http.StatusOK, "forgot-password.tmpl", gin.H{"Message": "If your email is registered, you will receive a password reset link."})
				return
			}
			c.HTML(http.StatusInternalServerError, "forgot-password.tmpl", gin.H{"Error": "Failed to process your request"})
			return
		}

		token, err := GenerateRandomToken()
		if err != nil {
			c.HTML(http.StatusInternalServerError, "forgot-password.tmpl", gin.H{"Error": "Failed to generate reset token"})
			return
		}

		// Create a ForgotPasswordEntry and write it to the database
		forgotPasswordEntry := database.ForgotPasswordEntry{
			UserID: user.ID,
			Token:  token,
		}

		if err := db.Create(&forgotPasswordEntry).Error; err != nil {
			c.HTML(http.StatusInternalServerError, "forgot-password.tmpl", gin.H{"Error": "Failed to save reset token"})
			return
		}

		resetLink := fmt.Sprintf("https://%s/reset-password?token=%s", c.Request.Host, token)
		err = emailSenderService.SendPasswordResetEmail(user.Username, resetLink) // Functionality of email system to be implemented separately
		if err != nil {
			c.HTML(http.StatusInternalServerError, "forgot-password.tmpl", gin.H{"Error": "Failed to send reset email"})
			return
		}

		c.HTML(http.StatusOK, "forgot-password.tmpl", gin.H{"Message": "If your email is registered, you will receive a password reset link."})
	}
}

func ResetPasswordEndpoint(db *gorm.DB) func(c *gin.Context) {
	return func(c *gin.Context) {
		var req struct {
			Token    string `form:"token"`
			Password string `form:"password"`
		}
		if err := c.ShouldBind(&req); err != nil {
			c.HTML(http.StatusBadRequest, "reset-password.tmpl", gin.H{"Error": "Invalid form input"})
			return
		}
		var forgotPasswordEntry database.ForgotPasswordEntry
		err := db.Where("token = ?", req.Token).First(&forgotPasswordEntry).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.HTML(http.StatusBadRequest, "reset-password.tmpl", gin.H{"Error": "Invalid or expired token"})
				return
			}
			c.HTML(http.StatusInternalServerError, "reset-password.tmpl", gin.H{"Error": "Failed to process your request"})
			return
		}
		if time.Since(forgotPasswordEntry.CreatedAt) > 24*time.Hour {
			c.HTML(http.StatusBadRequest, "reset-password.tmpl", gin.H{"Error": "Token has expired"})
			return
		}
		user, err := database.GetUserByID(db, forgotPasswordEntry.UserID)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "reset-password.tmpl", gin.H{"Error": "Failed to find user"})
			return
		}
		user.Password = req.Password
		if err := user.Upsert(db); err != nil {
			c.HTML(http.StatusInternalServerError, "reset-password.tmpl", gin.H{"Error": "Failed to update password"})
			return
		}
		c.HTML(http.StatusOK, "reset-password.tmpl", gin.H{"Success": "Password successfully reset"})
	}
}

func AuthInfoMiddleware(c *gin.Context) {
	username, userID, err := GetCurrentUsername(c)
	if err == nil && username != "" && userID != 0 {
		c.Set(AuthContextKey, &AuthContext{
			UserID:   userID,
			Username: username,
		})
	}
	c.Next()
}

func IsAuthenticated(c *gin.Context) bool {
	_, err := TryGetAuthInfo(c)
	return err == nil
}

func AuthMiddleware(c *gin.Context) {
	if !IsAuthenticated(c) {
		Logout(c)
		c.Redirect(http.StatusFound, "/")
		c.Abort()
		return
	}
	c.Next()
}

func Logout(c *gin.Context) {
	c.SetCookie("auth_token", "", -1, "/", c.Request.URL.Hostname(), c.Request.TLS != nil, true)
}

func GetAuthInfoFromContext(c *gin.Context) *AuthContext {
	return c.MustGet(AuthContextKey).(*AuthContext)
}

func TryGetAuthInfo(c *gin.Context) (*AuthContext, error) {
	authContext, exists := c.Get(AuthContextKey)
	if !exists {
		return nil, errors.New("no auth context")
	}

	authInfo, ok := authContext.(*AuthContext)
	if !ok {
		return nil, errors.New("invalid auth context")
	}

	if authInfo.Username == "" || authInfo.UserID == 0 {
		return nil, errors.New("empty auth context")
	}

	return authInfo, nil
}
