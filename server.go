package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/akutz/memconn"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// GociWrapperServer represents the main server
type GociWrapperServer struct {
	processor       *ImageProcessor
	localRegistry   http.Handler
	memoryTransport *http.Transport
}

// NewGociWrapperServer creates a new server instance
func NewGociWrapperServer() (*GociWrapperServer, error) {
	processor := NewImageProcessor()

	localRegistry := registry.New()

	memoryTransport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return memconn.Dial(MemoryNetwork, MemoryAddress)
		},
	}

	return &GociWrapperServer{
		processor:       processor,
		localRegistry:   localRegistry,
		memoryTransport: memoryTransport,
	}, nil
}

// imageSlashTagToImage converts "repo/name/tag" to "repo/name:tag"
func (s *GociWrapperServer) imageSlashTagToImage(image string) string {
	lastSlash := strings.LastIndex(image, "/")
	if lastSlash == -1 {
		return image
	}
	return image[:lastSlash] + ":" + image[lastSlash+1:]
}

// ParseImageRef parses the request path and extracts image information
func (s *GociWrapperServer) ParseImageRef(path string) (*ImageRef, error) {
	matches := ManifestRequestRegexp.FindStringSubmatch(path)
	if len(matches) != 4 {
		return nil, fmt.Errorf("invalid path, expected /wrap/{upstream-image}/with/{target-image}")
	}

	if !strings.Contains(matches[1], "/") {
		return nil, fmt.Errorf("invalid upstream image format")
	}
	if !strings.Contains(matches[2], "/") {
		return nil, fmt.Errorf("invalid target image format")
	}

	return &ImageRef{
		Path:          fmt.Sprintf("/wrap/%s/with/%s", matches[1], matches[2]),
		UpstreamImage: s.imageSlashTagToImage(matches[1]),
		TargetImage:   s.imageSlashTagToImage(matches[2]),
	}, nil
}

// /v2/wrap/mirror.gcr.io/woodpeckerci/plugin-ready-release-go/3.4.0/with/registry.yewolf.fr/test/test/latest/manifests/latest
// /v2/wrap/mirror.gcr.io/woodpeckerci/plugin-ready-release-go/3.4.0/with/registry.yewolf.fr/test/test/latest/manifests/latest

// HandleWrapRequest handles the image wrapping HTTP request
func (s *GociWrapperServer) HandleWrapRequest(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received request: %s %s", r.Method, r.URL.Path)
	// Match wrapping requests
	if !WrappingRegexp.MatchString(r.URL.Path) {
		s.localRegistry.ServeHTTP(w, r)
		return
	}

	// Do the wrapping only for manifest requests
	if !ManifestRequestRegexp.MatchString(r.URL.Path) {
		// Proxy to local registry
		s.localRegistry.ServeHTTP(w, r)
		return
	}

	imageRef, err := s.ParseImageRef(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Printf("Failed to parse image reference: %v", err)
		return
	}

	cacheKey := s.processor.Cache().Key(imageRef.UpstreamImage, imageRef.TargetImage)
	if s.processor.Cache().Has(cacheKey) {
		// Already wrapped, proxy to local registry
		s.localRegistry.ServeHTTP(w, r)
		return
	}

	// Wrap the image
	finalImage, err := s.processor.WrapImage(imageRef.UpstreamImage, imageRef.TargetImage)
	if err != nil {
		log.Printf("Failed to wrap image: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Push final image to in-memory registry
	targetRef, err := name.ParseReference("localhost:5000" + imageRef.Path)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to parse target reference: %v", err), http.StatusInternalServerError)
		log.Printf("Failed to parse target reference: %v", err)
		return
	}

	pushOpts := []remote.Option{
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
		remote.WithTransport(s.memoryTransport),
	}

	if err := remote.Write(targetRef, finalImage, pushOpts...); err != nil {
		http.Error(w, fmt.Sprintf("failed to push wrapped image: %v", err), http.StatusInternalServerError)
		log.Printf("Failed to push wrapped image: %v", err)
		return
	}

	s.processor.Cache().Set(cacheKey)

	// Proxy to local registry
	s.localRegistry.ServeHTTP(w, r)
}

// startMemoryRegistry starts the in-memory registry server
func (s *GociWrapperServer) startMemoryRegistry() error {
	lis, err := memconn.Listen(MemoryNetwork, MemoryAddress)
	if err != nil {
		return fmt.Errorf("failed to create memory listener: %w", err)
	}

	go func() {
		log.Println("Starting in-memory registry")
		if err := http.Serve(lis, s.localRegistry); err != nil {
			log.Printf("Memory registry server error: %v", err)
		}
	}()

	return nil
}

// Start starts the Goci Wrapper server
func (s *GociWrapperServer) Start() error {
	// Start the in-memory registry
	if err := s.startMemoryRegistry(); err != nil {
		return err
	}

	// Set up HTTP handler
	http.HandleFunc("/", s.HandleWrapRequest)

	log.Printf("Goci Wrapper server starting on %s", ServerPort)
	return http.ListenAndServe(ServerPort, nil)
}
