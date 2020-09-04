package fsutil

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

var NotADirectory = errors.New("Parent path is not a directory")
var NoPath = errors.New("No path specified")

// EnsureDir ensures that the parent directory of the specified filename
// exists.  If the parent exists and is not a directory, this function
// will return a NotADirectory error

func EnsureDir(filename string) error {
	if filename == "" {
		return NoPath
	}
	dirname := filepath.Dir(filename)
	info, err := os.Stat(dirname)
	if err == nil {
		if info.IsDir() {
			// parent exists and is a directory
			return nil
		}
		// parent exists, but is not a directory
		return errors.Wrapf(NotADirectory, "%s exists and is not a directory", dirname)
	}
	if !os.IsNotExist(err) {
		return errors.Wrapf(err, "Error getting info about %s", dirname)
	}
	// parent does not exist, create it
	err = os.MkdirAll(dirname, 0775)
	if err != nil {
		return errors.Wrapf(err, "Error creating directory %s", dirname)
	}
	// success!
	return nil
}
