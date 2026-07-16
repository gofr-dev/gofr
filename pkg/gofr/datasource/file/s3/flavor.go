package s3

// Flavor identifies an S3-compatible object storage provider.
//
// The S3 API is implemented by several providers beyond Amazon S3 (Cloudflare R2,
// MinIO, DigitalOcean Spaces, Backblaze B2, ...). They differ in a few details that
// matter for connecting and for generating signed URLs: the addressing style
// (path-style vs virtual-hosted), the region used to sign requests, and whether the
// provider accepts the integrity checksums that aws-sdk-go-v2 adds to uploads by
// default.
//
// Selecting a Flavor applies sensible presets for that provider. Individual presets
// can still be overridden through Config (EndPoint, Region, UsePathStyle).
//
// To use a provider that is not listed here, set FlavorGeneric and supply the
// relevant Config overrides. Adding first-class support for a new provider is a
// single case in profileFor below plus a new constant.
type Flavor string

const (
	// FlavorAWS targets Amazon S3. It is the default when Config.Flavor is empty.
	FlavorAWS Flavor = ""

	// FlavorR2 targets Cloudflare R2. R2 signs requests with the fixed region
	// "auto" and rejects the SDK's default upload checksums.
	FlavorR2 Flavor = "r2"

	// FlavorMinIO targets MinIO, which requires path-style addressing.
	FlavorMinIO Flavor = "minio"

	// FlavorSpaces targets DigitalOcean Spaces.
	FlavorSpaces Flavor = "spaces"

	// FlavorB2 targets Backblaze B2's S3-compatible API.
	FlavorB2 Flavor = "b2"

	// FlavorGeneric targets any other S3-compatible endpoint. Combine it with
	// explicit Config overrides (EndPoint, Region, UsePathStyle).
	FlavorGeneric Flavor = "generic"
)

// regionAuto is the fixed signing region required by some providers (e.g. Cloudflare R2).
const regionAuto = "auto"

// profile holds the presign- and client-affecting defaults for a Flavor.
type profile struct {
	// usePathStyle selects path-style addressing (host/bucket/key) over
	// virtual-hosted-style addressing (bucket.host/key). MinIO and most
	// self-hosted S3-compatible providers require path-style.
	usePathStyle bool

	// signingRegion overrides Config.Region for request signing when non-empty.
	// Cloudflare R2, for example, always signs with the region "auto".
	signingRegion string

	// disableUploadChecksum switches the SDK's request-checksum calculation to
	// "when required". aws-sdk-go-v2 adds a CRC32 checksum to uploads by default,
	// which several S3-compatible providers reject.
	disableUploadChecksum bool
}

// profileFor returns the preset profile for a Flavor. Adding first-class support
// for a new provider is a single case here plus a new Flavor constant.
//
// Note: FlavorAWS keeps path-style addressing to preserve the behavior of earlier
// releases (which always set UsePathStyle=true). Callers that need virtual-hosted
// addressing on AWS can set Config.UsePathStyle to false.
func profileFor(flavor Flavor) profile {
	switch flavor {
	case FlavorR2:
		return profile{usePathStyle: true, signingRegion: regionAuto, disableUploadChecksum: true}
	case FlavorMinIO:
		return profile{usePathStyle: true, disableUploadChecksum: true}
	case FlavorSpaces:
		return profile{usePathStyle: false}
	case FlavorB2:
		return profile{usePathStyle: true, disableUploadChecksum: true}
	case FlavorAWS:
		return profile{usePathStyle: true}
	case FlavorGeneric:
		return profile{usePathStyle: true}
	default:
		// An unrecognized Flavor falls back to generic defaults so a custom
		// endpoint still works.
		return profile{usePathStyle: true}
	}
}

// resolveProfile returns the effective profile for a Config: the Flavor preset with
// any explicit Config overrides applied.
func resolveProfile(cfg *Config) profile {
	p := profileFor(cfg.Flavor)

	if cfg.UsePathStyle != nil {
		p.usePathStyle = *cfg.UsePathStyle
	}

	return p
}

// region returns the region to sign requests with: the profile's fixed region when
// set (e.g. R2's "auto"), otherwise the configured region.
func (p profile) region(cfg *Config) string {
	if p.signingRegion != "" {
		return p.signingRegion
	}

	return cfg.Region
}
