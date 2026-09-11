package store

import (
	"errors"
	"time"

	"lunar-tear/server/internal/model"
)

var ErrNotFound = errors.New("store: not found")

type Clock func() time.Time

type UserRepository interface {
	CreateUser(uuid string, platform model.ClientPlatform) (int64, error)
	GetUserByUUID(uuid string) (int64, error)
	LoadUser(userId int64) (UserState, error)
	UpdateUser(userId int64, mutate func(*UserState)) (UserState, error)
	DefaultUserId() (int64, error)
	SetFacebookId(userId int64, facebookId int64) error
	GetUserByFacebookId(facebookId int64) (int64, error)
	GetFacebookId(userId int64) (int64, error)
	ClearFacebookId(userId int64) error
	UpdateUUID(userId int64, newUuid string) error
}

type SessionRepository interface {
	CreateSession(uuid string, ttl time.Duration) (SessionState, error)
	// CreateSessionForUser issues a session for an explicit account instead of
	// looking one up by uuid. uuid is recorded on the session row as-is, so the
	// connecting client stays identifiable even when it does not own the account.
	CreateSessionForUser(userId int64, uuid string, ttl time.Duration) (SessionState, error)
	ResolveUserId(sessionKey string) (int64, error)
}

// Repository is the combined store the gRPC services are wired with.
type Repository interface {
	UserRepository
	SessionRepository
}

// Account is one player account, identified by its in-game profile name.
type Account struct {
	UserId int64
	Name   string
}
