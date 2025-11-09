package main

import "regexp"

const (
	WrapperLabelKey = "org.goci.wrapper"
	MemoryNetwork   = "memu"
	MemoryAddress   = "goci-wrapper-registry"
	ServerPort      = ":5000"
)

var (
	WrappingRegexp        = regexp.MustCompile(`^/v2/wrap/(.+)/with/(.+)$`)
	ManifestRequestRegexp = regexp.MustCompile(`^/v2/wrap/(.+)/with/(.+)/manifests/(.+)$`)
)

// ImageRef represents parsed image reference information
type ImageRef struct {
	Path          string
	UpstreamImage string
	TargetImage   string
}
