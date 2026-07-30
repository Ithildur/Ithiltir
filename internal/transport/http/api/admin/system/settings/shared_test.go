package settings

import "testing"

func TestSiteBrandPatchValidatesOnlySubmittedFields(t *testing.T) {
	title := " New title "
	patch, err := (settingsInput{PageTitle: &title}).siteBrandPatch()
	if err != nil {
		t.Fatalf("siteBrandPatch() error = %v", err)
	}
	if patch.LogoURL != nil || patch.TopbarText != nil {
		t.Fatalf("siteBrandPatch() changed unspecified fields: %+v", patch)
	}
	if patch.PageTitle == nil || *patch.PageTitle != "New title" {
		t.Fatalf("siteBrandPatch() page title = %v, want New title", patch.PageTitle)
	}
}

func TestValidLogoURLAcceptsDocumentedDataImages(t *testing.T) {
	mediaTypes := []string{
		"image/svg+xml",
		"image/png",
		"image/jpeg",
		"image/gif",
		"image/webp",
		"image/ico",
		"image/x-icon",
		"image/vnd.microsoft.icon",
	}
	for _, mediaType := range mediaTypes {
		t.Run(mediaType, func(t *testing.T) {
			if value := "data:" + mediaType + ";base64,AQ=="; !validLogoURL(value) {
				t.Fatalf("validLogoURL(%q) = false, want true", value)
			}
		})
	}
}
