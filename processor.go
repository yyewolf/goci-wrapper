package main

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// ImageProcessor handles image wrapping operations
type ImageProcessor struct {
	cache *Cache
}

// NewImageProcessor creates a new image processor instance
func NewImageProcessor() *ImageProcessor {
	return &ImageProcessor{
		cache: NewCache(),
	}
}

// WrapImage wraps the upstream image with the target wrapper image
func (p *ImageProcessor) WrapImage(upstreamImage, targetImage string) (v1.Image, error) {
	// Pull both images
	imageToWrap, err := p.pullImage(upstreamImage)
	if err != nil {
		return nil, err
	}

	wrapperImage, err := p.pullImage(targetImage)
	if err != nil {
		return nil, err
	}

	// Get wrapper configuration
	cfg, err := wrapperImage.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("failed to get wrapper image config: %w", err)
	}

	wrapperScriptPath, ok := cfg.Config.Labels[WrapperLabelKey]
	if !ok {
		return nil, fmt.Errorf("wrapper image missing '%s' label", WrapperLabelKey)
	}

	// Get wrapper layers
	wrapperLayers, err := wrapperImage.Layers()
	if err != nil || len(wrapperLayers) == 0 {
		return nil, fmt.Errorf("wrapper image has no layers")
	}
	wrapperLayer := wrapperLayers[len(wrapperLayers)-1] // last layer is the copied one

	// Append wrapper layer to original image
	appendedImage, err := mutate.AppendLayers(imageToWrap, wrapperLayer)
	if err != nil {
		return nil, fmt.Errorf("failed to append wrapper layer: %w", err)
	}

	// Update configuration
	cfgOrig, err := appendedImage.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("failed to get appended image config: %w", err)
	}

	// Prepare new config: wrapper becomes entrypoint, original entrypoint becomes args
	cfgNew := cfgOrig.DeepCopy()
	cfgNew.Config.Cmd = append(cfgOrig.Config.Entrypoint, cfgOrig.Config.Cmd...)
	cfgNew.Config.Entrypoint = []string{wrapperScriptPath}

	// Apply new configuration
	finalImage, err := mutate.ConfigFile(appendedImage, cfgNew)
	if err != nil {
		return nil, fmt.Errorf("failed to update image config: %w", err)
	}

	return finalImage, nil
}

// pullImage pulls an image from a remote registry
func (p *ImageProcessor) pullImage(imageRef string) (v1.Image, error) {
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return nil, fmt.Errorf("failed to parse image reference %s: %w", imageRef, err)
	}

	img, err := remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, fmt.Errorf("failed to pull image %s: %w", imageRef, err)
	}

	return img, nil
}

// Cache returns the processor's cache instance
func (p *ImageProcessor) Cache() *Cache {
	return p.cache
}
