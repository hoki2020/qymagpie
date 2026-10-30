package gateway

import (
	"context"
	"net/http"
	"strings"

	"github.com/hoki2020/pigeon/internal/provider"
)

// pigeonUA is the User-Agent pigeon's requests go to a vendor with.
func pigeonUA() string { return "magpie/" + Version }

// userAgentFor is the User-Agent a request to p goes with, ua being the
// one the request came to pigeon with: the agent's own, as it sent it,
// where p is set to pass it on (provider.PassesUserAgent, #1851), and
// pigeon's where it isn't, where the agent sent none, and on pigeon's own
// requests — its classifier's, vision's, search's, the image tool's
// (pigeonsOwn) — which no agent sent. A User-Agent the user set among the
// provider's headers is put on after, when the request is signed, and
// stands over either.
func userAgentFor(p provider.Provider, ua string) string {
	if ua != "" && p.PassesUserAgent() && !pigeonsOwn(ua) {
		return ua
	}
	return pigeonUA()
}

// pigeonsOwn is a User-Agent pigeon's own requests carry: another pigeon's
// (magpie/…), the classifier's, vision's and search's (pigeon-router/1,
// pigeon-vision/1, pigeon-search/1), and the image tool's, which names the
// agent it draws for before its own (claude-code/2.1.0 pigeon-image/1)
// from what the agent told it, not from a User-Agent the agent sent.
func pigeonsOwn(ua string) bool {
	return strings.HasPrefix(ua, "magpie/") || strings.HasPrefix(ua, "pigeon-") || strings.HasSuffix(ua, " "+DrawAgent)
}

// callerUA is the User-Agent the request ctx is for came with (withCaller),
// "" when there is none.
func callerUA(ctx context.Context) string {
	c, _ := ctx.Value(callerCtx{}).(caller)
	return c.ua
}

// stampUA puts on req, a request to p that isn't pigeon's own forwarded
// one (an embedding, a drawing, a video), the User-Agent forwardOnce's
// carry: pigeon's, or the agent's where p passes it on. A signed-in
// account's is left to its signing, as before, and another pigeon's is
// set where it is asked.
func stampUA(ctx context.Context, p provider.Provider, h http.Header) {
	if p.Account != nil || p.IsRemotePigeon() {
		return
	}
	h.Set("User-Agent", userAgentFor(p, callerUA(ctx)))
}
