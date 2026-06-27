package voyager

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	cdpbrowser "github.com/oyaah/li/internal/browser"
)

func BrowserSendMessage(ctx context.Context, creds Creds, publicID, text string) (string, error) {
	br, err := launchLinkedInBrowser(ctx, creds, profileURL(publicID))
	if err != nil {
		return "", err
	}
	defer br.Close()
	rawHref, err := br.Evaluate(ctx, browserMessageHrefScript)
	if err != nil {
		return "", err
	}
	var href string
	if err := json.Unmarshal(rawHref, &href); err != nil {
		return "", err
	}
	if href == "" {
		return "", usagef("message rejected by LinkedIn; no Message action is available on this profile")
	}
	if err := br.Navigate(ctx, href); err != nil {
		return "", err
	}
	res, err := evalBrowserAction(ctx, br, browserMessageComposeScript, map[string]string{"text": text})
	if err != nil {
		return "", err
	}
	switch res.Status {
	case "sent":
		return "sent", nil
	case "not_messageable":
		return "", usagef("message rejected by LinkedIn; %s", res.Detail)
	default:
		return "", driftf("message: browser fallback returned %q", res.Status)
	}
}

func BrowserSendInvite(ctx context.Context, creds Creds, publicID, note string) (string, error) {
	br, err := launchLinkedInBrowser(ctx, creds, profileURL(publicID))
	if err != nil {
		return "", err
	}
	defer br.Close()
	rawHref, err := br.Evaluate(ctx, browserConnectHrefScript)
	if err != nil {
		return "", err
	}
	var href string
	if err := json.Unmarshal(rawHref, &href); err != nil {
		return "", err
	}
	if href == "" {
		res, err := evalBrowserAction(ctx, br, browserConnectStateScript, nil)
		if err == nil {
			switch res.Status {
			case "already_pending", "already_connected":
				return res.Status, nil
			}
		}
		return "", usagef("connection request rejected by LinkedIn; no Connect action is available on this profile")
	}
	if err := br.Navigate(ctx, href); err != nil {
		return "", err
	}
	res, err := evalBrowserAction(ctx, br, browserConnectConfirmScript, map[string]string{"note": note})
	if err != nil {
		return "", err
	}
	switch res.Status {
	case "sent", "already_pending", "already_connected":
		return res.Status, nil
	case "not_connectable":
		return "", usagef("connection request rejected by LinkedIn; %s", res.Detail)
	default:
		return "", driftf("connect: browser fallback returned %q", res.Status)
	}
}

type browserActionResult struct {
	Status string `json:"status"`
	Detail string `json:"detail"`
}

func evalBrowserAction(ctx context.Context, br *cdpbrowser.Browser, script string, input any) (browserActionResult, error) {
	b, _ := json.Marshal(input)
	raw, err := br.Evaluate(ctx, fmt.Sprintf("(%s)(%s)", script, string(b)))
	if err != nil {
		return browserActionResult{}, err
	}
	var res browserActionResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return browserActionResult{}, err
	}
	return res, nil
}

func profileURL(publicID string) string {
	return "https://www.linkedin.com/in/" + strings.Trim(publicID, "/") + "/"
}

func launchLinkedInBrowser(ctx context.Context, creds Creds, startURL string) (*cdpbrowser.Browser, error) {
	br, err := cdpbrowser.Launch(ctx, cdpbrowser.Options{
		UserDataDir:    creds.BrowserUserDataDir,
		StartURL:       startURL,
		StartupTimeout: 30 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	if err := seedBrowserCookies(ctx, br, creds); err != nil {
		_ = br.Close()
		return nil, err
	}
	return br, nil
}

func seedBrowserCookies(ctx context.Context, br *cdpbrowser.Browser, creds Creds) error {
	var cookies []cdpbrowser.Cookie
	seen := map[string]bool{}
	add := func(name, value string) {
		name, value = strings.TrimSpace(name), strings.TrimSpace(value)
		if name == "" || value == "" || seen[name] {
			return
		}
		seen[name] = true
		value = strings.Trim(value, `"`)
		cookies = append(cookies, cdpbrowser.Cookie{Name: name, Value: value, Domain: ".linkedin.com", Path: "/"})
	}
	for _, part := range strings.Split(creds.Cookie, ";") {
		if i := strings.IndexByte(part, '='); i > 0 {
			add(part[:i], part[i+1:])
		}
	}
	add("li_at", creds.LiAt)
	add("JSESSIONID", creds.JSESSIONID)
	return br.SetCookies(ctx, cookies)
}

const browserMessageHrefScript = `(() => {
  const norm = s => (s || "").replace(/\s+/g, " ").trim();
  const visible = el => !!el && el.getClientRects().length > 0 && getComputedStyle(el).visibility !== "hidden";
  const action = [...document.querySelectorAll('a[href*="/messaging/compose/"]')]
    .find(el => visible(el) && /^Message(\s|$)/i.test(norm(el.innerText || el.textContent || el.getAttribute("aria-label"))));
  return action ? action.href : "";
})()`

const browserMessageComposeScript = `async (input) => {
  const sleep = ms => new Promise(r => setTimeout(r, ms));
  const norm = s => (s || "").replace(/\s+/g, " ").trim();
  const visible = el => !!el && el.getClientRects().length > 0 && getComputedStyle(el).visibility !== "hidden";
  const byText = (sel, re) => [...document.querySelectorAll(sel)].find(el => visible(el) && re.test(norm(el.innerText || el.textContent || el.getAttribute("aria-label"))));
  const waitFor = async fn => {
    const end = Date.now() + 20000;
    while (Date.now() < end) {
      const v = fn();
      if (v) return v;
      await sleep(250);
    }
    return null;
  };
  let box = await waitFor(() => [...document.querySelectorAll('[contenteditable="true"], textarea')].find(visible));
  if (!box) return {status: "not_messageable", detail: "message composer did not open"};
  box.focus();
  if (box.isContentEditable) document.execCommand("insertText", false, input.text);
  else {
    box.value = input.text;
    box.dispatchEvent(new InputEvent("input", {bubbles: true, inputType: "insertText", data: input.text}));
  }
  await sleep(500);
  let send = byText('button', /^Send$/i) || [...document.querySelectorAll('button[aria-label*="Send" i]')].find(visible);
  if (!send || send.disabled || send.getAttribute("aria-disabled") === "true") {
    return {status: "not_messageable", detail: "Send button is disabled"};
  }
  send.click();
  await sleep(2500);
  const body = norm(document.body.innerText);
  if (/couldn.t send|not sent|unable to send|can.t send/i.test(body)) {
    return {status: "not_messageable", detail: "LinkedIn UI rejected the message"};
  }
  return {status: "sent", detail: ""};
}`

const browserConnectHrefScript = `(() => {
  const norm = s => (s || "").replace(/\s+/g, " ").trim();
  const visible = el => !!el && el.getClientRects().length > 0 && getComputedStyle(el).visibility !== "hidden";
  const textOf = el => norm(el.innerText || el.textContent || el.getAttribute("aria-label"));
  const top = document.querySelector('[data-view-name="profile-top-card"]') || document.querySelector('main') || document;
  const connect = [...top.querySelectorAll('a[href*="/preload/custom-invite/"], button, a')]
    .find(el => visible(el) && /^Connect$/i.test(textOf(el)) && /Invite .* to connect/i.test(el.getAttribute("aria-label") || textOf(el)));
  return connect && connect.href ? connect.href : "";
})()`

const browserConnectStateScript = `async () => {
  const body = (document.body.innerText || "").replace(/\s+/g, " ").trim();
  if (/Pending|Invitation sent|Request sent/i.test(body)) return {status: "already_pending", detail: ""};
  if (/\b1st\b/.test(body)) return {status: "already_connected", detail: ""};
  return {status: "not_connectable", detail: "no Connect action is available on this profile"};
}`

const browserConnectConfirmScript = `async (input) => {
  const sleep = ms => new Promise(r => setTimeout(r, ms));
  const norm = s => (s || "").replace(/\s+/g, " ").trim();
  const visible = el => !!el && el.getClientRects().length > 0 && getComputedStyle(el).visibility !== "hidden";
  const textOf = el => norm(el.innerText || el.textContent || el.getAttribute("aria-label"));
  const byText = (sel, re) => [...document.querySelectorAll(sel)].find(el => visible(el) && re.test(textOf(el)));
  const waitFor = async fn => {
    const end = Date.now() + 15000;
    while (Date.now() < end) {
      const v = fn();
      if (v) return v;
      await sleep(250);
    }
    return null;
  };
  const body = () => norm(document.body.innerText);
  if (/Pending|Invitation sent|Request sent/i.test(body())) return {status: "already_pending", detail: ""};
  if (/\b1st\b/.test(body())) return {status: "already_connected", detail: ""};
  if (input.note) {
    const addNote = byText('button', /Add a note/i);
    if (addNote) {
      addNote.click();
      await sleep(500);
    }
    const note = await waitFor(() => [...document.querySelectorAll('textarea')].find(visible));
    if (note) {
      note.value = input.note;
      note.dispatchEvent(new InputEvent("input", {bubbles: true, inputType: "insertText", data: input.note}));
    }
  }
  const send = await waitFor(() => byText('button', /^(Send|Send now|Send invitation|Send without a note)$/i));
  if (!send || send.disabled || send.getAttribute("aria-disabled") === "true") {
    if (/Pending|Invitation sent|Request sent/i.test(body())) return {status: "already_pending", detail: ""};
    return {status: "not_connectable", detail: "Send invitation button is unavailable"};
  }
  send.click();
  await sleep(2500);
  if (/Pending|Invitation sent|Request sent/i.test(body())) return {status: "sent", detail: ""};
  return {status: "sent", detail: ""};
}`
