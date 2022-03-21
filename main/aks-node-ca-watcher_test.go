package main

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/spf13/afero"
)

func TestCopyOperation(t *testing.T) {
	testFilesToCreate := []string{"ca1.txt", "c2.txt", "ca3.txt"}
	testFilesForRemoval := []string{"ca1-1.txt", "ca2-2.txt", "ca3-3.txt"}
	sourceDir := "source"
	destDir := "dest"

	t.Run("filesFromSourceShouldAppearInDestAfterCopy", func(t *testing.T) {
		watcher := setUpWatcherForTest(sourceDir, destDir)
		createTestFilesInFs(watcher.podFs, watcher.sourceDir, testFilesToCreate)

		err := watcher.moveFiles()
		if err != nil {
			t.Fatal(err)
		}

		if areFilesInDirectory(watcher.podFs, watcher.destDir, testFilesToCreate) != true {
			t.Fail()
		}
	})
	t.Run("FilesShouldBeMarkedWithTimestampAfterCopyToDest", func(t *testing.T) {
		watcher := setUpWatcherForTest(sourceDir, destDir)
		createTestFilesInFs(watcher.podFs, watcher.sourceDir, testFilesToCreate)

		err := watcher.moveFiles()
		if err != nil {
			t.Fatal(err)
		}

		if areFilesInDirectoryTagged(watcher.podFs, watcher.copyTimestamp, destDir) != true {
			t.Fail()
		}
	})
	t.Run("FilesWithOlderTimestampShouldBeRemovedInDestAfterCopy", func(t *testing.T) {
		watcher := setUpWatcherForTest(sourceDir, destDir)
		createTestFilesInFs(watcher.podFs, watcher.sourceDir, testFilesToCreate)
		createTestFilesInFs(watcher.podFs, watcher.destDir, testFilesForRemoval)

		err := watcher.runIteration()
		if err != nil {
			t.Fatal(err)
		}

		files, err := afero.ReadDir(watcher.podFs, watcher.destDir)
		if err != nil {
			t.Fatal(err)
		}
		if !areOlderFilesDeleted(files, watcher.copyTimestamp) {
			t.Fail()
		}
	})
}

func setUpWatcherForTest(sourceDir string, destDir string) *AksNodeCAWatcher {
	watcher := &AksNodeCAWatcher{
		copyTimestamp: strconv.FormatInt(time.Now().Unix(), 10),
		sourceDir:     sourceDir,
		destDir:       destDir,
		podFs:         createFileSystemForTest(sourceDir, destDir),
	}
	return watcher
}

func createFileSystemForTest(pathsToCreate ...string) afero.Fs {
	testFs := afero.NewMemMapFs()
	for _, path := range pathsToCreate {
		testFs.MkdirAll(path, 0644)
	}
	return testFs
}

func createTestFilesInFs(fs afero.Fs, path string, filesToCreate []string) {
	for _, fileName := range filesToCreate {
		afero.WriteFile(fs, path+"/"+fileName, []byte(fileName), 0644)
	}
}

func areFilesInDirectory(targetFs afero.Fs, targetDir string, fileList []string) bool {
	for _, fileName := range fileList {
		matches, err := afero.Glob(targetFs, targetDir+"/"+strings.TrimSuffix(filepath.Base(fileName),
			filepath.Ext(fileName))+"*")
		if err != nil {
			return false
		}
		return len(matches) > 0
	}
	return false
}

func areFilesInDirectoryTagged(targetFs afero.Fs, tag, targetDir string) bool {
	files, err := afero.ReadDir(targetFs, targetDir)
	if err != nil {
		log.Fatal(err)
	}
	for _, file := range files {
		if !strings.Contains(file.Name(), tag) {
			return false
		}
	}
	return true
}

func areOlderFilesDeleted(files []os.FileInfo, timestamp string) bool {
	for _, file := range files {
		fileTimestampTag := file.Name()[strings.LastIndex(file.Name(), "-")+1 : strings.Index(file.Name(), ".")]
		if fileTimestampTag < timestamp {
			return false
		}
	}
	return true
}
