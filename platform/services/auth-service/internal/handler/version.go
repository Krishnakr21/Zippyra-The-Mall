package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/zippyra/platform/services/auth-service/internal/model"
	"github.com/zippyra/platform/shared/errors"
)

var semverRegex = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// VersionHandler handles the version-check endpoint.
type VersionHandler struct {
	appVersionRepo appVersionRepo
}

type appVersionRepo interface {
	GetLatest(ctx context.Context, platform string) (*model.AppVersion, error)
}

// NewVersionHandler creates a new VersionHandler.
func NewVersionHandler(appVersionRepo appVersionRepo) *VersionHandler {
	return &VersionHandler{appVersionRepo: appVersionRepo}
}

type versionCheckResponse struct {
	IsSupported         bool    `json:"is_supported"`
	IsForceUpdate       bool    `json:"is_force_update"`
	LatestVersion       string  `json:"latest_version"`
	MinSupportedVersion string  `json:"min_supported_version"`
	ReleaseNotes        *string `json:"release_notes,omitempty"`
}

// VersionCheck handles GET /v1/auth/version-check
func (h *VersionHandler) VersionCheck(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")

	platform := r.URL.Query().Get("platform")
	version := r.URL.Query().Get("version")

	// Validate platform
	if platform != "android" && platform != "ios" {
		errors.WriteValidationError(w, "platform", "must be 'android' or 'ios'", requestID)
		return
	}

	// Validate version format
	if !semverRegex.MatchString(version) {
		errors.WriteValidationError(w, "version", "must be valid semver (e.g. 1.0.0)", requestID)
		return
	}

	// Query latest version
	av, err := h.appVersionRepo.GetLatest(r.Context(), platform)
	if err != nil {
		log.Error().Err(err).Str("platform", platform).Msg("version lookup failed")
		errors.WriteInternalError(w, requestID)
		return
	}

	// Compare versions
	isForceUpdate := compareSemver(version, av.MinSupportedVersion) < 0
	isSupported := !isForceUpdate

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(versionCheckResponse{
		IsSupported:         isSupported,
		IsForceUpdate:       isForceUpdate,
		LatestVersion:       av.Version,
		MinSupportedVersion: av.MinSupportedVersion,
		ReleaseNotes:        av.ReleaseNotes,
	})
}

// compareSemver compares two semver strings.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func compareSemver(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	for i := 0; i < 3; i++ {
		aNum := parseVersionPart(aParts, i)
		bNum := parseVersionPart(bParts, i)
		if aNum < bNum {
			return -1
		}
		if aNum > bNum {
			return 1
		}
	}
	return 0
}

func parseVersionPart(parts []string, index int) int {
	if index >= len(parts) {
		return 0
	}
	n := 0
	for _, c := range parts[index] {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}
