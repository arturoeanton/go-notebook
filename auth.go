package main

import (
	"bufio"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
)

const (
	authFile   = "users.json"
	authCookie = "gonote_auth"
	authTTL    = 7 * 24 * time.Hour
)

// AuthStore holds the users and the HMAC secret. When no users.json exists
// the server runs in open mode (local, single user), as before.
type AuthStore struct {
	Secret string            `json:"secret"`
	Users  map[string]string `json:"users"` // name -> bcrypt hash
}

func loadAuthStore() *AuthStore {
	data, err := os.ReadFile(authFile)
	if err != nil {
		return nil
	}
	var s AuthStore
	if err := json.Unmarshal(data, &s); err != nil || s.Secret == "" || len(s.Users) == 0 {
		fmt.Fprintln(os.Stderr, "warning: ignoring invalid", authFile)
		return nil
	}
	return &s
}

func (s *AuthStore) save() error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(authFile, b, 0600)
}

func (s *AuthStore) check(user, password string) bool {
	hash, ok := s.Users[user]
	if !ok {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// --- signed session tokens ----------------------------------------------

func (s *AuthStore) makeToken(user string, exp time.Time) string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(user)) + "." + strconv.FormatInt(exp.Unix(), 10)
	return payload + "." + s.sign(payload)
}

func (s *AuthStore) sign(payload string) string {
	mac := hmac.New(sha256.New, []byte(s.Secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// verifyToken returns the user name for a valid, unexpired token.
func (s *AuthStore) verifyToken(token string) (string, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", false
	}
	payload := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(s.sign(payload)), []byte(parts[2])) {
		return "", false
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return "", false
	}
	name, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", false
	}
	return string(name), true
}

// userFromRequest returns the authenticated user, or "" in open mode.
func userFromRequest(c echo.Context, store *AuthStore) (string, bool) {
	if store == nil {
		return "", true
	}
	cookie, err := c.Cookie(authCookie)
	if err != nil {
		return "", false
	}
	return store.verifyToken(cookie.Value)
}

// --- middleware and handlers ---------------------------------------------

func authMiddleware(store *AuthStore) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if store == nil {
				return next(c)
			}
			p := c.Request().URL.Path
			if p == "/login" || strings.HasPrefix(p, "/static/") || strings.HasPrefix(p, "/assets/") {
				return next(c)
			}
			user, ok := userFromRequest(c, store)
			if !ok || user == "" {
				if c.Request().Method == http.MethodGet && !strings.Contains(p, "ws_goliveview") {
					return c.Redirect(http.StatusFound, "/login")
				}
				return c.NoContent(http.StatusUnauthorized)
			}
			c.Set("user", user)
			return next(c)
		}
	}
}

func registerAuthRoutes(e *echo.Echo, store *AuthStore) {
	e.GET("/login", func(c echo.Context) error {
		return c.HTML(http.StatusOK, loginHTML(""))
	})
	e.POST("/login", func(c echo.Context) error {
		user := strings.TrimSpace(c.FormValue("username"))
		pass := c.FormValue("password")
		if !store.check(user, pass) {
			return c.HTML(http.StatusUnauthorized, loginHTML("Invalid username or password"))
		}
		c.SetCookie(&http.Cookie{
			Name:     authCookie,
			Value:    store.makeToken(user, time.Now().Add(authTTL)),
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   int(authTTL.Seconds()),
		})
		return c.Redirect(http.StatusFound, "/")
	})
	e.GET("/logout", func(c echo.Context) error {
		c.SetCookie(&http.Cookie{Name: authCookie, Value: "", Path: "/", MaxAge: -1})
		return c.Redirect(http.StatusFound, "/login")
	})
}

func loginHTML(errMsg string) string {
	msg := ""
	if errMsg != "" {
		msg = `<div class="err">` + errMsg + `</div>`
	}
	return `<!DOCTYPE html><html lang="en"><head><meta charset="utf-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1"/>
<title>Go Notebook — sign in</title><style>
:root{--bg:#f6f7f9;--panel:#fff;--border:#dfe3e8;--text:#1f2530;--dim:#6b7280;--accent:#00add8}
@media (prefers-color-scheme: dark){:root{--bg:#0f1419;--panel:#171d26;--border:#2a3341;--text:#e5e9f0;--dim:#8b95a7}}
body{margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center;background:var(--bg);
font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;color:var(--text)}
.card{background:var(--panel);border:1px solid var(--border);border-radius:14px;padding:34px 38px;width:320px;
box-shadow:0 6px 24px rgba(0,0,0,.12)}
h1{font-size:19px;margin:0 0 4px;display:flex;align-items:center;gap:10px}
p{color:var(--dim);font-size:13px;margin:0 0 20px}
label{display:block;font-size:12px;color:var(--dim);margin:12px 0 4px}
input{width:100%;box-sizing:border-box;padding:9px 12px;border:1px solid var(--border);border-radius:8px;
background:var(--bg);color:var(--text);font-size:14px}
input:focus{outline:none;border-color:var(--accent);box-shadow:0 0 0 3px rgba(0,173,216,.15)}
button{margin-top:20px;width:100%;padding:10px;border:none;border-radius:8px;background:var(--accent);
color:#fff;font-size:14px;font-weight:600;cursor:pointer}
button:hover{filter:brightness(1.06)}
.err{margin-bottom:12px;padding:9px 12px;border-radius:8px;background:rgba(220,38,38,.1);color:#dc2626;font-size:13px}
svg{width:26px;height:26px;stroke:var(--accent);stroke-width:2;fill:none;stroke-linecap:round;stroke-linejoin:round}
</style></head><body><form class="card" method="post" action="/login">
<h1><svg viewBox="0 0 24 24"><rect x="2" y="3" width="20" height="18" rx="4"/><path d="M7 9l3 3-3 3"/><line x1="12" y1="15" x2="17" y2="15"/></svg>Go Notebook</h1>
<p>Sign in to your workspace</p>` + msg + `
<label>Username</label><input name="username" autocomplete="username" autofocus/>
<label>Password</label><input name="password" type="password" autocomplete="current-password"/>
<button type="submit">Sign in</button></form></body></html>`
}

// --- user management CLI ---------------------------------------------------

// runUserCmd implements: go-notebook user add|del|list [name] [password]
func runUserCmd(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: go-notebook user add|del|list [name] [password]")
		return 2
	}
	store := loadAuthStore()
	switch args[0] {
	case "list":
		if store == nil {
			fmt.Println("no users (open mode) —", authFile, "not found")
			return 0
		}
		for name := range store.Users {
			fmt.Println(name)
		}
		return 0
	case "add":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: go-notebook user add <name> [password]")
			return 2
		}
		name := sanitizeNotebookName(args[1])
		if name == "" {
			fmt.Fprintln(os.Stderr, "error: invalid user name")
			return 2
		}
		var pass string
		if len(args) >= 3 {
			pass = args[2]
		} else {
			fmt.Fprint(os.Stderr, "password: ")
			line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
			pass = strings.TrimSpace(line)
		}
		if len(pass) < 6 {
			fmt.Fprintln(os.Stderr, "error: password must be at least 6 characters")
			return 2
		}
		if store == nil {
			secret := make([]byte, 32)
			rand.Read(secret)
			store = &AuthStore{Secret: hex.EncodeToString(secret), Users: map[string]string{}}
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		store.Users[name] = string(hash)
		if err := store.save(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		fmt.Println("user", name, "saved — authentication is now enabled")
		return 0
	case "del":
		if store == nil || len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: go-notebook user del <name>")
			return 2
		}
		delete(store.Users, args[1])
		if len(store.Users) == 0 {
			os.Remove(authFile)
			fmt.Println("last user removed —", authFile, "deleted, running in open mode")
			return 0
		}
		if err := store.save(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		fmt.Println("user", args[1], "removed")
		return 0
	}
	fmt.Fprintln(os.Stderr, "unknown user command:", args[0])
	return 2
}
