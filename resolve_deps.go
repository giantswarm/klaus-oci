package oci

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
)

// ResolvePersonalityDeps resolves a personality's toolchain and plugin
// references by describing each dependency from the registry. The toolchain
// and all plugins are resolved concurrently, bounded by the client's
// concurrency limit.
//
// Missing or unreachable artifacts produce warnings rather than hard failures,
// allowing callers to present partial results (e.g. "plugin gs-sre: not found
// in registry").
func (c *Client) ResolvePersonalityDeps(ctx context.Context, p Personality) (*ResolvedDependencies, error) {
	result := &ResolvedDependencies{}

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(c.concurrency)

	var mu sync.Mutex

	if p.Toolchain.Repository != "" {
		g.Go(func() error {
			tc, err := c.DescribeToolchain(ctx, p.Toolchain.Ref())
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("toolchain %s: %v", p.Toolchain.Ref(), err))
				return nil
			}
			result.Toolchain = tc
			return nil
		})
	}

	plugins := make([]DescribedPlugin, len(p.Plugins))
	for i, pRef := range p.Plugins {
		g.Go(func() error {
			dp, err := c.DescribePlugin(ctx, pRef.Ref())
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("plugin %s: %v", pRef.Ref(), err))
				return nil
			}
			plugins[i] = *dp
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Collect only the successfully resolved plugins (non-zero entries).
	for _, dp := range plugins {
		if dp.Plugin.Name != "" || dp.ArtifactInfo.Ref != "" {
			result.Plugins = append(result.Plugins, dp)
		}
	}

	return result, nil
}
