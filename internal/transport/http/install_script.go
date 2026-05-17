package transporthttp

import (
	"embed"
	"fmt"
	"path"
	"strings"

	"dash/internal/config"
)

const (
	downloadSchemeToken = "__DOWNLOAD_SCHEME__"
	downloadHostToken   = "__DOWNLOAD_HOST__"
	downloadPathToken   = "__DOWNLOAD_PATH__"
)

//go:embed installscript/linux/install_node.sh installscript/macos/install_node.sh installscript/windows/install_node.ps1
var installScriptTemplates embed.FS

func renderInstallScript(cfg *config.Config, platform string) ([]byte, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	templatePath, err := installScriptTemplatePath(platform)
	if err != nil {
		return nil, err
	}

	templateBytes, err := installScriptTemplates.ReadFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("read install script template %s: %w", templatePath, err)
	}

	rendered := renderTemplate(
		string(templateBytes),
		cfg.App.PublicURLScheme,
		cfg.App.PublicURLHost,
		publicDownloadPath(cfg.App.PublicURLBasePath, platform),
	)
	return []byte(rendered), nil
}

func installScriptTemplatePath(platform string) (string, error) {
	switch platform {
	case "linux":
		return "installscript/linux/install_node.sh", nil
	case "macos":
		return "installscript/macos/install_node.sh", nil
	case "windows":
		return "installscript/windows/install_node.ps1", nil
	default:
		return "", fmt.Errorf("unsupported platform %q", platform)
	}
}

func publicDownloadPath(basePath, platform string) string {
	p := strings.TrimPrefix(strings.TrimSpace(basePath), "/")
	if p == "" {
		return path.Join("/", "deploy", platform)
	}
	return path.Join("/", p, "deploy", platform)
}

func renderTemplate(tpl, scheme, host, dlPath string) string {
	out := tpl
	out = strings.ReplaceAll(out, downloadSchemeToken, scheme)
	out = strings.ReplaceAll(out, downloadHostToken, host)
	out = strings.ReplaceAll(out, downloadPathToken, dlPath)
	return out
}
