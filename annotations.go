package oci

import "strings"

// Klaus-specific OCI manifest annotation keys. All artifact types
// (plugins, personalities, toolchains) use these annotations to carry
// common metadata on the manifest.
const (
	AnnotationName        = "io.giantswarm.klaus.name"
	AnnotationDescription = "io.giantswarm.klaus.description"
	AnnotationHomepage    = "io.giantswarm.klaus.homepage"
	AnnotationRepository  = "io.giantswarm.klaus.repository"
	AnnotationLicense     = "io.giantswarm.klaus.license"
	AnnotationKeywords    = "io.giantswarm.klaus.keywords"
	AnnotationAuthorName  = "io.giantswarm.klaus.author.name"
	AnnotationAuthorEmail = "io.giantswarm.klaus.author.email"
	AnnotationAuthorURL   = "io.giantswarm.klaus.author.url"
)

// commonMetadata holds the shared metadata fields that all Klaus artifact
// types (plugins, personalities, toolchains) carry via OCI manifest
// annotations. Using a struct avoids error-prone positional parameters.
type commonMetadata struct {
	Name        string
	Description string
	Author      *Author
	Homepage    string
	SourceRepo  string
	License     string
	Keywords    []string
}

// buildKlausAnnotations builds an OCI manifest annotation map from common
// metadata fields. Only non-empty fields produce annotations. Returns nil
// when all fields are empty.
func buildKlausAnnotations(m commonMetadata) map[string]string {
	annotations := make(map[string]string)

	if m.Name != "" {
		annotations[AnnotationName] = m.Name
	}
	if m.Description != "" {
		annotations[AnnotationDescription] = m.Description
	}
	if m.Homepage != "" {
		annotations[AnnotationHomepage] = m.Homepage
	}
	if m.SourceRepo != "" {
		annotations[AnnotationRepository] = m.SourceRepo
	}
	if m.License != "" {
		annotations[AnnotationLicense] = m.License
	}
	if len(m.Keywords) > 0 {
		annotations[AnnotationKeywords] = strings.Join(m.Keywords, ",")
	}
	if m.Author != nil {
		if m.Author.Name != "" {
			annotations[AnnotationAuthorName] = m.Author.Name
		}
		if m.Author.Email != "" {
			annotations[AnnotationAuthorEmail] = m.Author.Email
		}
		if m.Author.URL != "" {
			annotations[AnnotationAuthorURL] = m.Author.URL
		}
	}

	if len(annotations) == 0 {
		return nil
	}
	return annotations
}

// metadataFromAnnotations parses Klaus manifest annotations back into
// common metadata fields. Missing annotation keys result in zero values.
func metadataFromAnnotations(annotations map[string]string) commonMetadata {
	m := commonMetadata{
		Name:        annotations[AnnotationName],
		Description: annotations[AnnotationDescription],
		Homepage:    annotations[AnnotationHomepage],
		SourceRepo:  annotations[AnnotationRepository],
		License:     annotations[AnnotationLicense],
	}

	if kw := annotations[AnnotationKeywords]; kw != "" {
		m.Keywords = strings.Split(kw, ",")
	}

	authorName := annotations[AnnotationAuthorName]
	authorEmail := annotations[AnnotationAuthorEmail]
	authorURL := annotations[AnnotationAuthorURL]
	if authorName != "" || authorEmail != "" || authorURL != "" {
		m.Author = &Author{Name: authorName, Email: authorEmail, URL: authorURL}
	}

	return m
}

// pluginFromAnnotations assembles a Plugin from OCI manifest annotations
// (common metadata) and a config blob (type-specific fields).
func pluginFromAnnotations(annotations map[string]string, tag string, blob pluginConfigBlob) Plugin {
	m := metadataFromAnnotations(annotations)
	return Plugin{
		Name:        m.Name,
		Description: m.Description,
		Author:      m.Author,
		Homepage:    m.Homepage,
		SourceRepo:  m.SourceRepo,
		License:     m.License,
		Keywords:    m.Keywords,
		Version:     tag,
		Skills:      blob.Skills,
		Commands:    blob.Commands,
		Agents:      blob.Agents,
		HasHooks:    blob.HasHooks,
		MCPServers:  blob.MCPServers,
		LSPServers:  blob.LSPServers,
	}
}

// personalityFromAnnotations assembles a Personality from OCI manifest
// annotations (common metadata) and a config blob (composition fields).
func personalityFromAnnotations(annotations map[string]string, tag string, blob personalityConfigBlob) Personality {
	m := metadataFromAnnotations(annotations)
	return Personality{
		Name:        m.Name,
		Description: m.Description,
		Author:      m.Author,
		Homepage:    m.Homepage,
		SourceRepo:  m.SourceRepo,
		License:     m.License,
		Keywords:    m.Keywords,
		Version:     tag,
		Toolchain:   blob.Toolchain,
		Plugins:     blob.Plugins,
	}
}

// toolchainFromAnnotations maps OCI manifest annotations into a Toolchain
// struct. Missing annotations result in zero-value fields. The Version
// field is not set here -- it is populated from the OCI tag by the caller.
func toolchainFromAnnotations(annotations map[string]string) Toolchain {
	m := metadataFromAnnotations(annotations)
	return Toolchain{
		Name:        m.Name,
		Description: m.Description,
		Author:      m.Author,
		Homepage:    m.Homepage,
		SourceRepo:  m.SourceRepo,
		License:     m.License,
		Keywords:    m.Keywords,
	}
}
