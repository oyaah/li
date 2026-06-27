package voyager

import (
	"net/url"
	"strings"
)

// SchemaVersion tags the pinned endpoint set. Bump it whenever a path or payload
// shape is re-verified against a live account. `li doctor` reports it so drift is
// attributable to a known pin.
const SchemaVersion = "2026-06-27"

// Endpoint paths, relative to BaseURL. These drift — several were already stale
// in the reference implementation (search moved to GraphQL, invite/share paths
// changed). Treat every entry here as needing live verification (see plan U2)
// and update SchemaVersion when re-pinned.
const (
	pathProfileView       = "/identity/dash/profiles"
	pathSearchBlended     = "/search/blended"          // DEPRECATED upstream; GraphQL is current. Verify live.
	pathJobSearch         = "/voyagerJobsDashJobCards" // GraphQL-migrated. Verify live.
	pathInvite            = "/growth/normInvitations"  // Commented out in reference fork. Verify live.
	pathConversations     = "/messaging/conversations"
	pathConversationEvent = "/messaging/conversations/%s/events"
	pathShare             = "/contentcreation/normShares" // Absent from reference fork. Verify live.
)

// ProfileView returns the path for a public-id profile fetch.
func ProfileView(publicID string) string {
	q := url.Values{}
	q.Set("q", "memberIdentity")
	q.Set("memberIdentity", publicID)
	return pathProfileView + "?" + q.Encode()
}

// PeopleSearch returns path + query for a blended people search. Marked
// drift-prone: LinkedIn serves this via GraphQL now; kept as the pinned target
// until live verification re-pins to the GraphQL queryId.
func PeopleSearch(keywords, title, company string) (string, url.Values) {
	q := url.Values{}
	q.Set("keywords", keywords)
	q.Set("origin", "GLOBAL_SEARCH_HEADER")
	q.Set("q", "all")
	if title != "" {
		q.Set("title", title)
	}
	if company != "" {
		q.Set("company", company)
	}
	return pathSearchBlended, q
}

// JobSearch returns path + query for a job search.
func JobSearch(keywords, location string) (string, url.Values) {
	query := "(origin:JOB_SEARCH_PAGE_OTHER_ENTRY,keywords:" + restliEscape(keywords) + ",spellCorrectionEnabled:true"
	if location != "" {
		query += ",location:" + restliEscape(location)
	}
	query += ")"
	return pathJobSearch +
		"?decorationId=com.linkedin.voyager.dash.deco.jobs.search.JobSearchCardsCollectionLite-88" +
		"&count=7&q=jobSearch&query=" + query + "&servedEventEnabled=false&start=0", nil
}

// Invite returns the path for sending a connection invitation.
func Invite() string { return pathInvite }

// Conversations returns the path for listing the inbox.
func Conversations(mailboxURN string) string {
	return "/voyagerMessagingGraphQL/graphql" +
		"?queryId=messengerConversations.9501074288a12f3ae9e3c7ea243bccbf" +
		"&variables=(query:(predicateUnions:List((conversationCategoryPredicate:(category:PRIMARY_INBOX)))),count:20,mailboxUrn:" +
		url.QueryEscape(mailboxURN) + ")"
}

func Me() string { return pathMe }

func restliEscape(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

// ConversationEvent returns the path for posting a message into a conversation.
func ConversationEvent(conversationURN string) string {
	return "/messaging/conversations/" + url.PathEscape(conversationURN) + "/events"
}

// Share returns the path for creating a feed post.
func Share() string { return pathShare }

// HealthProbes lists the endpoints `li doctor` checks for drift, with a stable
// label per category.
func HealthProbes() []string {
	return []string{"profile", "people-search", "job-search", "messaging"}
}
