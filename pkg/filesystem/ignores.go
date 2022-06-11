package filesystem

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"
)

func GetIgnoreMatcher(path string) (*gitignore.GitIgnore, error) {
	for _, file := range []string{".odoignore", ".gitignore"} {
		contents, err := os.ReadFile(filepath.Join(path, file))
		if err != nil {
			if notExist := os.IsNotExist(err); notExist {
				continue
			}
			return nil, err
		}
		scanner := bufio.NewScanner(bytes.NewReader(contents))
		var rules []string
		for scanner.Scan() {
			line := scanner.Text()
			spaceTrimmedLine := strings.TrimSpace(string(line))
			if len(spaceTrimmedLine) > 0 && !strings.HasPrefix(string(line), "#") && !strings.HasPrefix(string(line), ".git") {
				rules = append(rules, string(line))
			}
		}
		return gitignore.CompileIgnoreLines(rules...), nil
	}
	return gitignore.CompileIgnoreLines(), nil
}
