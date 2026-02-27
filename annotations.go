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

// buildKlausAnnotations builds an OCI manifest annotation map from common
// metadata fields. Only non-empty fields produce annotations. Returns nil
// when all fields are empty.
func buildKlausAnnotations(name, description string, author *Author, homepage, sourceRepo, license string, keywords []string) map[string]string {
	annotations := make(map[string]string)

	if name != "" {
		annotations[AnnotationName] = name
	}
	if description != "" {
		annotations[AnnotationDescription] = description
	}
	if homepage != "" {
		annotations[AnnotationHomepage] = homepage
	}
	if sourceRepo != "" {
		annotations[AnnotationRepository] = sourceRepo
	}
	if license != "" {
		annotations[AnnotationLicense] = license
	}
	if len(keywords) > 0 {
		annotations[AnnotationKeywords] = strings.Join(keywords, ",")
	}
	if author != nil {
		if author.Name != "" {
			annotations[AnnotationAuthorName] = author.Name
		}
		if author.Email != "" {
			annotations[AnnotationAuthorEmail] = author.Email
		}
		if author.URL != "" {
			annotations[AnnotationAuthorURL] = author.URL
		}
	}

	if len(annotations) == 0 {
		return nil
	}
	return annotations
}

// metadataFromAnnotations parses Klaus manifest annotations back into
// common metadata fields. Missing annotation keys result in zero values.
func metadataFromAnnotations(annotations map[string]string) (name, description string, author *Author, homepage, sourceRepo, license string, keywords []string) {
	name = annotations[AnnotationName]
	description = annotations[AnnotationDescription]
	homepage = annotations[AnnotationHomepage]
	sourceRepo = annotations[AnnotationRepository]
	license = annotations[AnnotationLicense]

	if kw := annotations[AnnotationKeywords]; kw != "" {
		keywords = strings.Split(kw, ",")
	}

	authorName := annotations[AnnotationAuthorName]
	authorEmail := annotations[AnnotationAuthorEmail]
	authorURL := annotations[AnnotationAuthorURL]
	if authorName != "" || authorEmail != "" || authorURL != "" {
		author = &Author{Name: authorName, Email: authorEmail, URL: authorURL}
	}

	return
}

// toolchainFromAnnotations maps OCI manifest annotations into a Toolchain
// struct. Missing annotations result in zero-value fields. The Version
// field is not set here -- it is populated from the OCI tag by the caller.
func toolchainFromAnnotations(annotations map[string]string) Toolchain {
	name, description, author, homepage, sourceRepo, license, keywords := metadataFromAnnotations(annotations)
	return Toolchain{
		Name:        name,
		Description: description,
		Author:      author,
		Homepage:    homepage,
		SourceRepo:  sourceRepo,
		License:     license,
		Keywords:    keywords,
	}
}
