package theme

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const DefaultID = "default"

const (
	AdminShellSidebar = "sidebar"
	AdminShellTopbar  = "topbar"

	AdminFrameLayered = "layered"
	AdminFrameFlat    = "flat"

	DashboardSummaryCards = "cards"
	DashboardSummaryStrip = "strip"

	DashboardDensityComfortable = "comfortable"
	DashboardDensityCompact     = "compact"
)

var themeIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

type Manifest struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Author      string `json:"author"`
	Description string `json:"description"`
	Skin        Skin   `json:"skin"`
}

type Skin struct {
	Admin     AdminSkin     `json:"admin"`
	Dashboard DashboardSkin `json:"dashboard"`
}

type AdminSkin struct {
	Shell string `json:"shell"`
	Frame string `json:"frame"`
}

type DashboardSkin struct {
	Summary string `json:"summary"`
	Density string `json:"density"`
}

func IsDefault(id string) bool {
	return strings.TrimSpace(id) == DefaultID
}

func IsReservedID(id string) bool {
	return IsDefault(id) || IsBuiltin(id)
}

func ParseManifest(raw []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return Manifest{}, fmt.Errorf("decode theme manifest: %w", err)
	}
	m.ID = strings.TrimSpace(m.ID)
	m.Name = strings.TrimSpace(m.Name)
	m.Version = strings.TrimSpace(m.Version)
	m.Author = strings.TrimSpace(m.Author)
	m.Description = strings.TrimSpace(m.Description)
	m.normalize()

	if err := ValidateID(m.ID); err != nil {
		return Manifest{}, err
	}
	if m.Name == "" {
		return Manifest{}, errors.New("theme name is required")
	}
	if m.Version == "" {
		return Manifest{}, errors.New("theme version is required")
	}
	if err := m.validate(); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func ValidateID(id string) error {
	id = strings.TrimSpace(id)
	if !themeIDPattern.MatchString(id) {
		return errors.New("theme id must match [a-z0-9][a-z0-9_-]{0,63}")
	}
	return nil
}

func NormalizeID(raw string) (string, error) {
	id := strings.TrimSpace(raw)
	if err := ValidateID(id); err != nil {
		return "", err
	}
	return id, nil
}

func NormalizeActiveID(raw string) (string, error) {
	id := strings.TrimSpace(raw)
	if id == "" {
		return DefaultID, nil
	}
	if err := ValidateID(id); err != nil {
		return "", err
	}
	return id, nil
}

func (m *Manifest) normalize() {
	m.Skin.Admin.normalize()
	m.Skin.Dashboard.normalize()
}

func (m Manifest) validate() error {
	if err := m.Skin.Admin.validate(); err != nil {
		return err
	}
	if err := m.Skin.Dashboard.validate(); err != nil {
		return err
	}
	return nil
}

func (s *AdminSkin) normalize() {
	if strings.TrimSpace(s.Shell) == "" {
		s.Shell = AdminShellSidebar
	}
	if strings.TrimSpace(s.Frame) == "" {
		s.Frame = AdminFrameLayered
	}
}

func (s AdminSkin) validate() error {
	switch s.Shell {
	case AdminShellSidebar, AdminShellTopbar:
	default:
		return fmt.Errorf("skin.admin.shell must be one of %q or %q", AdminShellSidebar, AdminShellTopbar)
	}

	switch s.Frame {
	case AdminFrameLayered, AdminFrameFlat:
	default:
		return fmt.Errorf("skin.admin.frame must be one of %q or %q", AdminFrameLayered, AdminFrameFlat)
	}
	return nil
}

func (s *DashboardSkin) normalize() {
	if strings.TrimSpace(s.Summary) == "" {
		s.Summary = DashboardSummaryCards
	}
	if strings.TrimSpace(s.Density) == "" {
		s.Density = DashboardDensityComfortable
	}
}

func (s DashboardSkin) validate() error {
	switch s.Summary {
	case DashboardSummaryCards, DashboardSummaryStrip:
	default:
		return fmt.Errorf(
			"skin.dashboard.summary must be one of %q or %q",
			DashboardSummaryCards,
			DashboardSummaryStrip,
		)
	}

	switch s.Density {
	case DashboardDensityComfortable, DashboardDensityCompact:
	default:
		return fmt.Errorf(
			"skin.dashboard.density must be one of %q or %q",
			DashboardDensityComfortable,
			DashboardDensityCompact,
		)
	}
	return nil
}
