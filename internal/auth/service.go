package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/parewaBhuvan/nexlink/internal/otp"
	"github.com/redis/go-redis/v9"
)

type Service struct {
	redis     *redis.Client
	otpSender otp.Sender
	db        *pgxpool.Pool
}

func NewService(db *pgxpool.Pool, redisClient *redis.Client, sender otp.Sender) *Service {
	return &Service{
		db:        db,
		redis:     redisClient,
		otpSender: sender,
	}
}

var (
	ErrCooldownActive = errors.New("please wait before requesting another OTP")
	ErrSendFailed     = errors.New("something went wrong, please try again later")
	ErrInvalidMobile  = errors.New("invalid mobile number")
)

func isValidMobile(mobile string) bool {
	if len(mobile) < 10 || len(mobile) > 15 {
		return false
	}
	for _, c := range mobile {
		if c < '0' || c > '9' {
			if c != '+' {
				return false
			}
		}
	}
	return true
}

func generateOTP() (string, error) {
	max := big.NewInt(1000000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func hashOTP(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

func (s *Service) RequestOTP(ctx context.Context, mobile string) error {
	if !isValidMobile(mobile) {
		return ErrInvalidMobile
	}

	cooldownKey := fmt.Sprintf("otp_cooldown:%s", mobile)
	exists, err := s.redis.Exists(ctx, cooldownKey).Result()
	if err != nil {
		return err
	}
	if exists == 1 {
		return ErrCooldownActive
	}

	code, err := generateOTP()
	if err != nil {
		return err
	}

	if err := s.otpSender.Send(mobile, code); err != nil {
		return ErrSendFailed
	}

	otpKey := fmt.Sprintf("otp:%s", mobile)
	hashedCode := hashOTP(code)

	if err := s.redis.Set(ctx, otpKey, hashedCode, 5*time.Minute).Err(); err != nil {
		return err
	}
	if err := s.redis.Set(ctx, cooldownKey, "1", 30*time.Second).Err(); err != nil {
		return err
	}

	return nil
}

func (s *Service) GetOrCreateUser(ctx context.Context, mobile string) (string, error) {
	_, err := s.db.Exec(ctx,
		`INSERT INTO users (user_mobile) VALUES ($1) ON CONFLICT (user_mobile) DO NOTHING`,
		mobile,
	)
	if err != nil {
		return "", err
	}

	var userID string
	err = s.db.QueryRow(ctx,
		`SELECT user_id FROM users WHERE user_mobile = $1`,
		mobile,
	).Scan(&userID)
	if err != nil {
		return "", err
	}

	return userID, nil
}
func (s *Service) CreateSession(ctx context.Context, userID string) (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(tokenBytes)

	sessionKey := fmt.Sprintf("session:%s", token)
	if err := s.redis.Set(ctx, sessionKey, userID, 3*24*time.Hour).Err(); err != nil {
		return "", err
	}

	return token, nil
}

var (
	ErrOTPExpiredOrNotFound = errors.New("OTP expired or not found, please request a new one")
	ErrOTPMismatch          = errors.New("incorrect OTP")
	ErrTooManyAttempts      = errors.New("too many incorrect attempts, please request a new OTP")
)

const maxOTPAttempts = 5

func (s *Service) VerifyOTP(ctx context.Context, mobile string, submittedCode string) error {
	otpKey := fmt.Sprintf("otp:%s", mobile)
	attemptsKey := fmt.Sprintf("otp_attempts:%s", mobile)

	storedHash, err := s.redis.Get(ctx, otpKey).Result()
	if err == redis.Nil {
		return ErrOTPExpiredOrNotFound
	}
	if err != nil {
		return err
	}

	submittedHash := hashOTP(submittedCode)
	if submittedHash != storedHash {
		attempts, err := s.redis.Incr(ctx, attemptsKey).Result()
		if err != nil {
			return err
		}
		if attempts == 1 {
			s.redis.Expire(ctx, attemptsKey, 5*time.Minute)
		}
		if attempts >= maxOTPAttempts {
			s.redis.Del(ctx, otpKey, attemptsKey)
			return ErrTooManyAttempts
		}
		return ErrOTPMismatch
	}

	s.redis.Del(ctx, otpKey, attemptsKey)

	return nil
}
