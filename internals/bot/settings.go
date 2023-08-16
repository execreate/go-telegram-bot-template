package bot

import "sync"

type Settings struct {
	myChannelID int64
	myGroupID   int64

	mu sync.RWMutex
}

func (s *Settings) SetMyChannelID(id int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.myChannelID = id
	GroupChats.Add(id)
}

func (s *Settings) GetMyChannelID() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.myChannelID
}

func (s *Settings) SetMyGroupID(id int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.myGroupID = id
	GroupChats.Add(id)
}

func (s *Settings) GetMyGroupID() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.myGroupID
}
