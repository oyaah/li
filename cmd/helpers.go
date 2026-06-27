package cmd

import (
	"github.com/oyaah/li/internal/auth"
	"github.com/oyaah/li/internal/voyager"
)

// authedClient loads stored credentials and returns a ready Voyager client, or
// ErrAuth (exit 77) when the user hasn't logged in.
func authedClient() (*voyager.Client, error) {
	creds, err := auth.Load()
	if err != nil {
		return nil, err
	}
	return voyager.New(creds), nil
}
