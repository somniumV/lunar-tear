package store

import (
	"log"
	"time"

	"lunar-tear/server/internal/model"
)

// Pinned forces every identity lookup to resolve to a single account.
//
// It exists for single-account setups (`lunar-tear --user <name>`): the game
// client picks its own uuid and the server normally maps uuid -> account, so a
// reinstalled or second client would land on a different save. Wrapping the
// store means every path that turns a client-supplied identifier (uuid,
// session key, Facebook id) into a user id returns the pinned account instead,
// no matter how many accounts or stale sessions the database holds.
//
// Calls that take an explicit user id (LoadUser, UpdateUser, ...) are passed
// through untouched — by the time they run, the id has already come from one of
// the pinned lookups below.
type Pinned struct {
	Repository
	userId int64
}

var _ Repository = (*Pinned)(nil)

// NewPinned wraps inner so that all client logins resolve to userId.
func NewPinned(inner Repository, userId int64) *Pinned {
	return &Pinned{Repository: inner, userId: userId}
}

// UserId is the account every client is pinned to.
func (p *Pinned) UserId() int64 { return p.userId }

// CreateUser hands back the pinned account rather than registering a new one,
// so a fresh client install does not litter the database with throwaway saves.
func (p *Pinned) CreateUser(uuid string, _ model.ClientPlatform) (int64, error) {
	log.Printf("[store] pinned: registration from uuid=%s mapped to user_id=%d (no account created)", uuid, p.userId)
	return p.userId, nil
}

func (p *Pinned) GetUserByUUID(uuid string) (int64, error) {
	return p.userId, nil
}

func (p *Pinned) DefaultUserId() (int64, error) {
	return p.userId, nil
}

func (p *Pinned) GetUserByFacebookId(_ int64) (int64, error) {
	return p.userId, nil
}

func (p *Pinned) SetFacebookId(_ int64, facebookId int64) error {
	return p.Repository.SetFacebookId(p.userId, facebookId)
}

func (p *Pinned) GetFacebookId(_ int64) (int64, error) {
	return p.Repository.GetFacebookId(p.userId)
}

func (p *Pinned) ClearFacebookId(_ int64) error {
	return p.Repository.ClearFacebookId(p.userId)
}

func (p *Pinned) UpdateUUID(_ int64, newUuid string) error {
	return p.Repository.UpdateUUID(p.userId, newUuid)
}

func (p *Pinned) CreateSession(uuid string, ttl time.Duration) (SessionState, error) {
	return p.Repository.CreateSessionForUser(p.userId, uuid, ttl)
}

func (p *Pinned) CreateSessionForUser(_ int64, uuid string, ttl time.Duration) (SessionState, error) {
	return p.Repository.CreateSessionForUser(p.userId, uuid, ttl)
}

// ResolveUserId accepts any session key, including expired ones and keys issued
// for a different account before the server was pinned.
func (p *Pinned) ResolveUserId(_ string) (int64, error) {
	return p.userId, nil
}
