package douyin

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ErrNoMediaAddress is what a caller gets when a payload names no address it can
// fetch. It is a sentinel because the worker answers it differently from a
// transport failure: one means the provider's answer did not contain a video, the
// other means fetching it broke.
var ErrNoMediaAddress = errors.New("douyin payload carries no playable address")

// ErrNoDetailItem is what a caller gets when a detail response carries no video
// item to read.
//
// It is separate from `ErrNoMediaAddress` because the two are different answers
// about different things: this one says the response's shape was not what the
// endpoint returns, and reading further into it would be guessing. The provider
// answers a request it refuses with a success-shaped envelope and no item, so
// without this the refusal would surface as "there is no video here" — which is
// also what a real video with no play address looks like.
var ErrNoDetailItem = errors.New("douyin detail response carries no video item")

// DetailItem reads the video item out of a detail response.
//
// `FetchByURL` answers with the item under `data`, which is the envelope
// `contentpool/service/discovery_crawler.go` already reads for the same endpoint.
// That is the shape this accepts and the only one: `FindAuthor` nests its answer
// under `data.aweme_detail` instead, so a fallback to that shape would be this
// package guessing which endpoint answered it. A response whose `data` is not an
// object is refused rather than searched.
//
// The item is handed to `MediaAddress` unchanged, and it is not inspected here: a
// response from another endpoint therefore comes back as an item with no video in
// it, and is refused one step later by `MediaAddress`. That is deliberate. Telling
// the envelopes apart would mean this function deciding which endpoint answered,
// and the nesting in the response is the only thing that says so.
func DetailItem(payload map[string]any) (map[string]any, error) {
	data, ok := payload["data"].(map[string]any)
	if !ok {
		return nil, ErrNoDetailItem
	}
	return data, nil
}

// MediaAddress reads the address of a video's playable source out of a payload the
// Douyin data API returned.
//
// The address is a short-lived signed URL: its query string is the credential. This
// function hands it back and does nothing else with it — it is not logged, not put
// into an error message — and the caller is bound by the same rule.
//
// `video.play_addr.url_list` is the only place it is read from. Real payloads also
// carry `play_addr_h264`, `download_addr` and a per-bitrate address; none is used,
// because which of them holds the source rather than a re-encoded or watermarked
// rendition has not been measured against this provider, and choosing one now would
// be a guess presented as a fallback. The rest of `url_list` is not walked either:
// its entries are equivalent addresses, so a fetch that fails is answered by the
// worker re-resolving on its next attempt, not by trying the next entry inside one
// attempt.
func MediaAddress(payload map[string]any) (string, error) {
	video, _ := payload["video"].(map[string]any)
	playAddress, _ := video["play_addr"].(map[string]any)
	// `json.Unmarshal` reads a JSON array into `any` as `[]any`, so that is the only
	// shape accepted here: a `[]string` is something the wire never produces, and a
	// branch no payload can reach is a branch nothing keeps honest.
	candidates, _ := playAddress["url_list"].([]any)
	for _, candidate := range candidates {
		text, ok := candidate.(string)
		if !ok || strings.TrimSpace(text) == "" {
			// A hole in the list is a hole, not a decision: the entries are equivalent
			// addresses, so the first one that is actually an address is the answer.
			continue
		}
		return checkedAddress(strings.TrimSpace(text))
	}
	return "", ErrNoMediaAddress
}

// checkedAddress refuses an address the source client could not fetch, and does so
// here, while the address is in scope.
//
// A scheme other than HTTP or HTTPS is refused because the fetch is an HTTP
// request: an address naming another scheme would otherwise steer the worker at a
// protocol its client does not speak. An address with no host is refused for the
// plainer reason that there is nothing to fetch.
//
// The refusal never carries the address, which is why `url.Parse`'s error is not
// passed through: its text is the address itself, in the form `parse "<url>": …`.
// Only the cause beneath it travels.
func checkedAddress(address string) (string, error) {
	parsed, err := url.Parse(address)
	if err != nil {
		var parseErr *url.Error
		if errors.As(err, &parseErr) && parseErr.Err != nil {
			return "", fmt.Errorf("%w: %v", ErrNoMediaAddress, parseErr.Err)
		}
		return "", ErrNoMediaAddress
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("%w: the address is not an http or https URL", ErrNoMediaAddress)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("%w: the address names no host", ErrNoMediaAddress)
	}
	// The address goes back exactly as it arrived: a signed URL carries its signature
	// in the query string, and rebuilding it from the parsed parts would re-encode
	// the very thing the provider is checking.
	return address, nil
}
