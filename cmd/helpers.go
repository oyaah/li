package cmd

import (
	"context"
	"errors"
	"net/url"
	"time"

	"github.com/oyaah/li/internal/auth"
	"github.com/oyaah/li/internal/output"
	"github.com/oyaah/li/internal/voyager"
)

type readTransport interface {
	GetRaw(path string, params url.Values) ([]byte, error)
}

// authedClient loads stored credentials and returns a ready Voyager client, or
// ErrAuth (exit 77) when the user hasn't logged in.
func authedClient() (*voyager.Client, error) {
	creds, err := auth.Load()
	if err != nil {
		return nil, err
	}
	return voyager.New(creds), nil
}

func authedReadClient() (readTransport, error) {
	creds, err := auth.Load()
	if err != nil {
		return nil, err
	}
	return &resilientReadClient{
		native: voyager.New(creds),
		creds:  creds,
	}, nil
}

type resilientReadClient struct {
	native *voyager.Client
	creds  voyager.Creds
}

func (c *resilientReadClient) GetRaw(path string, params url.Values) ([]byte, error) {
	b, err := c.native.GetRaw(path, params)
	if err == nil || !errors.Is(err, output.ErrAuth) {
		return b, err
	}
	if c.creds.BrowserUserDataDir == "" {
		return nil, err
	}
	out.Human("native session rejected; retrying through logged-in Chrome")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	bt, berr := voyager.NewBrowserTransport(ctx, c.creds)
	if berr != nil {
		return nil, err
	}
	defer bt.Close()
	return bt.GetRawContext(ctx, path, params)
}
