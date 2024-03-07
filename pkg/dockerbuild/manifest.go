package dockerbuild

type ImageManifestFile struct {
	Manifests []*ImageManifest
}

type ImageManifest struct {
	// The Docker Image reference, e.g. "gcr.io/my-project/my-image:sha256:abcdef..."
	Reference string

	// Services and gateways bundled in this image.
	BundledServices []string
	BundledGateways []string

	// FeatureFlags captures feature flags enabled for this image.
	FeatureFlags map[FeatureFlag]bool
}
