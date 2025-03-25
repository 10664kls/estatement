package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"aidanwoods.dev/go-paseto"
	sq "github.com/Masterminds/squirrel"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	rpcstatus "google.golang.org/grpc/status"
)

// ErrUserNotFound is returned when the user is not found.
var ErrUserNotFound = errors.New("user not found")

type Auth struct {
	db   *sql.DB
	aKey paseto.V4SymmetricKey
	rKey paseto.V4SymmetricKey
	zlog *zap.Logger
}

func NewAuthService(_ context.Context,
	db *sql.DB,
	aKey paseto.V4SymmetricKey,
	rKey paseto.V4SymmetricKey,
	zlog *zap.Logger) (*Auth, error) {
	if db == nil {
		return nil, errors.New("db is nil")
	}

	s := &Auth{
		db:   db,
		aKey: aKey,
		rKey: rKey,
		zlog: zlog,
	}

	return s, nil
}

func (s *Auth) Profile(ctx context.Context) (*User, error) {
	claims := ClaimsFromContext(ctx)
	user, err := getUserByUsername(ctx, s.db, claims.Username)
	if errors.Is(err, ErrUserNotFound) {
		return nil, rpcstatus.Error(
			codes.PermissionDenied,
			"You are not allowed to access this user (or it may not exist).")
	}
	return user, err
}

type LoginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Token struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

func (s *Auth) Login(ctx context.Context, req *LoginReq) (*Token, error) {
	zlog := s.zlog.With(
		zap.String("method", "Login"),
		zap.Any("username", req.Username),
	)

	zlog.Info("starting to login")

	user, err := getUserByUsername(ctx, s.db, req.Username)
	if errors.Is(err, ErrUserNotFound) {
		zlog.Info("user not found")
		return nil, rpcstatus.Error(codes.Unauthenticated, "Your credentials not valid. Please check and try again.")
	}
	if err != nil {
		zlog.Error("failed to get user by username", zap.Error(err))
		return nil, err
	}

	pass, err := user.Compare(req.Password)
	if err != nil || !pass {
		zlog.Info("password not match", zap.Error(err))
		return nil, rpcstatus.Error(codes.Unauthenticated, "Your credentials not valid. Please check and try again.")
	}

	token, err := s.genToken(user)
	if err != nil {
		zlog.Error("failed to gen token", zap.Error(err))
		return nil, err
	}

	return token, nil
}

type NewTokenReq struct {
	Token string `json:"token"`
}

func (s *Auth) RefreshToken(ctx context.Context, req *NewTokenReq) (*Token, error) {
	zlog := s.zlog.With(
		zap.String("method", "RefreshToken"),
		zap.Any("token", req.Token),
	)

	zlog.Info("starting to refresh token")

	roles := []paseto.Rule{
		paseto.NotExpired(),
		paseto.ValidAt(time.Now()),
	}

	parser := paseto.MakeParser(roles)
	token, err := parser.ParseV4Local(s.rKey, req.Token, nil)
	if err != nil {
		zlog.Info("failed to parse token", zap.Error(err))
		return nil, rpcstatus.Error(codes.Unauthenticated, "Your credentials not valid. Please check and try again.")
	}

	claims := new(Claims)
	if err := token.Get("profile", claims); err != nil {
		zlog.Info("failed to get claims", zap.Error(err))
		return nil, rpcstatus.Error(codes.Unauthenticated, "Your credentials not valid. Please check and try again.")
	}

	user, err := getUserByUsername(ctx, s.db, claims.Username)
	if errors.Is(err, ErrUserNotFound) {
		zlog.Info("user not found")
		return nil, rpcstatus.Error(codes.Unauthenticated, "Your credentials not valid. Please check and try again.")
	}
	if err != nil {
		zlog.Error("failed to get user by username", zap.Error(err))
		return nil, err
	}

	tk, err := s.genToken(user)
	if err != nil {
		zlog.Error("failed to gen token", zap.Error(err))
		return nil, err
	}

	return tk, nil
}

type Claims struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	ProductName string `json:"productName"`
}

func (s *Auth) genToken(user *User) (*Token, error) {
	now := time.Now()

	t := paseto.NewToken()
	t.SetSubject(user.Username)
	t.SetIssuedAt(now)
	t.SetNotBefore(now)
	t.SetExpiration(now.Add(time.Hour))
	t.SetFooter([]byte(now.Format(time.RFC3339)))

	if err := t.Set("profile", &Claims{
		ID:          user.ID,
		Username:    user.Username,
		ProductName: user.ProductName,
	}); err != nil {
		return nil, fmt.Errorf("failed to set claims: %w", err)
	}

	aToken := t.V4Encrypt(s.aKey, nil)

	t.SetExpiration(now.Add(time.Hour * 7 * 24))
	rToken := t.V4Encrypt(s.rKey, nil)

	return &Token{
		AccessToken:  aToken,
		RefreshToken: rToken,
	}, nil
}

type ctxKey int

const (
	claimsKey ctxKey = iota
)

func ClaimsFromContext(ctx context.Context) *Claims {
	claims, ok := ctx.Value(claimsKey).(*Claims)
	if !ok {
		return &Claims{}
	}
	return claims
}

func ContextWithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, claimsKey, claims)
}

type User struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	ProductName string `json:"productName"`
	password    string
	CreatedAt   time.Time `json:"createdAt"`
}

func (u *User) Compare(password string) (bool, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(u.password), bcrypt.DefaultCost)
	if err != nil {
		return false, err
	}

	return bcrypt.CompareHashAndPassword(hashed, []byte(password)) == nil, nil
}

func getUserByUsername(ctx context.Context, db *sql.DB, username string) (*User, error) {
	q, args := sq.Select(
		"TOP 1 USID",
		"Username",
		"pwd",
		"productnames",
		"createdate",
	).
		From("dbo.tb_user").
		PlaceholderFormat(sq.AtP).
		Where(sq.Eq{
			"rectype":  "ADD",
			"Username": username,
		}).
		MustSql()

	row := db.QueryRowContext(ctx, q, args...)
	var u User

	err := row.Scan(
		&u.ID,
		&u.Username,
		&u.password,
		&u.ProductName,
		&u.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}
