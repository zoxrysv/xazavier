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
    key := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
    return fmt.Sprintf("$argon2id$v=19$m=65536,t=1,p=4$%x$%x", salt, key), nil
}

func VerifyPassword(password, encoded string) bool {
    if encoded == "" || !strings.HasPrefix(encoded, "$argon2id$") {
        return false
    }
    parts := strings.Split(encoded, "$")
    if len(parts) < 6 {
        return false
    }
    saltHex := parts[4]
    expectedHex := parts[5]
    salt, err := hex.DecodeString(saltHex)
    if err != nil {
        return false
    }
    expectedKey, err := hex.DecodeString(expectedHex)
    if err != nil {
        return false
    }
    actualKey := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
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

func GenerateRefreshToken() string {
    b := make([]byte, 32)
    if _, err := rand.Read(b); err != nil {
        return base64.RawURLEncoding.EncodeToString([]byte(time.Now().String()))
    }
    return base64.RawURLEncoding.EncodeToString(b)
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
