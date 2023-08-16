package bot

import "sync"

type ChatsMap struct {
	chats map[int64]interface{}
	mu    sync.RWMutex
}

func NewChatsMap() *ChatsMap {
	return &ChatsMap{
		chats: make(map[int64]interface{}),
		mu:    sync.RWMutex{},
	}
}

func (gc *ChatsMap) Add(chatID int64) {
	gc.mu.Lock()
	defer gc.mu.Unlock()
	gc.chats[chatID] = nil
}

func (gc *ChatsMap) IsGroupChat(chatID int64) bool {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	_, ok := gc.chats[chatID]
	return ok
}

var GroupChats = NewChatsMap()
