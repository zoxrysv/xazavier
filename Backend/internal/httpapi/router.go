package httpapi

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "log/slog"
    "net"
    "net/http"
    "strconv"
    "strings"
    "sync"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "xazavier/internal/apperr"
    "xazavier/internal/auth"
    "xazavier/internal/config"
    "xazavier/internal/domain"
    "xazavier/internal/repository"
    "xazavier/internal/service"
)

const apiPrefix = "/api"

type ctxKey string

const userContextKey ctxKey = "user"

func NewServer(cfg config.Config, store repository.Store) http.Handler {
    if store == nil {
        panic("repository.Store must not be nil")
    }
    svc := service.NewService(cfg, store)

    r := chi.NewRouter()
    r.Use(middleware.RequestID)
    r.Use(middleware.RealIP)
    r.Use(middleware.Recoverer)
    r.Use(middleware.Logger)
    r.Use(middleware.Timeout(30 * time.Second))
    r.Use(corsMiddleware(cfg))

    authLimiter := newRateLimiter(10, time.Minute)

    r.Route(apiPrefix, func(r chi.Router) {
        r.Post("/auth/register", limit(authLimiter, cfg, func(w http.ResponseWriter, r *http.Request) {
            var body domain.AuthRegisterRequest
            if err := decodeJSON(r, &body); err != nil { writeError(w, err); return }
            bundle, err := svc.Register(r.Context(), body, clientIP(r, cfg))
            if err != nil { writeError(w, err); return }
            setRefreshTokenCookie(w, bundle.RefreshToken, cfg)
            writeJSON(w, http.StatusCreated, bundle)
        }))
        r.Post("/auth/login", limit(authLimiter, cfg, func(w http.ResponseWriter, r *http.Request) {
            var body domain.AuthLoginRequest
            if err := decodeJSON(r, &body); err != nil { writeError(w, err); return }
            bundle, err := svc.Login(r.Context(), body, clientIP(r, cfg))
            if err != nil { writeError(w, err); return }
            setRefreshTokenCookie(w, bundle.RefreshToken, cfg)
            writeJSON(w, http.StatusOK, bundle)
        }))
        r.Post("/auth/refresh", limit(authLimiter, cfg, func(w http.ResponseWriter, r *http.Request) {
            var body domain.AuthRefreshRequest
            _ = decodeJSON(r, &body)
            if cookie, err := r.Cookie("refresh_token"); err == nil && cookie.Value != "" {
                body.RefreshToken = cookie.Value
            }
            bundle, err := svc.Refresh(r.Context(), body, clientIP(r, cfg))
            if err != nil { writeError(w, err); return }
            if bundle.RefreshToken != "" {
                setRefreshTokenCookie(w, bundle.RefreshToken, cfg)
            }
            writeJSON(w, http.StatusOK, bundle)
        }))
        r.Post("/auth/logout", func(w http.ResponseWriter, r *http.Request) {
            var body domain.AuthLogoutRequest
            _ = decodeJSON(r, &body)
            if cookie, err := r.Cookie("refresh_token"); err == nil && cookie.Value != "" {
                body.RefreshToken = cookie.Value
            }
            if err := svc.Logout(r.Context(), body); err != nil { writeError(w, err); return }
            clearRefreshTokenCookie(w, cfg)
            writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
        })

        r.With(requireAuth(svc)).Get("/me", func(w http.ResponseWriter, r *http.Request) {
            user, ok := svc.CurrentUser(r.Context())
            if !ok { writeError(w, apperr.Unauthorized("Session expired.")); return }
            writeJSON(w, http.StatusOK, user.Public())
        })
        r.With(requireAuth(svc)).Get("/clusters", func(w http.ResponseWriter, r *http.Request) {
            items, err := svc.ListClusters(r.Context())
            if err != nil { writeError(w, err); return }
            writeJSON(w, http.StatusOK, items)
        })
        r.With(requireAuth(svc)).Post("/clusters", func(w http.ResponseWriter, r *http.Request) {
            var body domain.ClusterCreateRequest
            if err := decodeJSON(r, &body); err != nil { writeError(w, err); return }
            item, err := svc.CreateCluster(r.Context(), body)
            if err != nil { writeError(w, err); return }
            writeJSON(w, http.StatusCreated, item)
        })
        r.With(requireAuth(svc)).Put("/clusters/{id}", func(w http.ResponseWriter, r *http.Request) {
            id := chi.URLParam(r, "id")
            var body domain.ClusterUpdateRequest
            if err := decodeJSON(r, &body); err != nil { writeError(w, err); return }
            item, err := svc.UpdateCluster(r.Context(), id, body)
            if err != nil { writeError(w, err); return }
            writeJSON(w, http.StatusOK, item)
        })
        r.With(requireAuth(svc)).Delete("/clusters/{id}", func(w http.ResponseWriter, r *http.Request) {
            id := chi.URLParam(r, "id")
            if err := svc.DeleteCluster(r.Context(), id); err != nil { writeError(w, err); return }
            writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
        })

        r.With(requireAuth(svc)).Get("/activities", func(w http.ResponseWriter, r *http.Request) {
            items, err := svc.ListActivities(r.Context())
            if err != nil { writeError(w, err); return }
            writeJSON(w, http.StatusOK, items)
        })
        r.With(requireAuth(svc)).Post("/activities", func(w http.ResponseWriter, r *http.Request) {
            var body domain.ActivityCreateRequest
            if err := decodeJSON(r, &body); err != nil { writeError(w, err); return }
            item, err := svc.CreateActivity(r.Context(), body)
            if err != nil { writeError(w, err); return }
            writeJSON(w, http.StatusCreated, item)
        })
        r.With(requireAuth(svc)).Put("/activities/{id}", func(w http.ResponseWriter, r *http.Request) {
            id := chi.URLParam(r, "id")
            var body domain.ActivityUpdateRequest
            if err := decodeJSON(r, &body); err != nil { writeError(w, err); return }
            item, err := svc.UpdateActivity(r.Context(), id, body)
            if err != nil { writeError(w, err); return }
            writeJSON(w, http.StatusOK, item)
        })
        r.With(requireAuth(svc)).Delete("/activities/{id}", func(w http.ResponseWriter, r *http.Request) {
            id := chi.URLParam(r, "id")
            if err := svc.DeleteActivity(r.Context(), id); err != nil { writeError(w, err); return }
            writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
        })

        r.With(requireAuth(svc)).Get("/logs", func(w http.ResponseWriter, r *http.Request) {
            filter := domain.LogListFilter{
                Limit:    queryInt(r, "limit", 0),
                Offset:   queryInt(r, "offset", 0),
                FromDate: r.URL.Query().Get("from"),
                ToDate:   r.URL.Query().Get("to"),
            }
            items, err := svc.ListLogs(r.Context(), filter)
            if err != nil { writeError(w, err); return }
            writeJSON(w, http.StatusOK, items)
        })
        r.With(requireAuth(svc)).Get("/logs/{id}", func(w http.ResponseWriter, r *http.Request) {
            id := chi.URLParam(r, "id")
            item, err := svc.GetLog(r.Context(), id)
            if err != nil { writeError(w, err); return }
            writeJSON(w, http.StatusOK, item)
        })
        r.With(requireAuth(svc)).Post("/logs", func(w http.ResponseWriter, r *http.Request) {
            var body domain.LogCreateRequest
            if err := decodeJSON(r, &body); err != nil { writeError(w, err); return }
            item, err := svc.CreateLog(r.Context(), body)
            if err != nil { writeError(w, err); return }
            writeJSON(w, http.StatusCreated, item)
        })
        r.With(requireAuth(svc)).Put("/logs/{id}", func(w http.ResponseWriter, r *http.Request) {
            id := chi.URLParam(r, "id")
            var body domain.LogUpdateRequest
            if err := decodeJSON(r, &body); err != nil { writeError(w, err); return }
            item, err := svc.UpdateLog(r.Context(), id, body)
            if err != nil { writeError(w, err); return }
            writeJSON(w, http.StatusOK, item)
        })
        r.With(requireAuth(svc)).Delete("/logs/{id}", func(w http.ResponseWriter, r *http.Request) {
            id := chi.URLParam(r, "id")
            if err := svc.DeleteLog(r.Context(), id); err != nil { writeError(w, err); return }
            writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
        })
        r.With(requireAuth(svc)).Get("/logs/{id}/audit", func(w http.ResponseWriter, r *http.Request) {
            id := chi.URLParam(r, "id")
            items, err := svc.ListAuditForLog(r.Context(), id)
            if err != nil { writeError(w, err); return }
            writeJSON(w, http.StatusOK, items)
        })
        r.With(requireAuth(svc)).Get("/audit", func(w http.ResponseWriter, r *http.Request) {
            filter := domain.AuditListFilter{
                Limit:  queryInt(r, "limit", 0),
                Offset: queryInt(r, "offset", 0),
            }
            items, err := svc.ListAudit(r.Context(), filter)
            if err != nil { writeError(w, err); return }
            writeJSON(w, http.StatusOK, items)
        })
        r.With(requireAuth(svc)).Post("/reset", func(w http.ResponseWriter, r *http.Request) {
            if err := svc.Reset(r.Context()); err != nil { writeError(w, err); return }
            writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
        })
    })

    r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
        writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
    })
    r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
        writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
    })

    return r
}

func corsMiddleware(cfg config.Config) func(http.Handler) http.Handler {
    allowedOrigins := make(map[string]bool)
    allowAll := false
    for _, o := range cfg.CORSAllowedOrigins {
        trimmed := strings.TrimSpace(o)
        if trimmed == "*" {
            allowAll = true
        } else if trimmed != "" {
            allowedOrigins[strings.ToLower(trimmed)] = true
        }
    }
    // If empty in dev, allow localhost defaults
    if len(allowedOrigins) == 0 && !allowAll {
        allowedOrigins["http://localhost:5173"] = true
        allowedOrigins["http://localhost:3000"] = true
        allowedOrigins["http://127.0.0.1:5173"] = true
        allowedOrigins["http://127.0.0.1:3000"] = true
    }

    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.Header().Set("Content-Type", "application/json")
            w.Header().Set("X-Content-Type-Options", "nosniff")
            w.Header().Set("Cache-Control", "no-store")
            origin := r.Header.Get("Origin")
            if origin != "" {
                isAllowed := allowAll || allowedOrigins[strings.ToLower(origin)]
                if isAllowed {
                    w.Header().Set("Access-Control-Allow-Origin", origin)
                    w.Header().Set("Access-Control-Allow-Credentials", "true")
                    w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
                    w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
                }
            }
            if r.Method == http.MethodOptions {
                w.WriteHeader(http.StatusNoContent)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}

func requireAuth(s *service.Service) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            header := r.Header.Get("Authorization")
            token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
            if token == "" {
                writeError(w, apperr.Unauthorized("Missing or invalid bearer token."))
                return
            }
            claims, err := auth.ParseAccessToken(token, s.JWTSecret())
            if err != nil {
                writeError(w, apperr.Unauthorized("Session expired."))
                return
            }
            user, ok := s.ValidateSession(claims.SessionID)
            if !ok {
                writeError(w, apperr.Unauthorized("Session expired."))
                return
            }
            ctx := context.WithValue(r.Context(), "user", user)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

func decodeJSON(r *http.Request, v any) error {
    if r.Body == nil {
        return apperr.InvalidRequest("Request body is required.")
    }
    defer r.Body.Close()
    // Limit body size to 1MB to prevent memory exhaustion attacks
    r.Body = http.MaxBytesReader(nil, r.Body, 1024*1024)
    if err := json.NewDecoder(r.Body).Decode(v); err != nil {
        var maxBytesErr *http.MaxBytesError
        if errors.As(err, &maxBytesErr) {
            return apperr.InvalidRequest("Request body exceeds maximum size of 1MB.")
        }
        return apperr.InvalidRequest("Malformed JSON request.")
    }
    return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
    w.WriteHeader(status)
    if err := json.NewEncoder(w).Encode(v); err != nil {
        slog.Warn("json encode failed", "err", err)
    }
}

func queryInt(r *http.Request, key string, def int) int {
    if v := r.URL.Query().Get(key); v != "" {
        if i, err := strconv.Atoi(v); err == nil {
            return i
        }
    }
    return def
}

func setRefreshTokenCookie(w http.ResponseWriter, token string, cfg config.Config) {
    http.SetCookie(w, &http.Cookie{
        Name:     "refresh_token",
        Value:    token,
        Path:     "/",
        HttpOnly: true,
        Secure:   cfg.CookieSecure,
        SameSite: http.SameSiteStrictMode,
    })
}

func clearRefreshTokenCookie(w http.ResponseWriter, cfg config.Config) {
    http.SetCookie(w, &http.Cookie{
        Name:     "refresh_token",
        Value:    "",
        Path:     "/",
        HttpOnly: true,
        Secure:   cfg.CookieSecure,
        MaxAge:   -1,
        SameSite: http.SameSiteStrictMode,
    })
}

func writeError(w http.ResponseWriter, err error) {
    var apiErr *apperr.Error
    if errors.As(err, &apiErr) {
        writeJSON(w, apiErr.Status, map[string]string{
            "title":   apiErr.Title,
            "message": apiErr.Message,
            "code":    apiErr.Code,
        })
        return
    }
    serverErr := apperr.InternalServerError()
    writeJSON(w, http.StatusInternalServerError, map[string]string{
        "title":   serverErr.Title,
        "message": serverErr.Message,
        "code":    serverErr.Code,
    })
}

func clientIP(r *http.Request, cfg config.Config) string {
    trustedProxy := strings.TrimSpace(cfg.TrustedProxy)
    remoteHost := strings.TrimSpace(r.RemoteAddr)
    if host, _, err := net.SplitHostPort(remoteHost); err == nil {
        remoteHost = host
    }

    // Only trust X-Forwarded-For if TRUSTED_PROXY is configured and matches remote host or CIDR
    if trustedProxy != "" {
        isTrusted := false
        if _, cidrNet, err := net.ParseCIDR(trustedProxy); err == nil {
            if ip := net.ParseIP(remoteHost); ip != nil && cidrNet.Contains(ip) {
                isTrusted = true
            }
        } else if strings.EqualFold(remoteHost, trustedProxy) {
            isTrusted = true
        }

        if isTrusted {
            if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
                return strings.TrimSpace(strings.Split(forwarded, ",")[0])
            }
        }
    }
    return remoteHost
}

type tokenBucket struct {
    mu     sync.Mutex
    limit  int
    window time.Duration
    data   map[string][]time.Time
}

func newRateLimiter(limit int, window time.Duration) *tokenBucket {
    tb := &tokenBucket{limit: limit, window: window, data: make(map[string][]time.Time)}
    // Periodically clean up old IP entries to prevent memory leak
    go func() {
        ticker := time.NewTicker(window * 2)
        defer ticker.Stop()
        for range ticker.C {
            tb.mu.Lock()
            now := time.Now()
            for ip, bucket := range tb.data {
                filtered := bucket[:0]
                for _, ts := range bucket {
                    if now.Sub(ts) < tb.window {
                        filtered = append(filtered, ts)
                    }
                }
                if len(filtered) == 0 {
                    delete(tb.data, ip)
                } else {
                    tb.data[ip] = filtered
                }
            }
            tb.mu.Unlock()
        }
    }()
    return tb
}

func (b *tokenBucket) Allow(ip string) bool {
    b.mu.Lock(); defer b.mu.Unlock()
    now := time.Now()
    bucket := b.data[ip]
    filtered := bucket[:0]
    for _, ts := range bucket {
        if now.Sub(ts) < b.window {
            filtered = append(filtered, ts)
        }
    }
    if len(filtered) >= b.limit {
        b.data[ip] = filtered
        return false
    }
    filtered = append(filtered, now)
    b.data[ip] = filtered
    return true
}

func limit(l *tokenBucket, cfg config.Config, next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if !l.Allow(clientIP(r, cfg)) {
            writeError(w, apperr.TooManyRequests())
            return
        }
        next.ServeHTTP(w, r)
    }
}

func makeErrorResponse(err error) map[string]string {
    if err == nil {
        return map[string]string{"title": "Invalid request", "message": "Unknown error.", "code": "INVALID_REQUEST"}
    }
    return map[string]string{"title": "Invalid request", "message": err.Error(), "code": "INVALID_REQUEST"}
}

func debugString(v any) string { return fmt.Sprintf("%v", v) }

