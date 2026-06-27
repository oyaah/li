package cmd

import (
	"github.com/oyaah/li/internal/output"
	"github.com/oyaah/li/internal/safety"
	"github.com/oyaah/li/internal/voyager"
)

// The run* functions hold the write choreography so they can be tested with
// injected client/ledger/jitter, independent of cobra wiring.

func runConnect(c *voyager.Client, l *safety.Ledger, j *safety.Jitterer, p *output.Printer, publicID, note string, force bool) error {
	if err := l.GuardOrBlock(p, "connect", force); err != nil {
		return err
	}
	urn, err := c.ResolveURN(publicID) // warm-up GET before the invite
	if err != nil {
		return err
	}
	j.Wait()
	if err := c.SendInvite(urn, note); err != nil {
		return err
	}
	if err := l.Record("connect"); err != nil {
		return err
	}
	p.Human("invite sent to %s", publicID)
	return nil
}

func runMsg(c *voyager.Client, l *safety.Ledger, j *safety.Jitterer, p *output.Printer, publicID, text string, force bool) error {
	if err := l.GuardOrBlock(p, "msg", force); err != nil {
		return err
	}
	urn, err := c.ResolveURN(publicID)
	if err != nil {
		return err
	}
	j.Wait()
	if err := c.SendMessage(urn, text); err != nil {
		return err
	}
	if err := l.Record("msg"); err != nil {
		return err
	}
	p.Human("message sent to %s", publicID)
	return nil
}

func runPost(c *voyager.Client, l *safety.Ledger, j *safety.Jitterer, p *output.Printer, text string, force bool) error {
	if err := l.GuardOrBlock(p, "post", force); err != nil {
		return err
	}
	j.Wait()
	if err := c.CreateShare(text); err != nil {
		return err
	}
	if err := l.Record("post"); err != nil {
		return err
	}
	p.Human("posted")
	return nil
}

// deps assembles the live client/ledger/jitter for the cobra commands.
func writeDeps() (*voyager.Client, *safety.Ledger, *safety.Jitterer, error) {
	c, err := authedClient()
	if err != nil {
		return nil, nil, nil, err
	}
	l, err := safety.DefaultLedger()
	if err != nil {
		return nil, nil, nil, err
	}
	return c, l, safety.NewJitter(), nil
}
