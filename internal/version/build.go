package version

// Current is set at build time via -ldflags.
var Current = "0.0.0-dev"

// BundledNode is the Ithiltir-node release bundled with this Dash build.
var BundledNode = "unknown"

func CurrentString() string {
	return Current
}

func BundledNodeString() string {
	return BundledNode
}
