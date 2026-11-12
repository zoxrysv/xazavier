package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/argon2"
	"xazavier/internal/domain"
)

func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	timeCost := uint32(1)
	memoryCost := uint32(64 * 1024)
	parallelism := uint8(4)
	keyLen := uint32(32)

	key := argon2.IDKey([]byte(password), salt, timeCost, memoryCost, parallelism, keyLen)
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Key := base64.RawStdEncoding.EncodeToString(key)

	// Standard PHC format: $argon2id$v=19$m=<memory>,t=<time>,p=<parallelism>$<salt>$<hash>
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, memoryCost, timeCost, parallelism, b64Salt, b64Key), nil
}

func VerifyPassword(password, encoded string) bool {
	if encoded == "" || !strings.HasPrefix(encoded, "$argon2id$") {
		return false
	}
	parts := strings.Split(encoded, "$")
	if len(parts) < 6 {
		return false
	}

	var memory uint32
	var time uint32
	var threads uint8

	params := strings.Split(parts[3], ",")
	for _, p := range params {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) == 2 {
			switch kv[0] {
			case "m":
				fmt.Sscanf(kv[1], "%d", &memory)
			case "t":
				fmt.Sscanf(kv[1], "%d", &time)
			case "p":
				fmt.Sscanf(kv[1], "%d", &threads)
			}
		}
	}
	if memory == 0 || time == 0 || threads == 0 {
		memory = 64 * 1024
		time = 1
		threads = 4
	}

	// Support both base64 (standard PHC) and legacy hex formats
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		salt, err = hex.DecodeString(parts[4])
		if err != nil {
			return false
		}
	}

	expectedKey, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		expectedKey, err = hex.DecodeString(parts[5])
		if err != nil {
			return false
		}
	}

	actualKey := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(expectedKey)))
	return hmac.Equal(actualKey, expectedKey)
}

func GenerateAccessToken(userID, sessionID, secret string, ttl time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"sid": sessionID,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(ttl).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ParseAccessToken(tokenString, secret string) (*domain.SessionClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid claims")
	}
	userID, _ := claims["sub"].(string)
	sessionID, _ := claims["sid"].(string)
	return &domain.SessionClaims{UserID: userID, SessionID: sessionID}, nil
}

func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func GenerateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto rand failed: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func NormalizeEmail(v string) string {
	return strings.TrimSpace(strings.ToLower(v))
}

func IsValidEmail(email string) bool {
	if len(email) < 3 || len(email) > 254 {
		return false
	}
	at := strings.Index(email, "@")
	if at <= 0 || at >= len(email)-1 {
		return false
	}
	dot := strings.LastIndex(email, ".")
	if dot <= at+1 || dot >= len(email)-1 {
		return false
	}
	return !strings.ContainsAny(email, " \t\r\n")
}
