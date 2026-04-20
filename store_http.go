package oci

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Known OCI manifest media types. Used to decide whether to fetch via
// /v2/{repo}/manifests/<digest> or /v2/{repo}/blobs/<digest>. Extend here
// if new manifest shapes appear.
var manifestMediaTypes = map[string]struct{}{
	ocispec.MediaTypeImageManifest:                              {},
	ocispec.MediaTypeImageIndex:                                 {},
	"application/vnd.docker.distribution.manifest.v2+json":      {},
	"application/vnd.docker.distribution.manifest.list.v2+json": {},
	"application/vnd.docker.distribution.manifest.v1+json":      {},
	"application/vnd.docker.distribution.manifest.v1+prettyjws": {},
}

func isManifestMediaType(mt string) bool {
	_, ok := manifestMediaTypes[mt]
	return ok
}

func (d *diskCache) scheme() string {
	if d.plainHTTP {
		return "http"
	}
	return "https"
}

// probeTag issues HEAD /v2/{repo}/manifests/{tag} and captures digest,
// size and media type from the response headers. ifNoneMatch is a hint
// for registries that support ETag on manifest HEADs; most do not, so
// it is best effort.
func (d *diskCache) probeTag(ctx context.Context, host, repo, tag, ifNoneMatch string) (ocispec.Descriptor, string, error) {
	u := fmt.Sprintf("%s://%s/v2/%s/manifests/%s", d.scheme(), host, repo, url.PathEscape(tag))
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, u, nil)
	if err != nil {
		return ocispec.Descriptor{}, "", err
	}
	// Prefer OCI manifests, but fall back to docker manifests for older
	// registries.
	req.Header.Set("Accept", strings.Join([]string{
		ocispec.MediaTypeImageManifest,
		ocispec.MediaTypeImageIndex,
		"application/vnd.docker.distribution.manifest.v2+json",
		"application/vnd.docker.distribution.manifest.list.v2+json",
	}, ", "))
	if ifNoneMatch != "" {
		req.Header.Set("If-None-Match", ifNoneMatch)
	}
	resp, err := d.authClient.Do(req)
	if err != nil {
		return ocispec.Descriptor{}, "", fmt.Errorf("HEAD %s: %w", u, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotModified {
		return ocispec.Descriptor{}, "", fmt.Errorf("HEAD %s: unexpected status %s", u, resp.Status)
	}
	dcd := resp.Header.Get("Docker-Content-Digest")
	if dcd == "" {
		return ocispec.Descriptor{}, "", fmt.Errorf("HEAD %s: missing Docker-Content-Digest", u)
	}
	dgst := digest.Digest(dcd)
	if err := dgst.Validate(); err != nil {
		return ocispec.Descriptor{}, "", fmt.Errorf("HEAD %s: invalid Docker-Content-Digest: %w", u, err)
	}
	mediaType := parseContentType(resp.Header.Get("Content-Type"))
	if !isManifestMediaType(mediaType) {
		// Either the server returned an unexpected/generic Content-Type
		// (e.g. "application/json") or one we do not recognise. Fall
		// back to the OCI manifest type; we only accept allowlisted
		// types so an arbitrary server header cannot steer us toward
		// the /blobs/ path in fetchContent.
		mediaType = ocispec.MediaTypeImageManifest
	}
	size := resp.ContentLength
	if size < 0 {
		// Chunked responses (common for some registries) lack a
		// Content-Length. Leave Size zero so callers treat it as
		// unknown rather than caching -1 as a permanent fact.
		size = 0
	}
	desc := ocispec.Descriptor{
		Digest:    dgst,
		Size:      size,
		MediaType: mediaType,
	}
	return desc, resp.Header.Get("ETag"), nil
}

// parseContentType returns the media-type portion of a Content-Type header,
// stripping any parameters (charset, boundary, etc.). Returns "" when the
// header is empty or malformed.
func parseContentType(v string) string {
	if v == "" {
		return ""
	}
	mt, _, err := mime.ParseMediaType(v)
	if err != nil {
		return ""
	}
	return mt
}

// fetchTags paginates /v2/{repo}/tags/list. If ifNoneMatch is non-empty,
// it is sent as If-None-Match on the first page request; a 304 on the
// first page short-circuits pagination and notModified is true. Later
// pages are fetched unconditionally.
func (d *diskCache) fetchTags(ctx context.Context, host, repo, ifNoneMatch string) (tags []string, etag string, notModified bool, err error) {
	next := fmt.Sprintf("%s://%s/v2/%s/tags/list", d.scheme(), host, repo)
	first := true
	for next != "" {
		req, rerr := http.NewRequestWithContext(ctx, http.MethodGet, next, nil)
		if rerr != nil {
			return nil, "", false, rerr
		}
		req.Header.Set("Accept", "application/json")
		if first && ifNoneMatch != "" {
			req.Header.Set("If-None-Match", ifNoneMatch)
		}
		resp, rerr := d.authClient.Do(req)
		if rerr != nil {
			return nil, "", false, fmt.Errorf("GET %s: %w", next, rerr)
		}
		if first && resp.StatusCode == http.StatusNotModified {
			resp.Body.Close()
			return nil, ifNoneMatch, true, nil
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, "", false, fmt.Errorf("GET %s: unexpected status %s", next, resp.Status)
		}
		if first {
			etag = resp.Header.Get("ETag")
		}
		var page struct {
			Tags []string `json:"tags"`
		}
		dec := json.NewDecoder(io.LimitReader(resp.Body, 32*1024*1024))
		derr := dec.Decode(&page)
		resp.Body.Close()
		if derr != nil {
			return nil, "", false, fmt.Errorf("parsing tag list from %s: %w", next, derr)
		}
		tags = append(tags, page.Tags...)
		next = parseNextLink(resp.Header.Get("Link"), next)
		first = false
	}
	return tags, etag, false, nil
}

// fetchCatalog paginates /v2/_catalog, filtering to repos sharing the
// given prefix (with trailing slash). Uses the registry spec's `last`
// parameter to seek past entries that sort before the prefix.
func (d *diskCache) fetchCatalog(ctx context.Context, host, prefix string) ([]string, error) {
	seek := strings.TrimSuffix(prefix, "/")
	base := fmt.Sprintf("%s://%s/v2/_catalog", d.scheme(), host)
	next := base
	if seek != "" {
		next = base + "?last=" + url.QueryEscape(seek)
	}
	var repos []string
	for next != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, next, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		resp, err := d.authClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("GET %s: %w", next, err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("GET %s: unexpected status %s", next, resp.Status)
		}
		var page struct {
			Repositories []string `json:"repositories"`
		}
		dec := json.NewDecoder(io.LimitReader(resp.Body, 32*1024*1024))
		derr := dec.Decode(&page)
		resp.Body.Close()
		if derr != nil {
			return nil, fmt.Errorf("parsing catalog from %s: %w", next, derr)
		}
		done := false
		for _, name := range page.Repositories {
			if prefix != "" {
				if !strings.HasPrefix(name, prefix) {
					if name > prefix {
						done = true
						break
					}
					continue
				}
			}
			repos = append(repos, host+"/"+name)
		}
		if done {
			break
		}
		next = parseNextLink(resp.Header.Get("Link"), next)
	}
	return repos, nil
}

// fetchContent fetches a manifest or blob from the registry by digest.
func (d *diskCache) fetchContent(ctx context.Context, host, repo string, desc ocispec.Descriptor) (io.ReadCloser, error) {
	var path string
	if isManifestMediaType(desc.MediaType) {
		path = "manifests"
	} else {
		path = "blobs"
	}
	u := fmt.Sprintf("%s://%s/v2/%s/%s/%s", d.scheme(), host, repo, path, desc.Digest.String())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	if desc.MediaType != "" {
		req.Header.Set("Accept", desc.MediaType)
	}
	resp, err := d.authClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", u, err)
	}
	if resp.StatusCode != http.StatusOK {
		// Drain a small portion of the body to free the connection,
		// but do not surface the response body: some registries echo
		// request headers (including auth artefacts) in error bodies.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
		resp.Body.Close()
		return nil, fmt.Errorf("GET %s: unexpected status %s", u, resp.Status)
	}
	return resp.Body, nil
}

// parseNextLink parses the rel="next" entry of a distribution-spec Link
// header. Returns "" if no next page is indicated. The base URL is used
// to resolve relative references.
func parseNextLink(header string, base string) string {
	if header == "" {
		return ""
	}
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, `rel="next"`) {
			continue
		}
		start := strings.Index(part, "<")
		end := strings.Index(part, ">")
		if start < 0 || end < 0 || end <= start {
			continue
		}
		raw := part[start+1 : end]
		link, err := url.Parse(raw)
		if err != nil {
			continue
		}
		if link.IsAbs() {
			return link.String()
		}
		baseURL, err := url.Parse(base)
		if err != nil {
			return ""
		}
		return baseURL.ResolveReference(link).String()
	}
	return ""
}
