package jscontext

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type pkgJSON struct {
	Main     string            `json:"main"`
	Exports  map[string]string `json:"exports"`
	Deps     map[string]string `json:"dependencies"`
	DevDeps  map[string]string `json:"devDependencies"`
	PeerDeps map[string]string `json:"peerDependencies"`
}

func (p *pkgJSON) hasDep(name string) (string, bool) {
	for _, dependencies := range []map[string]string{p.Deps, p.DevDeps, p.PeerDeps} {
		if version, ok := dependencies[name]; ok {
			return version, true
		}
	}
	return "", false
}

func readPkgJSON(root string) *pkgJSON {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return nil
	}
	var pkg pkgJSON
	if json.Unmarshal(data, &pkg) != nil {
		return nil
	}
	return &pkg
}

var frameworkPriority = []struct{ dep, name string }{
	{"react-native", "react-native"},
	{"next", "nextjs"},
	{"@nestjs/core", "nestjs"},
	{"nuxt", "nuxt"},
	{"@sveltejs/kit", "sveltekit"},
	{"svelte", "svelte"},
	{"fastify", "fastify"},
	{"express", "express"},
	{"vite", "vite"},
	{"react", "react"},
}

func cleanVersion(version string) string {
	return strings.TrimLeft(version, "^~>=<v ")
}

func tsconfigAliases(root string) map[string]bool {
	data, err := os.ReadFile(filepath.Join(root, "tsconfig.json"))
	if err != nil {
		return nil
	}
	var config struct {
		CompilerOptions struct {
			Paths map[string]json.RawMessage `json:"paths"`
		} `json:"compilerOptions"`
	}
	if json.Unmarshal(data, &config) != nil {
		return nil
	}
	aliases := make(map[string]bool, len(config.CompilerOptions.Paths))
	for pattern := range config.CompilerOptions.Paths {
		prefix := strings.TrimSuffix(pattern, "*")
		prefix = strings.TrimSuffix(prefix, "/")
		if prefix != "" {
			aliases[prefix] = true
		}
	}
	return aliases
}

func externalPkgName(specifier string) string {
	specifier = strings.TrimPrefix(specifier, "node:")
	if strings.HasPrefix(specifier, "@") {
		parts := strings.SplitN(specifier, "/", 3)
		if len(parts) >= 2 {
			return parts[0] + "/" + parts[1]
		}
		return specifier
	}
	return strings.SplitN(specifier, "/", 2)[0]
}
