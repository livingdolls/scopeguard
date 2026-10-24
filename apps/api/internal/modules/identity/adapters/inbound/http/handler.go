package httpadapter

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/application"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/domain"
	"github.com/livingdolls/scopeguard/apps/api/internal/platform/authctx"
)

type Handler struct {
	register application.RegisterUser
	verify   application.VerifyEmail
	login    application.LoginUser
	profile  application.GetCurrentUser
	logger   *slog.Logger
}

func NewHandler(register application.RegisterUser, verify application.VerifyEmail, login application.LoginUser, profile application.GetCurrentUser, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return &Handler{
		register: register,
		verify:   verify,
		login:    login,
		profile:  profile,
		logger:   logger,
	}
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type verifyEmailRequest struct {
	Token string `json:"token"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type profileResponse struct {
	ID    uuid.UUID `json:"id"`
	Email string    `json:"email"`
}

func noStore(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
}

func acceptedRegistration(c *gin.Context) {
	c.JSON(http.StatusAccepted, gin.H{
		"message": "If the request can be processed, verification instructions will be sent",
	},
	)
}

func (h *Handler) Register(c *gin.Context) {
	noStore(c)

	var request registerRequest

	if err := decodeStrictJSON(c, &request); err != nil {
		writeJSONDecoderError(c, err)
		return
	}

	_, err := h.register.Execute(c.Request.Context(), application.RegisterUserInput{
		Email:    request.Email,
		Password: request.Password,
	})

	switch {
	case err == nil:
		acceptedRegistration(c)

	case errors.Is(err, application.ErrEmailAlreadyExists):
		acceptedRegistration(c)

	case errors.Is(err, domain.ErrInvalidEmail):
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid email",
		})

	case errors.Is(err, domain.ErrInvalidPassword):
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid password",
		})

	default:
		h.logger.Error("registration operation failed")
		c.JSON(
			http.StatusInternalServerError, gin.H{
				"error": "internal server error",
			})
	}
}

func (h *Handler) VerifyEmail(c *gin.Context) {
	noStore(c)

	var request verifyEmailRequest

	if err := decodeStrictJSON(c, &request); err != nil {
		writeJSONDecoderError(c, err)
		return
	}

	err := h.verify.Execute(c.Request.Context(), request.Token)

	switch {
	case err == nil:
		c.Status(http.StatusNoContent)
	case errors.Is(err, application.ErrInvalidVerificationToken):
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid or expired verification token",
		})
	default:
		h.logger.Error("verification operation failed")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "internal server error",
		})
	}
}

func (h *Handler) Login(c *gin.Context) {
	noStore(c)

	var request loginRequest

	if err := decodeStrictJSON(c, &request); err != nil {
		writeJSONDecoderError(c, err)
		return
	}

	output, err := h.login.Execute(c.Request.Context(), application.LoginUserInput{
		Email:    request.Email,
		Password: request.Password,
	})

	switch {
	case err == nil:
		c.JSON(http.StatusOK, loginResponse{
			AccessToken: output.AccessToken,
			TokenType:   output.TokenType,
			ExpiresAt:   output.ExpiresAt,
		})

	case errors.Is(err, application.ErrInvalidCredentials):
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid credentials",
		})

	default:
		h.logger.Error("login operation failed")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "internal server error",
		})
	}
}

func (h *Handler) Me(c *gin.Context) {
	noStore(c)

	userID, authenticated := authctx.UserIDFromContext(c.Request.Context())

	if !authenticated {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "authentication required",
		})
		return
	}

	profile, err := h.profile.Execute(c.Request.Context(), userID)

	switch {
	case err == nil:
		c.JSON(http.StatusOK, profileResponse{
			ID:    profile.ID,
			Email: profile.Email,
		})

	case errors.Is(err, application.ErrUserNotFound):
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "unauthorized",
		})

	default:
		h.logger.Error("user operation failed")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "internal server error",
		})
	}
}
