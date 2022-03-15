package main

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCopyOperation(t *testing.T) {
	// setup
	testFiles := []string{"ca1.txt", "c2.txt", "ca3.txt"}
	testFilesForRemoval := []string{"ca1-1.txt", "ca2-2.txt", "ca3-3.txt"}
	sourceDir := "source"
	destDir := "dest"
	err := os.Mkdir(sourceDir, 0755)
	if err != nil {
		t.Fail()
	}
	err = createFiles(testFiles, sourceDir)
	if err != nil {
		t.Fail()
	}
	err = os.Mkdir(destDir, 0755)
	if err != nil {
		t.Fail()
	}
	watcher := &AksNodeCAWatcher{
		copyTimestamp: time.Now().Unix(),
		sourceDir:     sourceDir,
		destDir:       destDir,
	}

	t.Run("filesFromSourceShouldAppearInDestAfterCopy", func(t *testing.T) {
		watcher.tagAndCopyFiles()
		if areFilesInDirectory(testFiles, destDir) != true {
			t.Fail()
		}
	})
	t.Run("FilesShouldBeMarkedWithTimestampAfterCopyToDest", func(t *testing.T) {
		watcher.tagAndCopyFiles()
		tag := strconv.FormatInt(watcher.copyTimestamp, 10)
		if areFilesInDirectoryTagged(destDir, tag) != true {
			t.Fail()
		}
	})
	t.Run("FilesWithOlderTimestampShouldBeRemovedInDestAfterCopy", func(t *testing.T) {
		err := createFiles(testFilesForRemoval, destDir)
		if err != nil {
			t.Fail()
		}

		watcher.runIteration()
		files, err := os.ReadDir(watcher.destDir)
		if err != nil {
			t.Fail()
		}
		if !areOlderFilesDeleted(files, watcher.copyTimestamp) {
			t.Fail()
		}
	})

	t.Cleanup(func() {
		err := os.RemoveAll(sourceDir)
		if err != nil {
			t.Fail()
		}
		err = os.RemoveAll(destDir)
		if err != nil {
			t.Fail()
		}
	})
}

func createFiles(fileNames []string, dir string) error {
	for _, fileName := range fileNames {
		_, err := os.Create(dir + "/" + fileName)
		if err != nil {
			return err
		}
	}
	return nil
}

func areFilesInDirectory(fileList []string, targetDir string) bool {
	for _, fileName := range fileList {
		matches, err := filepath.Glob(targetDir + "/" + strings.TrimSuffix(filepath.Base(fileName),
			filepath.Ext(fileName)) + "*")
		if err != nil {
			return false
		}
		return len(matches) > 0
	}
	return false
}

func areFilesInDirectoryTagged(targetDir, tag string) bool {
	files, err := os.ReadDir(targetDir)
	if err != nil {
		log.Fatal(err)
		return false
	}
	for _, file := range files {
		if !strings.Contains(file.Name(), tag) {
			return false
		}
	}
	return true
}

func areOlderFilesDeleted(files []os.DirEntry, timestamp int64) bool {
	for _, file := range files {
		fileTimestampTag, _ := strconv.ParseInt(file.Name()[strings.LastIndex(file.Name(),
			"-")+1:strings.Index(file.Name(), ".")], 10, 64)
		if fileTimestampTag < timestamp {
			return false
		}
	}
	return true
}
