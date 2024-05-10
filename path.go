package main

import (
	"os"
	"path/filepath"
	"strings"
)

// Path represents a wrapper around a string to provide path manipulation methods.
type Path string

// Append adds a component to the path.
func (p Path) appendingPathComponent(component string) Path {
	return Path(filepath.Join(string(p), component))
}

// LastComponent returns the last component of the path.
func (p Path) lastPathComponent() string {
	return filepath.Base(string(p))
}

// RemovingLastPathComponent removes the last component from the path.
func (p Path) removingLastPathComponent() Path {
	return Path(filepath.Dir(string(p)))
}

// IsDirectory checks if the path represents a directory.
func (p Path) isDirectory() bool {
	info, err := os.Stat(string(p))
	if err != nil {
		return false
	}
	return info.IsDir()
}

func (p Path) getDirectoryContents() ([]Path, error) {
	files, err := os.ReadDir(string(p))
	if err != nil {
		return nil, err
	}

	var fileNames []Path
	for _, file := range files {
		if strings.HasPrefix(file.Name(), ".") {
			continue
		}
		fileNames = append(fileNames, p.appendingPathComponent(file.Name()))
	}

	return fileNames, nil
}

// Function to find all video files in a directory recursively
func (p Path) getDirectoryContentsRecursively() ([]Path, error) {
	var videoFiles []Path
	err := filepath.Walk(string(p), func(s string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		path := Path(s)
		if !info.IsDir() && !strings.HasPrefix(s, ".") {
			videoFiles = append(videoFiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return videoFiles, nil
}

func (p Path) extension() string {
	return strings.TrimPrefix(filepath.Ext(string(p)), ".")
}

func (p Path) removingPathExtension() Path {
	return Path(strings.TrimSuffix(string(p), "."+p.extension()))
}

func (p Path) appendingPathExtension(ext string) Path {
	return Path(string(p) + "." + ext)
}

var videoExtensions []string = []string{"mov", "m4v", "mkv", "avi", "mp4", "mpg", "wmv", "flv", "webm", "ts", "m2ts", "mxf", "ogv", "3gp", "3g2"}

func (p Path) isVideoFile() bool {
	if strings.HasPrefix(p.lastPathComponent(), ".") {
		return false
	}

	if p.isDirectory() {
		return p.appendingPathComponent("VIDEO_TS").isDirectory()
	}
	ext := p.extension()
	for _, videoExt := range videoExtensions {
		if strings.EqualFold(ext, videoExt) {
			return true
		}
	}

	return false
}

func (p Path) findRelatedVideoSymlink() Path {
	if p.isSymlink() {
		return p
	} else if p.isDirectory() {
		videoFiles := getVideoFiles(p)
		if len(videoFiles) > 0 {
			return videoFiles[0]
		}
		return ""
	}
	fileName := p.lastPathComponent()

	fileName = strings.TrimSuffix(fileName, "-poster.jpg")
	fileName = strings.TrimSuffix(fileName, "-fanart.jpg")
	fileName = strings.TrimSuffix(fileName, ".nfo")

	base := p.removingLastPathComponent()
	for _, ext := range videoExtensions {
		path := base.appendingPathComponent(fileName + "." + ext)
		if path.isSymlink() {
			return path
		}
	}
	return ""
}

func (p Path) isSymlink() bool {
	if info, err := os.Lstat(string(p)); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return true
	}
	return false
}

// Function to find all video files in a directory recursively
func (p Path) findVideoFilesInFolder() []Path {
	paths, err := p.getDirectoryContentsRecursively()
	if err != nil {
		return nil
	}
	var filteredPaths []Path
	for _, path := range paths {
		if path.isVideoFile() {
			filteredPaths = append(filteredPaths, path)
		}
	}
	return filteredPaths
}

func (p Path) exists() bool {
	if _, err := os.Lstat(string(p)); err == nil {
		return true
	} else {
		return !os.IsNotExist(err)
	}
}

func (p Path) removeItem() error {
	if p.isDirectory() {
		return os.RemoveAll(string(p))
	} else {
		return os.Remove(string(p))
	}
}
