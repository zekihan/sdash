package session

import (
	cfg "github.com/zekihan/sdash/config"
	"sync"
	"time"
)

var Store *StoreStruct

type StoreStruct struct {
	Sessions map[string]*Session
	sync.Mutex
}

func Init(cfg *cfg.Config) {
	go cleanUp(cfg)
	Store = &StoreStruct{
		Sessions: make(map[string]*Session),
	}
}

func (s *StoreStruct) NewSession() *Session {
	s.Lock()
	defer s.Unlock()
	sess := newSession()
	s.Sessions[sess.ID] = sess
	return sess
}

func (s *StoreStruct) GetSession(id string) *Session {
	s.Lock()
	defer s.Unlock()
	return s.Sessions[id]
}

func (s *StoreStruct) DeleteSession(id string) {
	s.Lock()
	defer s.Unlock()
	delete(s.Sessions, id)
}

func cleanUp(cfg *cfg.Config) {
	for {
		time.Sleep(cfg.WebConfig.SessionTimeout)
		Store.Lock()
		for key, val := range Store.Sessions {
			if val.IsExpired() {
				delete(Store.Sessions, key)
			}
		}
		Store.Unlock()
	}
}
