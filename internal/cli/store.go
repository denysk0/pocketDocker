package cli

import "github.com/denysk0/pocketDocker/internal/store"

var globalStore *store.Store

func SetStore(s *store.Store) {
	globalStore = s
}

func getStore() *store.Store { return globalStore }
