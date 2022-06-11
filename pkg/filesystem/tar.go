package filesystem

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"k8s.io/klog"
)

var customHomeDir = os.Getenv("CUSTOM_HOMEDIR")

// checkFileExist check if given file exists or not
func checkFileExistWithFS(fileName string) bool {
	_, err := os.Stat(fileName)
	return !os.IsNotExist(err)
}

// MakeTar function is copied from https://github.com/kubernetes/kubernetes/blob/master/pkg/kubectl/cmd/cp.go#L309
// srcPath is ignored if files is set
func MakeTar(srcPath, destPath string, writer io.Writer, files []string, globExps []string) error {
	// TODO: use compression here?
	tarWriter := tar.NewWriter(writer)
	defer tarWriter.Close()
	srcPath = filepath.Clean(srcPath)

	// "ToSlash" is used as all containers within OpenShift are Linux based
	// and thus \opt\app-root\src would be an invalid path. Backward slashes
	// are converted to forward.
	destPath = filepath.ToSlash(filepath.Clean(destPath))
	uniquePaths := make(map[string]bool)
	klog.V(4).Infof("makeTar arguments: srcPath: %s, destPath: %s, files: %+v", srcPath, destPath, files)
	if len(files) == 0 {
		return nil
	}

	//ignoreMatcher := gitignore.CompileIgnoreLines(globExps...)
	for _, fileName := range files {

		if _, ok := uniquePaths[fileName]; ok {
			continue
		} else {
			uniquePaths[fileName] = true
		}

		if !checkFileExistWithFS(fileName) {
			continue
		}

		//				rel, err := filepath.Rel(srcPath, fileName)
		//				if err != nil {
		//					return err
		//				}

		//				matched := ignoreMatcher.MatchesPath(rel)
		//				if matched {
		//					continue
		//				}

		// Fetch path of source file relative to that of source base path so that it can be passed to recursiveTar
		// which uses path relative to base path for taro header to correctly identify file location when untarred

		// now that the file exists, now we need to get the absolute path
		fileAbsolutePath, err := getAbsPath(fileName)
		if err != nil {
			return err
		}
		klog.V(4).Infof("Got abs path: %s", fileAbsolutePath)
		klog.V(4).Infof("Making %s relative to %s", srcPath, fileAbsolutePath)

		// We use "FromSlash" to make this OS-based (Windows uses \, Linux & macOS use /)
		// we get the relative path by joining the two
		destFile, err := filepath.Rel(filepath.FromSlash(srcPath), filepath.FromSlash(fileAbsolutePath))
		if err != nil {
			return err
		}

		// Now we get the source file and join it to the base directory.
		srcFile := filepath.Join(filepath.Base(srcPath), destFile)

		klog.V(4).Infof("makeTar srcFile: %s", srcFile)
		klog.V(4).Infof("makeTar destFile: %s", destFile)

		// The file could be a regular file or even a folder, so use recursiveTar which handles symlinks, regular files and folders
		err = linearTar(filepath.Dir(srcPath), srcFile, filepath.Dir(destPath), destFile, tarWriter)
		if err != nil {
			return err
		}
	}

	return nil
}

// linearTar function is a modified version of https://github.com/kubernetes/kubernetes/blob/master/pkg/kubectl/cmd/cp.go#L319
func linearTar(srcBase, srcFile, destBase, destFile string, tw *tar.Writer) error {
	if destFile == "" {
		return fmt.Errorf("linear Tar error, destFile cannot be empty")
	}

	klog.V(4).Infof("recursiveTar arguments: srcBase: %s, srcFile: %s, destBase: %s, destFile: %s", srcBase, srcFile, destBase, destFile)

	// The destination is a LINUX container and thus we *must* use ToSlash in order
	// to get the copying over done correctly..
	destBase = filepath.ToSlash(destBase)
	destFile = filepath.ToSlash(destFile)
	klog.V(4).Infof("Corrected destinations: base: %s file: %s", destBase, destFile)

	joinedPath := filepath.Join(srcBase, srcFile)

	stat, err := os.Stat(joinedPath)
	if err != nil {
		return err
	}

	if stat.IsDir() {
		files, err := os.ReadDir(joinedPath)
		if err != nil {
			return err
		}
		if len(files) == 0 {
			// case empty directory
			hdr, _ := tar.FileInfoHeader(stat, joinedPath)
			hdr.Name = destFile
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
		}
		return nil
	} else if stat.Mode()&os.ModeSymlink != 0 {
		// case soft link
		hdr, _ := tar.FileInfoHeader(stat, joinedPath)
		target, err := os.Readlink(joinedPath)
		if err != nil {
			return err
		}

		hdr.Linkname = target
		hdr.Name = destFile
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
	} else {
		// case regular file or other file type like pipe
		hdr, err := tar.FileInfoHeader(stat, joinedPath)
		if err != nil {
			return err
		}
		hdr.Name = destFile

		err = tw.WriteHeader(hdr)
		if err != nil {
			return err
		}

		f, err := os.Open(joinedPath)
		if err != nil {
			return err
		}
		defer f.Close() // #nosec G307

		if _, err := io.Copy(tw, f); err != nil {
			return err
		}

		return f.Close()
	}

	return nil
}

// GetAbsPath returns absolute path from passed file path resolving even ~ to user home dir and any other such symbols that are only
// shell expanded can also be handled here
func getAbsPath(path string) (string, error) {
	// Only shell resolves `~` to home so handle it specially
	var dir string
	if strings.HasPrefix(path, "~") {
		if len(customHomeDir) > 0 {
			dir = customHomeDir
		} else {
			usr, err := user.Current()
			if err != nil {
				return path, err
			}
			dir = usr.HomeDir
		}

		if len(path) > 1 {
			path = filepath.Join(dir, path[1:])
		} else {
			path = dir
		}
	}

	path, err := filepath.Abs(path)
	if err != nil {
		return path, err
	}
	return path, nil
}
