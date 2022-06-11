package filesystem

import (
	"os"
	"path/filepath"

	gitignore "github.com/sabhiram/go-gitignore"
)

func Archive(path string, tarFile string, ignoreMatcher *gitignore.GitIgnore) (int64, error) {
	err := os.Remove(tarFile)
	if err != nil {
		if notExist := os.IsNotExist(err); !notExist {
			return 0, err
		}
	}
	allFiles, err := getAllFiles(path, ignoreMatcher)
	if err != nil {
		return 0, err
	}
	tar, err := os.Create(tarFile)
	if err != nil {
		return 0, err
	}
	err = MakeTar(path, path, tar, allFiles, nil)
	if err != nil {
		return 0, err
	}
	info, err := os.Stat(tarFile)
	if err != nil {
		return 0, err
	}
	return info.ModTime().UnixNano(), nil

}

func getAllFiles(rootPath string, ignoreMatcher *gitignore.GitIgnore) ([]string, error) {
	var result []string
	err := filepath.Walk(rootPath,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			rel, err := filepath.Rel(rootPath, path)
			if err != nil {
				return err
			}

			if matched := ignoreMatcher.MatchesPath(rel); matched {
				if info.IsDir() {
					return filepath.SkipDir
				} else {
					return nil
				}
			}

			if info.IsDir() {
				if rel == ".git" || rel == ".odo" {
					return filepath.SkipDir
				}
				return nil
			}

			result = append(result, path)
			return nil
		})
	if err != nil {
		return nil, err
	}
	return result, nil
}
