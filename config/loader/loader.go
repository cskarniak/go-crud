// config/loader/loader.go
package loader

import (
    "fmt"
    "os"

    "gopkg.in/yaml.v3"
)

// Load lit le fichier YAML à l’emplacement `path` et le décode dans `out`.
func Load(path string, out interface{}) error {
    data, err := os.ReadFile(path)
    if err != nil {
        return fmt.Errorf("impossible de lire %s : %w", path, err)
    }
    if err := yaml.Unmarshal(data, out); err != nil {
        return fmt.Errorf("impossible de parser %s : %w", path, err)
    }
    return nil
}

