package voyager

import (
	"fmt"
	"net/url"
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
	pathProfileView       = "/identity/profiles/%s/profileView"
	pathSearchBlended     = "/search/blended"        // DEPRECATED upstream; GraphQL is current. Verify live.
	pathJobSearch         = "/voyagerJobsDashJobCards" // GraphQL-migrated. Verify live.
	pathInvite            = "/growth/normInvitations"  // Commented out in reference fork. Verify live.
	pathConversations     = "/messaging/conversations"
	pathConversationEvent = "/messaging/conversations/%s/events"
	pathShare             = "/contentcreation/normShares" // Absent from reference fork. Verify live.
)

// ProfileView returns the path for a public-id profile fetch.
func ProfileView(publicID string) string {
	return fmt.Sprintf(pathProfileView, url.PathEscape(publicID))
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
	q := url.Values{}
	q.Set("keywords", keywords)
	if location != "" {
		q.Set("location", location)
	}
	return pathJobSearch, q
}

// Invite returns the path for sending a connection invitation.
func Invite() string { return pathInvite }

// Conversations returns the path for listing the inbox.
func Conversations() string { return pathConversations }

// ConversationEvent returns the path for posting a message into a conversation.
func ConversationEvent(conversationURN string) string {
	return fmt.Sprintf(pathConversationEvent, url.PathEscape(conversationURN))
}

// Share returns the path for creating a feed post.
func Share() string { return pathShare }

// HealthProbes lists the endpoints `li doctor` checks for drift, with a stable
// label per category.
func HealthProbes() []string {
	return []string{"profile", "people-search", "job-search", "messaging"}
}
