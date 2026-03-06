package cli

import (
	"github.com/toto/withingy/internal/api"
	"github.com/toto/withingy/internal/config"
	"github.com/toto/withingy/internal/tokens"
)

var apiClientFactory = func() (*api.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	store, err := tokens.NewStore("")
	if err != nil {
		return nil, err
	}
	return api.NewClient(cfg, store, api.WithUserAgent(userAgentString())), nil
}
